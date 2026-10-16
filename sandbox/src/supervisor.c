// SPDX-License-Identifier: MIT
/*
 * The supervisor's watchdog loop.
 *
 * One poll() loop on one timer enforces the wall-clock timeout, the CPU-time
 * budget (cpu.stat), memory sampling and the combined output cap, serves the
 * seccomp notifications and feeds stdin. The exit status, signal and rusage
 * come from wait4(2); nothing the program prints can change them.
 */
#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <seccomp.h>
#include <signal.h>
#include <stdlib.h>
#include <string.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/wait.h>
#include <unistd.h>

#include "sbx.h"

#ifndef SYS_pidfd_send_signal
#define SYS_pidfd_send_signal 424
#endif
#ifndef SYS_pidfd_getfd
#define SYS_pidfd_getfd 438
#endif

#define TICK_MS		5
#define KILL_GRACE_NS	(5ULL * 1000000000ULL)

static volatile sig_atomic_t stop_signal;

static void on_stop(int sig)
{
	stop_signal = sig;
}

void supervisor_signals(void)
{
	/* No SA_RESTART: poll() has to wake up. */
	struct sigaction sa = { .sa_handler = on_stop };

	sigemptyset(&sa.sa_mask);
	sigaction(SIGTERM, &sa, NULL);
	sigaction(SIGINT, &sa, NULL);
	sigaction(SIGHUP, &sa, NULL);
}

struct watch {
	struct run *r;
	struct result *res;
	bool killed;
	uint64_t killed_at;
	int64_t peak_sampled;	/* bytes; only used when memory.peak is unavailable */
	size_t in_off;
};

/* The first cause wins: later detections of the same death do not rewrite it. */
static void kill_sandbox(struct watch *w, enum term_reason why)
{
	if (w->res->reason == TR_NONE)
		w->res->reason = why;
	if (w->killed)
		return;
	w->killed = true;
	w->killed_at = now_ns();
	/* SIGKILL to PID 1 of the PID namespace takes every descendant with it. */
	syscall(SYS_pidfd_send_signal, w->r->pidfd, SIGKILL, NULL, 0);
	cgroup_kill(&w->r->cg);
}

static void on_setup_msg(struct watch *w)
{
	struct run *r = w->r;
	struct msg m;
	ssize_t n = recv(r->sock[0], &m, sizeof(m), MSG_DONTWAIT);
	int fd;

	if (!n) {	/* EOF: the child closed its end, by execve() or by dying */
		close_fd(&r->sock[0]);
		return;
	}
	if (n != (ssize_t)sizeof(m))
		return;
	if (m.type == MSG_ERR) {
		m.text[sizeof(m.text) - 1] = '\0';
		sbx_fail("%s", m.text);
		kill_sandbox(w, TR_INTERNAL_ERROR);
	} else if (m.type == MSG_NOTIFY_FD) {
		/*
		 * pidfd_getfd(2) needs ptrace-attach rights over the child: the
		 * supervisor owns the child's user namespace and, for Yama
		 * ptrace_scope=1, is its ancestor.
		 */
		fd = (int)syscall(SYS_pidfd_getfd, r->pidfd, m.value, 0);
		if (fd < 0) {
			sbx_fail("pidfd_getfd: %m");
			kill_sandbox(w, TR_INTERNAL_ERROR);
			return;
		}
		fcntl(fd, F_SETFL, fcntl(fd, F_GETFL) | O_NONBLOCK);
		r->notify_fd = fd;
		m = (struct msg){ .type = MSG_ACK };
		if (send(r->sock[0], &m, sizeof(m), MSG_NOSIGNAL) != (ssize_t)sizeof(m)) {
			sbx_fail("ack to child: %m");
			kill_sandbox(w, TR_INTERNAL_ERROR);
		}
	}
}

