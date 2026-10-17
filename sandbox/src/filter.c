// SPDX-License-Identifier: MIT
/*
 * seccomp-BPF filter for the sandboxed program.
 *
 * Allowlist: the default action is SCMP_ACT_NOTIFY, so a syscall that is not
 * listed blocks its caller and is reported to the supervisor through the
 * listener fd, which reads seccomp_notif.data.nr for the audit log (FR-12).
 * SCMP_ACT_TRAP does not work for this: SIGSYS is delivered to the offending
 * process, not to the supervisor, and execve() resets the handler.
 *
 * Where notification is not available the default action is SCMP_ACT_TRACE
 * and the supervisor is a ptrace tracer instead (filter_install(), SRS risk
 * R12). The allowlist is the same.
 *
 * The list is what CPython 3.11+ and its standard library need with no
 * network, mount, ptrace, namespace or credential changes. Extending it is a
 * security-sensitive change: say in the review what the syscall exposes.
 * To find what a program needs, run it under `strace -f -c` outside the sandbox.
 */
#include <errno.h>
#include <fcntl.h>
#include <sched.h>
#include <seccomp.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/prctl.h>
#include <sys/socket.h>

#include "sbx.h"

#ifndef CLONE_NEWTIME
#define CLONE_NEWTIME 0x00000080
#endif

/* _IOR('T', 0x2A, struct termios2); written out because userspace leaves termios2 incomplete. */
#define SBX_TCGETS2 0x802c542aUL

static const char *const allowed[] = {
	/* descriptors and I/O */
	"read", "write", "readv", "writev", "pread64", "pwrite64", "preadv", "pwritev", "preadv2",
	"pwritev2", "open", "openat", "creat", "close", "close_range", "lseek", "dup", "dup2",
	"dup3", "pipe", "pipe2", "fcntl", "flock", "fsync", "fdatasync", "sync_file_range",
	"ftruncate", "truncate", "fallocate", "fadvise64", "readahead", "sendfile",
	"copy_file_range", "splice", "tee", "memfd_create",
	/* readiness and fd-based timers */
	"poll", "ppoll", "select", "pselect6", "epoll_create", "epoll_create1", "epoll_ctl",
	"epoll_wait", "epoll_pwait", "epoll_pwait2", "eventfd", "eventfd2", "timerfd_create",
	"timerfd_settime", "timerfd_gettime",
	/* file metadata inside the sandbox root */
	"stat", "fstat", "lstat", "newfstatat", "statx", "access", "faccessat", "faccessat2",
	"readlink", "readlinkat", "getdents", "getdents64", "getcwd", "chdir", "fchdir", "mkdir",
	"mkdirat", "rmdir", "unlink", "unlinkat", "rename", "renameat", "renameat2", "link",
	"linkat", "symlink", "symlinkat", "chmod", "fchmod", "fchmodat", "fchmodat2", "chown",
	"fchown", "lchown", "fchownat", "utime", "utimes", "utimensat", "futimesat", "umask",
	"statfs", "fstatfs", "getxattr", "lgetxattr", "fgetxattr", "listxattr", "llistxattr",
	"flistxattr",
	/* memory */
	"brk", "mmap", "mprotect", "munmap", "mremap", "madvise", "mincore", "msync", "membarrier",
	/* processes, threads, signals; pidfds are needed by Python 3.14 subprocess */
	"pidfd_open", "pidfd_send_signal", "fork", "vfork", "execve", "execveat", "exit",
	"exit_group", "wait4", "waitid", "getpid", "getppid", "gettid", "getpgid", "getpgrp",
	"getsid", "setpgid", "setsid", "kill", "tkill", "tgkill", "rt_sigaction", "rt_sigprocmask",
	"rt_sigreturn", "rt_sigsuspend", "rt_sigpending", "rt_sigtimedwait", "rt_sigqueueinfo",
	"rt_tgsigqueueinfo", "sigaltstack", "signalfd", "signalfd4", "pause", "set_tid_address",
	"set_robust_list", "get_robust_list", "futex", "futex_waitv", "futex_wake", "futex_wait",
	"futex_requeue", "rseq", "sched_yield", "sched_getaffinity", "sched_getparam",
	"sched_getscheduler", "sched_get_priority_max", "sched_get_priority_min",
	"sched_rr_get_interval", "sched_getattr", "getcpu", "restart_syscall", "arch_prctl",
	"set_thread_area", "get_thread_area",
	/* time */
	"clock_gettime", "clock_getres", "clock_nanosleep", "gettimeofday", "time", "nanosleep",
	"getitimer", "setitimer", "alarm", "times", "timer_create", "timer_settime",
	"timer_gettime", "timer_getoverrun", "timer_delete",
	/* identity, limits, information */
	"getuid", "geteuid", "getgid", "getegid", "getgroups", "getresuid", "getresgid",
	"getrlimit", "setrlimit", "prlimit64", "getrusage", "getpriority", "uname", "sysinfo",
	"getrandom", "capget",
	/*
	 * Operations on connected AF_UNIX socketpairs (asyncio, multiprocessing).
	 * socket(), connect(), bind() and listen() stay denied: no new endpoint.
	 */
	"sendto", "recvfrom", "sendmsg", "recvmsg", "sendmmsg", "recvmmsg", "shutdown",
	"getsockopt", "setsockopt", "getsockname", "getpeername",
};

