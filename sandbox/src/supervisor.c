// SPDX-License-Identifier: MIT
/*
 * The supervisor's watchdog loop.
 *
 * One poll() loop on one timer enforces the wall-clock timeout, the CPU-time
 * budget (cpu.stat), memory sampling and the combined output cap, serves the
 * seccomp notifications (or, in the ptrace fallback, the stops of the traced
 * tree) and feeds stdin. The exit status, signal and rusage come from
 * wait4(2); nothing the program prints can change them.
 */
#include <errno.h>
#include <fcntl.h>
#include <linux/ptrace.h>
#include <poll.h>
#include <seccomp.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/resource.h>
#include <sys/signalfd.h>
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

/*
 * What the tracer asks for in trace mode: stops at denied syscalls, and
 * automatic attachment of every thread and process the program creates (they
 * would otherwise run unobserved). EXITKILL makes the kernel kill the whole
 * tree if this process dies.
 */
#define TRACE_OPTIONS	(PTRACE_O_TRACESECCOMP | PTRACE_O_TRACECLONE | PTRACE_O_TRACEFORK | \
			 PTRACE_O_TRACEVFORK | PTRACE_O_EXITKILL)

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
	int sfd;		/* signalfd for SIGCHLD: wakes the loop on a ptrace stop */
	int fault_signal;	/* trace mode: the fault that ended PID 1 (see in_init_group) */
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

/* ptrace(2) through syscall(2): glibc's <sys/ptrace.h> clashes with <linux/ptrace.h>. */
static long sbx_ptrace(long req, pid_t pid, long addr, long data)
{
	return syscall(SYS_ptrace, req, (long)pid, addr, data);
}

/* Lets the child proceed to execve(). */
static void ack_child(struct watch *w)
{
	struct msg m = { .type = MSG_ACK };

	if (send(w->r->sock[0], &m, sizeof(m), MSG_NOSIGNAL) != (ssize_t)sizeof(m)) {
		sbx_fail("ack to child: %m");
		kill_sandbox(w, TR_INTERNAL_ERROR);
	}
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
		r->notify_fd = fd;
		ack_child(w);
	} else if (m.type == MSG_TRACE_READY) {
		/*
		 * Fallback for kernels without seccomp user notification. SEIZE
		 * does not stop the child, and it has to be in place before
		 * execve(): a traced syscall without a tracer fails with ENOSYS.
		 * Needs the same ptrace rights over the child as pidfd_getfd.
		 */
		r->traced = true;
		if (sbx_ptrace(PTRACE_SEIZE, r->pid, 0, TRACE_OPTIONS)) {
			sbx_fail("PTRACE_SEIZE: %m");
			kill_sandbox(w, TR_INTERNAL_ERROR);
			return;
		}
		ack_child(w);
	}
}

/* The syscall number of a PTRACE_EVENT_SECCOMP stop, or -1. */
static long traced_syscall_nr(pid_t pid)
{
	struct ptrace_syscall_info info;

	if (sbx_ptrace(PTRACE_GET_SYSCALL_INFO, pid, sizeof(info), (long)&info) <= 0 ||
	    info.op != PTRACE_SYSCALL_INFO_SECCOMP)
		return -1;
	return (long)info.seccomp.nr;
}

static bool stop_signal_is_group_stop(int sig)
{
	return sig == SIGSTOP || sig == SIGTSTP || sig == SIGTTIN || sig == SIGTTOU;
}

/*
 * The program is PID 1 of its namespace, and the kernel treats PID 1
 * specially: it ignores a signal for which it has no handler, unless the CPU
 * raised it (a fault), in which case it dies of it. Both rules lapse while a
 * tracer is attached: a signal the tracer passes on is dropped even if it is
 * a fault, and a stop signal does stop PID 1. The helpers below put the
 * kernel's rules back, so that tracing changes nothing the program can see.
 */

/*
 * True if pid is PID 1 of the namespace or one of its threads:
 * /proc/<init>/task/<tid> exists only then.
 */