/* A syscall outside the allowlist: record its number, refuse it, end the run. */
static void on_notify(struct watch *w)
{
	struct seccomp_notif *req;
	struct seccomp_notif_resp *resp;

	for (;;) {
		if (seccomp_notify_alloc(&req, &resp))
			return;
		/* -EAGAIN: drained; -ENOENT: the caller died meanwhile. */
		if (seccomp_notify_receive(w->r->notify_fd, req)) {
			seccomp_notify_free(req, resp);
			return;
		}
		if (!w->res->has_violation) {
			w->res->has_violation = true;
			w->res->syscall_nr = req->data.nr;
		}
		/* Never FLAG_CONTINUE: the arguments could change after the check. */
		resp->id = req->id;
		resp->error = -EPERM;
		seccomp_notify_respond(w->r->notify_fd, resp);
		seccomp_notify_free(req, resp);
		kill_sandbox(w, TR_SECURITY_VIOLATION);
	}
}

static void pump_stdin(struct watch *w)
{
	struct run *r = w->r;

	while (r->in_w >= 0 && w->in_off < r->input_len) {
		ssize_t n = write(r->in_w, r->input + w->in_off, r->input_len - w->in_off);

		if (n > 0)
			w->in_off += (size_t)n;
		else if (n < 0 && errno == EINTR)
			continue;
		else if (n < 0 && errno == EAGAIN)
			return;
		else	/* EPIPE: the program closed stdin, nothing left to deliver */
			w->in_off = r->input_len;
	}
	if (r->in_w >= 0)
		close_fd(&r->in_w);	/* EOF for the program */
}

static int64_t output_size(const struct run *r)
{
	struct stat so, se;

	if (fstat(r->out_fd, &so) || fstat(r->err_fd, &se))
		return 0;
	return (int64_t)so.st_size + (int64_t)se.st_size;
}

static void check_limits(struct watch *w, uint64_t now, uint64_t deadline)
{
	const struct limits *l = &w->r->opts->limits;
	int64_t mem;

	if (stop_signal) {
		sbx_fail("terminated by signal %d", (int)stop_signal);
		kill_sandbox(w, TR_INTERNAL_ERROR);
	} else if (now >= deadline) {
		kill_sandbox(w, TR_TIMEOUT_WALL);
	} else if (cgroup_cpu_usec(&w->r->cg) >= (int64_t)(l->cpu_time_ms * 1000)) {
		kill_sandbox(w, TR_TIMEOUT_CPU);
	} else if (output_size(w->r) > (int64_t)l->max_output_bytes) {
		kill_sandbox(w, TR_OUTPUT_LIMIT);
	} else if (w->r->cg.fd[CG_MEM_PEAK] < 0) {
		mem = cgroup_mem_current(&w->r->cg);
		if (mem > w->peak_sampled)
			w->peak_sampled = mem;
	}
}

static char *read_capture(int fd, int64_t size, int64_t budget, size_t *len)
{
	int64_t want = size < budget ? size : budget, got = 0;
	char *buf;

	*len = 0;
	if (want <= 0)
		return NULL;
	buf = malloc((size_t)want + 1);
	if (!buf)
		return NULL;
	while (got < want) {
		ssize_t n = pread(fd, buf + got, (size_t)(want - got), got);

		if (n <= 0)
			break;
		got += n;
	}
	buf[got] = '\0';
	*len = (size_t)got;
	return buf;
}

static enum term_reason classify(struct watch *w, int status, int64_t cpu_us)
{
	const struct cgroup *cg = &w->r->cg;
	int sig;

	if (cgroup_counter(cg, CG_MEM_EVENTS, "oom_kill") ||
	    cgroup_counter(cg, CG_MEM_EVENTS, "oom_group_kill"))
		return TR_MEMORY_LIMIT;
	if (w->res->output_truncated)
		return TR_OUTPUT_LIMIT;
	if (WIFEXITED(status))
		return TR_EXITED;
	sig = WTERMSIG(status);
	if (sig == SIGSYS) {	/* foreign-ABI syscall: the filter's default kills the thread */
		w->res->has_violation = true;
		w->res->syscall_nr = -1;
		return TR_SECURITY_VIOLATION;
	}
	if ((sig == SIGKILL || sig == SIGXCPU) &&
	    cpu_us >= (int64_t)(w->r->opts->limits.cpu_time_ms * 1000))
		return TR_TIMEOUT_CPU;	/* the RLIMIT_CPU backstop */
	return TR_SIGNALED;
}

