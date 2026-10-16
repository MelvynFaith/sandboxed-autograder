// SPDX-License-Identifier: MIT
/*
 * Life cycle of one execution, supervisor side.
 *
 * mount(2) needs CAP_SYS_ADMIN over the user namespace that owns the mount
 * namespace, which the unprivileged service user lacks in the host
 * namespace. The supervisor therefore enters a helper user + mount namespace
 * of its own and creates the work tmpfs there; the sandbox child is cloned as
 * a namespace nested below it. As a side effect the tmpfs cannot outlive the
 * supervisor, however it dies.
 */
#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <sched.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mount.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/wait.h>
#include <unistd.h>

#include "sbx.h"

#ifndef SYS_clone3
#define SYS_clone3 435
#endif
#ifndef SYS_pidfd_send_signal
#define SYS_pidfd_send_signal 424
#endif

/* struct clone_args up to CLONE_ARGS_SIZE_VER0, to stay independent of <linux/sched.h>. */
struct clone_args {
	uint64_t flags;
	uint64_t pidfd;
	uint64_t child_tid;
	uint64_t parent_tid;
	uint64_t exit_signal;
	uint64_t stack;
	uint64_t stack_size;
	uint64_t tls;
};

void result_free(struct result *res)
{
	free(res->out);
	free(res->err);
	res->out = res->err = NULL;
}

static void run_init(struct run *r, const struct opts *o, const unsigned char *input, size_t len)
{
	memset(r, 0, sizeof(*r));
	r->opts = o;
	r->input = input;
	r->input_len = len;
	r->out_fd = r->err_fd = r->in_r = r->in_w = -1;
	r->sock[0] = r->sock[1] = -1;
	r->pidfd = r->notify_fd = -1;
	cgroup_init(&r->cg);
}

static int enter_helper_namespace(void)
{
	char map[32];
	uid_t uid = geteuid();
	gid_t gid = getegid();

	/* CLONE_NEWUSER needs a single-threaded caller. */
	if (unshare(CLONE_NEWUSER | CLONE_NEWNS))
		return sbx_fail("unshare: %m (unprivileged user namespaces disabled?)");
	/* gid_map may only be written by an unprivileged process after setgroups=deny. */
	if (write_file("/proc/self/setgroups", "deny"))
		return sbx_fail("write /proc/self/setgroups: %m");
	snprintf(map, sizeof(map), "%u %u 1\n", uid, uid);
	if (write_file("/proc/self/uid_map", map))
		return sbx_fail("write /proc/self/uid_map: %m");
	snprintf(map, sizeof(map), "%u %u 1\n", gid, gid);
	if (write_file("/proc/self/gid_map", map))
		return sbx_fail("write /proc/self/gid_map: %m");
	if (mount(NULL, "/", NULL, MS_REC | MS_PRIVATE, NULL))
		return sbx_fail("make mounts private: %m");
	return 0;
}

static int mkdir_parents(const char *base, const char *rel)
{
	char path[PATH_MAX];
	char *p;

	if (pathf(path, sizeof(path), "%s/%s", base, rel))
		return -1;
	for (p = path + strlen(base) + 1; *p; p++) {
		if (*p != '/')
			continue;
		*p = '\0';
		if (mkdir(path, 0755) && errno != EEXIST)
			return sbx_fail("mkdir %s: %m", path);
		*p = '/';
	}
	return 0;
}

static int copy_into_work(const struct run *r, const struct copy_spec *c)
{
	char dst[PATH_MAX], buf[16384];
	int in, out, ret = 0;
	ssize_t n;

	if (mkdir_parents(r->workdir, c->dest) ||
	    pathf(dst, sizeof(dst), "%s/%s", r->workdir, c->dest))
		return -1;
	in = open(c->src, O_RDONLY | O_CLOEXEC);
	if (in < 0)
		return sbx_fail("open %s: %m", c->src);
	out = open(dst, O_WRONLY | O_CREAT | O_EXCL | O_CLOEXEC, 0644);
	if (out < 0) {
		sbx_fail("create %s: %m", c->dest);
		close(in);
		return -1;
	}
	while ((n = read(in, buf, sizeof(buf))) > 0) {
		if (write_all(out, buf, (size_t)n)) {
			ret = sbx_fail("write %s: %m", c->dest);
			break;
		}
	}
	if (n < 0)
		ret = sbx_fail("read %s: %m", c->src);
	close(in);
	close(out);
	return ret;
}