static bool in_init_group(const struct watch *w, pid_t pid)
{
	char path[64];
	struct stat st;

	snprintf(path, sizeof(path), "/proc/%d/task/%d", (int)w->r->pid, (int)pid);
	return !stat(path, &st);
}

/* True if the thread has a handler for sig (SigCgt in its /proc status); unknown counts as yes. */
static bool has_handler(const struct watch *w, pid_t pid, int sig)
{
	char path[64], line[128];
	bool caught = true;
	FILE *f;

	snprintf(path, sizeof(path), "/proc/%d/task/%d/status", (int)w->r->pid, (int)pid);
	f = fopen(path, "re");
	if (!f)
		return true;
	while (fgets(line, sizeof(line), f)) {
		if (!strncmp(line, "SigCgt:", 7)) {
			caught = (strtoull(line + 7, NULL, 16) >> (sig - 1)) & 1;
			break;
		}
	}
	fclose(f);
	return caught;
}

/* True for a signal the CPU raised in the program, not one sent with kill(). */
static bool is_fault(pid_t pid, int sig)
{
	siginfo_t si;

	if (sig != SIGSEGV && sig != SIGBUS && sig != SIGILL && sig != SIGFPE)
		return false;
	return sbx_ptrace(PTRACE_GETSIGINFO, pid, 0, (long)&si) == 0 && si.si_code > 0;
}

/*
 * One ptrace stop of the program or of a thread or process it created. A
 * denied syscall is recorded and ends the run: SIGKILL reaches a task that is
 * stopped, so the syscall never executes. Every other stop is resumed,
 * passing on the signal that caused it, so the program behaves as untraced.
 */
static void on_trace_stop(struct watch *w, pid_t pid, int status)
{
	int sig = WSTOPSIG(status), event = status >> 16;

	if (event == PTRACE_EVENT_SECCOMP) {
		if (!w->res->has_violation) {
			w->res->has_violation = true;
			w->res->syscall_nr = traced_syscall_nr(pid);
		}
		kill_sandbox(w, TR_SECURITY_VIOLATION);
	} else if (event == PTRACE_EVENT_STOP) {
		/* A group-stop stays stopped (LISTEN); a new tracee's first stop goes on. */
		sbx_ptrace(stop_signal_is_group_stop(sig) ? PTRACE_LISTEN : PTRACE_CONT, pid, 0, 0);
	} else if (event) {
		sbx_ptrace(PTRACE_CONT, pid, 0, 0);	/* clone, fork, vfork */
	} else {
		/* Signal-delivery-stop: pass the signal on as the kernel would. */
		if (in_init_group(w, pid) && !has_handler(w, pid, sig)) {
			if (is_fault(pid, sig)) {
				w->fault_signal = sig;
				kill_sandbox(w, TR_SIGNALED);
				return;
			}
			sig = 0;	/* PID 1 ignores it */
		}
		sbx_ptrace(PTRACE_CONT, pid, 0, sig);
	}
}

/*
 * Serves every pending wait event of the traced tree. The exit of the program itself is only peeked
 * at (WNOWAIT) and left for supervise() to collect with its rusage; the exits of other tasks, which
 * are our ptrace children too, are reaped here.
 */
static void trace_events(struct watch *w)
{
	for (;;) {
		const int peek = WEXITED | WSTOPPED | WNOHANG | WNOWAIT | __WALL;
		siginfo_t si = { 0 };
		int status;

		if (waitid(P_ALL, 0, &si, peek) < 0 || !si.si_pid)
			return;
		if (si.si_code == CLD_EXITED || si.si_code == CLD_KILLED ||
		    si.si_code == CLD_DUMPED) {
			if (si.si_pid == w->r->pid)
				return;
			if (waitpid(si.si_pid, &status, WNOHANG | __WALL) != si.si_pid)
				return;	/* not consumed: try again on the next tick */
			continue;
		}
		if (waitpid(si.si_pid, &status, WNOHANG | __WALL) != si.si_pid)
			return;
		if (WIFSTOPPED(status))
			on_trace_stop(w, si.si_pid, status);
	}
}