static void collect(struct watch *w, int status, const struct rusage *ru, uint64_t start)
{
	const struct limits *l = &w->r->opts->limits;
	struct result *res = w->res;
	struct run *r = w->r;
	int64_t cpu_us = cgroup_cpu_usec(&r->cg);
	int64_t peak = cgroup_mem_peak(&r->cg);
	int64_t cap = (int64_t)l->max_output_bytes;
	struct stat st;
	int64_t out_size = 0, err_size = 0;

	res->wall_ms = (now_ns() - start) / 1000000;
	res->cpu_ms = cpu_us > 0 ? (uint64_t)cpu_us / 1000 : 0;
	if (WIFEXITED(status))
		res->exit_code = WEXITSTATUS(status);
	if (WIFSIGNALED(status))
		res->term_signal = WTERMSIG(status);

	/* Peak memory: memory.peak (5.19+), else sampled memory.current, else ru_maxrss. */
	if (peak >= 0) {
		res->memory_kb = (uint64_t)peak / 1024;
		res->mem_source = MEMSRC_CGROUP_PEAK;
	} else if (w->peak_sampled > 0) {
		res->memory_kb = (uint64_t)w->peak_sampled / 1024;
		res->mem_source = MEMSRC_CGROUP_SAMPLED;
	} else {
		res->memory_kb = (uint64_t)ru->ru_maxrss;
		res->mem_source = MEMSRC_RUSAGE;
	}

	if (!fstat(r->out_fd, &st))
		out_size = st.st_size;
	if (!fstat(r->err_fd, &st))
		err_size = st.st_size;
	res->output_truncated = out_size + err_size > cap;
	res->out = read_capture(r->out_fd, out_size, cap, &res->out_len);
	res->err = read_capture(r->err_fd, err_size, cap - (int64_t)res->out_len, &res->err_len);

	res->pids_limit_hit = cgroup_counter(&r->cg, CG_PIDS_EVENTS, "max") > 0;
	if (res->reason == TR_NONE)
		res->reason = classify(w, status, cpu_us);
	if (res->reason != TR_SECURITY_VIOLATION)
		res->has_violation = false;
}

int supervise(struct run *r, struct result *res)
{
	const struct limits *l = &r->opts->limits;
	struct watch w = { .r = r, .res = res };
	const uint64_t start = now_ns();
	const uint64_t deadline = start + l->timeout_s * 1000000000ULL;
	struct rusage ru;
	int status;

	res->exit_code = -1;
	if (fcntl(r->in_w, F_SETFL, fcntl(r->in_w, F_GETFL) | O_NONBLOCK))
		return sbx_fail("fcntl stdin pipe: %m");
	if (!r->input_len)
		close_fd(&r->in_w);

	for (;;) {
		/* poll() ignores negative descriptors, so absent ones need no special case. */
		struct pollfd pfd[] = {
			{ .fd = r->pidfd, .events = POLLIN },
			{ .fd = r->notify_fd, .events = POLLIN },
			{ .fd = r->in_w, .events = POLLOUT },
			{ .fd = r->sock[0], .events = POLLIN },
		};
		uint64_t now;

		if (poll(pfd, ARRAY_SIZE(pfd), TICK_MS) < 0 && errno != EINTR) {
			sbx_fail("poll: %m");
			kill_sandbox(&w, TR_INTERNAL_ERROR);
			break;
		}
		if (pfd[3].revents & (POLLIN | POLLHUP))
			on_setup_msg(&w);
		if (pfd[1].revents & POLLIN)
			on_notify(&w);
		if (pfd[2].revents & (POLLOUT | POLLERR | POLLHUP))
			pump_stdin(&w);
		if (pfd[0].revents & POLLIN)
			break;

		now = now_ns();
		if (!w.killed)
			check_limits(&w, now, deadline);
		else if (now - w.killed_at > KILL_GRACE_NS)
			return sbx_fail("sandbox survived SIGKILL");
	}

	if (wait4(r->pid, &status, 0, &ru) < 0)
		return sbx_fail("wait4: %m");
	r->pid = 0;
	collect(&w, status, &ru, start);
	return res->reason == TR_INTERNAL_ERROR ? -1 : 0;
}