static int prepare_dirs(struct run *r)
{
	const struct opts *o = r->opts;
	char base[PATH_MAX], tmpl[PATH_MAX + 32], mopts[96];
	size_t i;

	if (!realpath(o->run_dir, base))
		return sbx_fail("run-dir %s: %m", o->run_dir);
	/* The pid in the name lets --sweep tell a live run from a dead one. */
	if (pathf(tmpl, sizeof(tmpl), "%s/sbx-%d-XXXXXX", base, (int)getpid()))
		return -1;
	if (!mkdtemp(tmpl))
		return sbx_fail("mkdtemp %s: %m", tmpl);
	if (pathf(r->rundir, sizeof(r->rundir), "%s", tmpl) ||
	    pathf(r->rootdir, sizeof(r->rootdir), "%s/root", tmpl) ||
	    pathf(r->workdir, sizeof(r->workdir), "%s/work", tmpl))
		return -1;

	/* The child mounts its own tmpfs on rootdir: pivot_root(2) rejects a new
	 * root that was inherited from a more privileged namespace. */
	if (mkdir(r->rootdir, 0755) || mkdir(r->workdir, 0755))
		return sbx_fail("mkdir below %s: %m", r->rundir);

	snprintf(mopts, sizeof(mopts), "size=%llum,nr_inodes=32768,mode=0755",
		 (unsigned long long)o->limits.tmpfs_size_mb);
	if (mount("tmpfs", r->workdir, "tmpfs", MS_NOSUID | MS_NODEV | MS_NOEXEC, mopts))
		return sbx_fail("mount work tmpfs: %m%s", errno == EPERM ?
				" (kernel.apparmor_restrict_unprivileged_userns=1 strips the "
				"capabilities of new user namespaces)" : "");
	r->work_mounted = true;

	for (i = 0; i < o->ncopies; i++)
		if (copy_into_work(r, &o->copies[i]))
			return -1;

	/*
	 * stdout and stderr are unnamed files in the work tmpfs (O_TMPFILE): the
	 * program inherits them as fd 1 and 2 and cannot reach them by path; the
	 * supervisor reads them through its own descriptors after the run.
	 */
	r->out_fd = open(r->workdir, O_TMPFILE | O_RDWR | O_CLOEXEC, 0600);
	r->err_fd = open(r->workdir, O_TMPFILE | O_RDWR | O_CLOEXEC, 0600);
	if (r->out_fd < 0 || r->err_fd < 0)
		return sbx_fail("open output files: %m");
	if (fcntl(r->out_fd, F_SETFL, O_APPEND) || fcntl(r->err_fd, F_SETFL, O_APPEND))
		return sbx_fail("fcntl O_APPEND: %m");
	return 0;
}

/* Maps SBX_ID in the child to the supervisor's own id; a single id needs no privilege. */
static int write_idmaps(pid_t pid)
{
	char path[64], map[32];

	snprintf(path, sizeof(path), "/proc/%d/setgroups", (int)pid);
	if (write_file(path, "deny"))
		return sbx_fail("write %s: %m", path);
	snprintf(path, sizeof(path), "/proc/%d/uid_map", (int)pid);
	snprintf(map, sizeof(map), "%u %u 1\n", SBX_ID, geteuid());
	if (write_file(path, map))
		return sbx_fail("write %s: %m", path);
	snprintf(path, sizeof(path), "/proc/%d/gid_map", (int)pid);
	snprintf(map, sizeof(map), "%u %u 1\n", SBX_ID, getegid());
	if (write_file(path, map))
		return sbx_fail("write %s: %m", path);
	return 0;
}