/* Refused with ENOSYS and no audit event, so libc falls back to the older call. */
static const char *const enosys[] = {
	"clone3",	/* flags are behind a pointer and cannot be checked; glibc retries clone */
	"openat2",
};

/*
 * Flags a clone() may not carry: a new namespace, and CLONE_UNTRACED, which would make the child
 * invisible to a ptrace-based supervisor (the fallback in filter_install()).
 */
#define NS_MASK	(CLONE_NEWNS | CLONE_NEWUSER | CLONE_NEWPID | CLONE_NEWNET | CLONE_NEWIPC | \
		 CLONE_NEWUTS | CLONE_NEWCGROUP | CLONE_NEWTIME | CLONE_UNTRACED)

/* Allowed only with the given argument; one entry per permitted value. */
static const struct {
	const char *name;
	unsigned int arg;
	enum scmp_compare op;
	scmp_datum_t a;
	scmp_datum_t b;
} arg_rules[] = {
	/* fork and threads, but no flag that creates a namespace */
	{ "clone", 0, SCMP_CMP_MASKED_EQ, NS_MASK, 0 },
	{ "socketpair", 0, SCMP_CMP_EQ, AF_UNIX, 0 },
	/* isatty(), terminal size, FIONREAD, non-blocking and close-on-exec flags */
	{ "ioctl", 1, SCMP_CMP_EQ, TCGETS, 0 },
	{ "ioctl", 1, SCMP_CMP_EQ, SBX_TCGETS2, 0 },
	{ "ioctl", 1, SCMP_CMP_EQ, TIOCGWINSZ, 0 },
	{ "ioctl", 1, SCMP_CMP_EQ, FIONREAD, 0 },
	{ "ioctl", 1, SCMP_CMP_EQ, FIONBIO, 0 },
	{ "ioctl", 1, SCMP_CMP_EQ, FIOCLEX, 0 },
	{ "ioctl", 1, SCMP_CMP_EQ, FIONCLEX, 0 },
	/* thread names, dumpable flag, timer slack */
	{ "prctl", 0, SCMP_CMP_EQ, PR_SET_NAME, 0 },
	{ "prctl", 0, SCMP_CMP_EQ, PR_GET_NAME, 0 },
	{ "prctl", 0, SCMP_CMP_EQ, PR_GET_DUMPABLE, 0 },
	{ "prctl", 0, SCMP_CMP_EQ, PR_SET_DUMPABLE, 0 },
	{ "prctl", 0, SCMP_CMP_EQ, PR_SET_TIMERSLACK, 0 },
	{ "prctl", 0, SCMP_CMP_EQ, PR_GET_TIMERSLACK, 0 },
	{ "prctl", 0, SCMP_CMP_EQ, PR_GET_TID_ADDRESS, 0 },
	{ "prctl", 0, SCMP_CMP_EQ, PR_GET_NO_NEW_PRIVS, 0 },
};