static void drain_sigchld(int sfd)
{
	struct signalfd_siginfo si;

	while (read(sfd, &si, sizeof(si)) == (ssize_t)sizeof(si))
		;
}

/*
 * A syscall outside the allowlist: record its number, refuse it, end the run.
 *
 * Called when poll() reports a pending notification, and receives exactly one.
 * SECCOMP_IOCTL_NOTIF_RECV sleeps in the kernel when nothing is pending, even
 * on an O_NONBLOCK descriptor, so a second call "to drain the queue" blocks
 * until the filter is released, which only happens once this process has
 * reaped the sandbox. Poll is level triggered: anything still pending is
 * served on the next iteration.
 */
static void on_notify(struct watch *w)
{
	struct seccomp_notif *req;
	struct seccomp_notif_resp *resp;

	if (seccomp_notify_alloc(&req, &resp))
		return;
	/* -ENOENT: the caller was killed between poll() and here. */
	if (seccomp_notify_receive(w->r->notify_fd, req) == 0) {
		if (!w->res->has_violation) {
			w->res->has_violation = true;
			w->res->syscall_nr = req->data.nr;
		}
		/* Never FLAG_CONTINUE: the arguments could change after the check. */
		resp->id = req->id;
		resp->error = -EPERM;
		seccomp_notify_respond(w->r->notify_fd, resp);
		kill_sandbox(w, TR_SECURITY_VIOLATION);
	}
	seccomp_notify_free(req, resp);
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
	if (w->fault_signal) {	/* the supervisor sent SIGKILL, but the program died of its fault */
		res->exit_code = -1;
		res->term_signal = w->fault_signal;
	}

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
	struct watch w = { .r = r, .res = res, .sfd = -1 };
	const uint64_t start = now_ns();
	const uint64_t deadline = start + l->timeout_s * 1000000000ULL;
	struct rusage ru;
	sigset_t chld;
	int status, ret = 0;

	res->exit_code = -1;
	if (fcntl(r->in_w, F_SETFL, fcntl(r->in_w, F_GETFL) | O_NONBLOCK))
		return sbx_fail("fcntl stdin pipe: %m");
	if (!r->input_len)
		close_fd(&r->in_w);

	/*
	 * In trace mode every stop of the program is a wait event, and pidfd does not report those.
	 * SIGCHLD through a signalfd wakes the loop at once; the tick below is only the safety net.
	 */
	sigemptyset(&chld);
	sigaddset(&chld, SIGCHLD);
	sigprocmask(SIG_BLOCK, &chld, NULL);
	w.sfd = signalfd(-1, &chld, SFD_NONBLOCK | SFD_CLOEXEC);
	if (w.sfd < 0)
		return sbx_fail("signalfd: %m");

	for (;;) {
		/* poll() ignores negative descriptors, so absent ones need no special case. */
		struct pollfd pfd[] = {
			{ .fd = r->pidfd, .events = POLLIN },
			{ .fd = r->notify_fd, .events = POLLIN },
			{ .fd = r->in_w, .events = POLLOUT },
			{ .fd = r->sock[0], .events = POLLIN },
			{ .fd = w.sfd, .events = POLLIN },
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
		if (pfd[4].revents & POLLIN)
			drain_sigchld(w.sfd);
		if (r->traced)
			trace_events(&w);
		if (pfd[2].revents & (POLLOUT | POLLERR | POLLHUP))
			pump_stdin(&w);
		if (pfd[0].revents & POLLIN)
			break;

		now = now_ns();
		if (!w.killed) {
			check_limits(&w, now, deadline);
		} else if (now - w.killed_at > KILL_GRACE_NS) {
			ret = sbx_fail("sandbox survived SIGKILL");
			goto out;
		}
	}

	if (wait4(r->pid, &status, 0, &ru) < 0) {
		ret = sbx_fail("wait4: %m");
		goto out;
	}
	r->pid = 0;
	collect(&w, status, &ru, start);
	ret = res->reason == TR_INTERNAL_ERROR ? -1 : 0;
out:
	close_fd(&w.sfd);
	return ret;
}