static int spawn_child(struct run *r)
{
	struct clone_args ca = { 0 };
	struct msg go = { .type = MSG_GO };
	int pipefd[2], pidfd = -1;
	long pid;

	if (pipe2(pipefd, O_CLOEXEC))
		return sbx_fail("pipe2: %m");
	r->in_r = pipefd[0];
	r->in_w = pipefd[1];
	if (socketpair(AF_UNIX, SOCK_SEQPACKET | SOCK_CLOEXEC, 0, r->sock))
		return sbx_fail("socketpair: %m");

	/*
	 * Without a stack clone3(2) behaves like fork(). The new user namespace
	 * grants the privilege to create the others; the network namespace has
	 * no interfaces. CLONE_PIDFD gives a pidfd for race-free signalling.
	 */
	ca.flags = CLONE_NEWUSER | CLONE_NEWPID | CLONE_NEWNS | CLONE_NEWNET | CLONE_NEWUTS |
		   CLONE_NEWIPC | CLONE_PIDFD;
	ca.pidfd = (uint64_t)(uintptr_t)&pidfd;
	ca.exit_signal = SIGCHLD;
	fflush(NULL);
	pid = syscall(SYS_clone3, &ca, sizeof(ca));
	if (pid < 0)
		return sbx_fail("clone3: %m");
	if (!pid)
		child_main(r);
	r->pid = (pid_t)pid;
	r->pidfd = pidfd;

	if (write_idmaps(r->pid) || cgroup_add_pid(&r->cg, r->pid))
		return -1;
	if (send(r->sock[0], &go, sizeof(go), MSG_NOSIGNAL) != (ssize_t)sizeof(go))
		return sbx_fail("send go: %m");
	close_fd(&r->in_r);	/* the child holds the read end now */
	return 0;
}

static void cleanup(struct run *r)
{
	int status;

	/* supervise() reaps the child on the normal path; error paths kill it here. */
	if (r->pid > 0) {
		if (r->pidfd >= 0)
			syscall(SYS_pidfd_send_signal, r->pidfd, SIGKILL, NULL, 0);
		else
			kill(r->pid, SIGKILL);
		cgroup_kill(&r->cg);
		waitpid(r->pid, &status, 0);
	}
	close_fd(&r->pidfd);
	close_fd(&r->notify_fd);
	close_fd(&r->sock[0]);
	close_fd(&r->sock[1]);
	close_fd(&r->in_r);
	close_fd(&r->in_w);
	close_fd(&r->out_fd);
	close_fd(&r->err_fd);
	cgroup_destroy(&r->cg);

	if (r->work_mounted)
		umount2(r->workdir, MNT_DETACH);
	if (r->rundir[0]) {
		rmdir(r->workdir);
		rmdir(r->rootdir);
		if (rmdir(r->rundir))
			sbx_log("cannot remove %s: %m", r->rundir);
	}
}

int run_execute(const struct opts *o, const unsigned char *input, size_t len, struct result *res)
{
	const char *name;
	struct run r;
	int ret = -1;

	run_init(&r, o, input, len);
	memset(res, 0, sizeof(*res));
	res->exit_code = -1;

	if (enter_helper_namespace() || prepare_dirs(&r))
		goto out;
	name = strrchr(r.rundir, '/') + 1;
	if (cgroup_create(&r.cg, o->cgroup_root, name, &o->limits) || spawn_child(&r))
		goto out;
	ret = supervise(&r, res);
out:
	if (ret)
		res->reason = TR_INTERNAL_ERROR;
	cleanup(&r);
	return ret;
}

int run_sweep(const struct opts *o)
{
	struct dirent *e;
	int cgroups, dirs = 0;
	DIR *d;

	cgroups = cgroup_sweep(o->cgroup_root);
	if (cgroups < 0)
		return -1;
	d = opendir(o->run_dir);
	if (!d)
		return sbx_fail("opendir %s: %m", o->run_dir);
	while ((e = readdir(d))) {
		char path[PATH_MAX], sub[PATH_MAX + 8];

		if (strncmp(e->d_name, "sbx-", 4) || owner_alive(e->d_name))
			continue;
		if (pathf(path, sizeof(path), "%s/%s", o->run_dir, e->d_name))
			continue;
		/* Mounts die with their namespace, so a dead run leaves empty directories. */
		snprintf(sub, sizeof(sub), "%s/work", path);
		rmdir(sub);
		snprintf(sub, sizeof(sub), "%s/root", path);
		rmdir(sub);
		if (!rmdir(path))
			dirs++;
	}
	closedir(d);
	sbx_log("swept %d cgroup(s) and %d run director%s", cgroups, dirs, dirs == 1 ? "y" : "ies");
	return 0;
}