/* Syscalls missing on the build architecture resolve to a negative number and are skipped. */
static int add_rule(scmp_filter_ctx ctx, uint32_t action, const char *name,
		    const struct scmp_arg_cmp *cmp)
{
	int nr = seccomp_syscall_resolve_name(name);
	int rc;

	if (nr < 0)
		return 0;
	if (cmp)
		rc = seccomp_rule_add(ctx, action, nr, 1, *cmp);
	else
		rc = seccomp_rule_add(ctx, action, nr, 0);
	if (rc)
		return sbx_fail("seccomp rule for %s failed", name);
	return 0;
}

/*
 * Builds the allowlist with the given default action for everything else. Returns NULL without a
 * message when libseccomp does not know the action, and NULL with one when a rule cannot be added.
 */
static scmp_filter_ctx build(uint32_t deflt)
{
	scmp_filter_ctx ctx = seccomp_init(deflt);
	size_t i;

	if (!ctx)
		return NULL;
	/* Binary-tree layout: cheapest to build, and to evaluate without the syscall bitmap. */
	if (seccomp_attr_set(ctx, SCMP_FLTATR_CTL_OPTIMIZE, 2)) {
		sbx_fail("seccomp_attr_set failed");
		goto fail;
	}

	for (i = 0; i < ARRAY_SIZE(allowed); i++)
		if (add_rule(ctx, SCMP_ACT_ALLOW, allowed[i], NULL))
			goto fail;
	for (i = 0; i < ARRAY_SIZE(enosys); i++)
		if (add_rule(ctx, SCMP_ACT_ERRNO(ENOSYS), enosys[i], NULL))
			goto fail;
	for (i = 0; i < ARRAY_SIZE(arg_rules); i++) {
		struct scmp_arg_cmp cmp = SCMP_CMP(arg_rules[i].arg, arg_rules[i].op,
						   arg_rules[i].a, arg_rules[i].b);

		if (add_rule(ctx, SCMP_ACT_ALLOW, arg_rules[i].name, &cmp))
			goto fail;
	}
	return ctx;
fail:
	seccomp_release(ctx);
	return NULL;
}

/*
 * Installs the filter. The preferred default action is SCMP_ACT_NOTIFY;
 * *notify_fd receives the listener and *traced is false.
 *
 * When the kernel or libseccomp cannot do user notification (SRS risk R12)
 * the default action is SCMP_ACT_TRACE(0) instead, *notify_fd is -1 and
 * *traced is true: the supervisor must PTRACE_SEIZE the process before it
 * runs the program, and then sees each denied syscall as a
 * PTRACE_EVENT_SECCOMP stop. Without a tracer the kernel fails a traced
 * syscall with ENOSYS, so the filter still refuses.
 *
 * Building with -DSBX_FORCE_TRACE always takes the fallback, which is how
 * the suite exercises it.
 */
int filter_install(int *notify_fd, bool *traced)
{
	scmp_filter_ctx ctx;
	int rc;

#ifndef SBX_FORCE_TRACE
	ctx = build(SCMP_ACT_NOTIFY);
	if (!ctx && sbx_error()[0])
		return -1;
	if (ctx) {
		int fd;

		/* seccomp(SET_MODE_FILTER, FLAG_NEW_LISTENER); sets no_new_privs. */
		if (!seccomp_load(ctx)) {
			/* The context owns its fd and closes it on release: keep a CLOEXEC copy. */
			fd = fcntl(seccomp_notify_fd(ctx), F_DUPFD_CLOEXEC, 3);
			seccomp_release(ctx);
			if (fd < 0)
				return sbx_fail("seccomp listener: %m");
			*notify_fd = fd;
			*traced = false;
			return 0;
		}
		seccomp_release(ctx);
	}
#endif

	ctx = build(SCMP_ACT_TRACE(0));
	if (!ctx)
		return sbx_error()[0] ? -1 : sbx_fail("seccomp_init failed");
	rc = seccomp_load(ctx);
	seccomp_release(ctx);
	if (rc)
		return sbx_fail("seccomp_load failed: %s", strerror(-rc));
	*notify_fd = -1;
	*traced = true;
	return 0;
}
