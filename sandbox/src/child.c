// SPDX-License-Identifier: MIT
/*
 * Sandbox child, PID 1 of the new PID namespace.
 *
 * Order: wait for the supervisor (id maps written, child placed in its
 * cgroup), build the root file system and pivot into it, apply rlimits, wire
 * up stdio, drop every privilege, load the seccomp filter, execve().
 * Everything that needs a capability happens before the drop.
 */
#include <errno.h>
#include <fcntl.h>
#include <linux/capability.h>
#include <signal.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mount.h>
#include <sys/prctl.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/statvfs.h>
#include <sys/syscall.h>
#include <unistd.h>

#include "sbx.h"

#ifndef SYS_close_range
#define SYS_close_range 436
#endif
#ifndef CLOSE_RANGE_CLOEXEC
#define CLOSE_RANGE_CLOEXEC (1U << 2)
#endif
/* mount_setattr(2), Linux 5.12; older libc headers lack it. */
#ifndef SYS_mount_setattr
#define SYS_mount_setattr 442
#endif
#ifndef AT_RECURSIVE
#define AT_RECURSIVE 0x8000
#endif

struct sbx_mount_attr {
	uint64_t attr_set;
	uint64_t attr_clr;
	uint64_t propagation;
	uint64_t userns_fd;
};

#define SBX_MOUNT_ATTR_RDONLY	0x1ULL

/* Top-level directories of the new root that hold bound host trees (fallback walk only). */
static const char *const ro_roots[] = {
	"usr", "bin", "sbin", "lib", "lib32", "lib64", "libx32", "etc",
};

static __printf(2, 3) __noreturn void die(struct run *r, const char *fmt, ...)
{
	struct msg m = { .type = MSG_ERR };
	va_list ap;

	va_start(ap, fmt);
	vsnprintf(m.text, sizeof(m.text), fmt, ap);
	va_end(ap);
	send(r->sock[1], &m, sizeof(m), MSG_NOSIGNAL);
	_exit(SBX_EXIT_INTERNAL);
}

/* Absolute path of rel in the new root. The buffer is reused by the next call. */
static const char *rpath(struct run *r, const char *rel)
{
	static char buf[PATH_MAX];

	if (pathf(buf, sizeof(buf), "%s/%s", r->rootdir, rel))
		die(r, "%s", sbx_error());
	return buf;
}

static void do_mkdir(struct run *r, const char *rel)
{
	const char *p = rpath(r, rel);

	if (mkdir(p, 0755) && errno != EEXIST)
		die(r, "mkdir %s: %m", p);
}

static void do_touch(struct run *r, const char *rel)
{
	const char *p = rpath(r, rel);
	int fd = open(p, O_WRONLY | O_CREAT | O_CLOEXEC, 0644);

	if (fd < 0)
		die(r, "create %s: %m", p);
	close(fd);
}

static void do_mount(struct run *r, const char *src, const char *dst, const char *type,
		     unsigned long flags, const char *data)
{
	if (mount(src, dst, type, flags, data))
		die(r, "mount %s on %s: %m", type ? type : src, dst);
}

/* Mount flags a remount has to repeat: a user namespace cannot clear locked ones. */
static unsigned long keep_flags(const struct statvfs *sv)
{
	static const struct {
		unsigned long st;
		unsigned long ms;
	} map[] = {
		{ ST_NOSUID, MS_NOSUID },
		{ ST_NODEV, MS_NODEV },
		{ ST_NOEXEC, MS_NOEXEC },
		{ ST_NOATIME, MS_NOATIME },
		{ ST_NODIRATIME, MS_NODIRATIME },
		{ ST_RELATIME, MS_RELATIME },
	};
	unsigned long flags = 0;
	size_t i;

	for (i = 0; i < ARRAY_SIZE(map); i++)
		if (sv->f_flag & map[i].st)
			flags |= map[i].ms;
	return flags;
}

/* MS_RDONLY is ignored on the initial bind; a second MS_REMOUNT call applies it. */
static void remount_ro(struct run *r, const char *path)
{
	struct statvfs sv;

	if (statvfs(path, &sv))
		die(r, "statvfs %s: %m", path);
	if (mount(NULL, path, NULL, MS_BIND | MS_REMOUNT | MS_RDONLY | keep_flags(&sv), NULL))
		die(r, "remount %s read-only: %m", path);
}

/* Decodes the octal escapes of mountinfo paths (\040 is a space). */
static void unescape_mountinfo(char *s)
{
	char *w = s;

	while (*s) {
		if (s[0] == '\\' && s[1] >= '0' && s[1] <= '3' && s[2] >= '0' && s[2] <= '7' &&
		    s[3] >= '0' && s[3] <= '7') {
			*w++ = (char)(((s[1] - '0') << 6) | ((s[2] - '0') << 3) | (s[3] - '0'));
			s += 4;
		} else {
			*w++ = *s++;
		}
	}
	*w = '\0';
}

static bool is_ro_root(const char *rel)
{
	size_t i, n = strcspn(rel, "/");

	for (i = 0; i < ARRAY_SIZE(ro_roots); i++)
		if (strlen(ro_roots[i]) == n && !strncmp(rel, ro_roots[i], n))
			return true;
	return false;
}

/*
 * Fallback for kernels without mount_setattr(2): makes every mount below the
 * bound directories read-only in one pass over mountinfo.
 */
static void remount_ro_trees(struct run *r)
{
	size_t root_len = strlen(r->rootdir);
	char *line = NULL, mp[PATH_MAX];
	size_t cap = 0;
	FILE *f;

	f = fopen("/proc/self/mountinfo", "re");
	if (!f)
		die(r, "open mountinfo: %m");
	while (getline(&line, &cap, f) > 0) {
		/* id parent major:minor root mountpoint ... */
		if (sscanf(line, "%*d %*d %*u:%*u %*s %4095s", mp) != 1)
			continue;
		unescape_mountinfo(mp);
		if (strncmp(mp, r->rootdir, root_len) || mp[root_len] != '/')
			continue;
		if (is_ro_root(mp + root_len + 1))
			remount_ro(r, mp);
	}
	free(line);
	fclose(f);
}

/*
 * Makes the bound trees read-only. A recursive bind brings the host's
 * sub-mounts along (a user namespace refuses a non-recursive bind of a tree
 * that has them), and each is a separate mount with its own flags; the
 * recursive mount_setattr() reaches all of them in one call per tree.
 */
static void make_read_only(struct run *r, const char *const *bound, size_t n)
{
	struct sbx_mount_attr attr = { .attr_set = SBX_MOUNT_ATTR_RDONLY };
	size_t i;

	for (i = 0; i < n; i++)
		if (syscall(SYS_mount_setattr, AT_FDCWD, rpath(r, bound[i]), AT_RECURSIVE, &attr,
			    sizeof(attr)))
			break;
	if (i < n)
		remount_ro_trees(r);
}

static void build_root(struct run *r)
{
	static const char *const toplevel[] = { "bin", "sbin", "lib", "lib32", "lib64", "libx32" };
	static const char *const devnodes[] = { "null", "zero", "full", "urandom", "random" };
	static const char *const masked[] = { "kcore", "keys", "timer_list", "sched_debug" };
	static const char *const stdio_links[] = { "stdin", "stdout", "stderr" };
	const char *bound[ARRAY_SIZE(toplevel) + 2];
	char host[32], rel[32], target[PATH_MAX];
	size_t i, nbound = 0;
	struct stat st;
	ssize_t n;

	/*
	 * The new root is a tmpfs mounted here: mounts inherited from the
	 * supervisor are locked and cannot be a pivot_root target.
	 */
	do_mount(r, "tmpfs", r->rootdir, "tmpfs", MS_NOSUID | MS_NODEV,
		 "size=1m,nr_inodes=256,mode=0755");

	/* /work is the supervisor's capped tmpfs; the bind shares its pages. */
	do_mkdir(r, "work");
	do_mount(r, r->workdir, rpath(r, "work"), NULL, MS_BIND, NULL);

	do_mkdir(r, "usr");
	do_mount(r, "/usr", rpath(r, "usr"), NULL, MS_BIND | MS_REC, NULL);
	bound[nbound++] = "usr";

	/* Merged-/usr hosts have /bin, /lib, ... as symlinks into /usr. */
	for (i = 0; i < ARRAY_SIZE(toplevel); i++) {
		snprintf(host, sizeof(host), "/%s", toplevel[i]);
		if (lstat(host, &st))
			continue;
		if (S_ISLNK(st.st_mode)) {
			n = readlink(host, target, sizeof(target) - 1);
			if (n < 0)
				die(r, "readlink %s: %m", host);
			target[n] = '\0';
			if (symlink(target, rpath(r, toplevel[i])))
				die(r, "symlink %s: %m", toplevel[i]);
		} else if (S_ISDIR(st.st_mode)) {
			do_mkdir(r, toplevel[i]);
			do_mount(r, host, rpath(r, toplevel[i]), NULL, MS_BIND | MS_REC, NULL);
			bound[nbound++] = toplevel[i];
		}
	}

	/* From /etc only the loader cache: no passwd, hosts or shadow. */
	do_mkdir(r, "etc");
	if (!stat("/etc/ld.so.cache", &st)) {
		do_touch(r, "etc/ld.so.cache");
		do_mount(r, "/etc/ld.so.cache", rpath(r, "etc/ld.so.cache"), NULL, MS_BIND, NULL);
		bound[nbound++] = "etc/ld.so.cache";
	}

	/* /dev: a tiny tmpfs holding bind-mounted harmless device nodes. */
	do_mkdir(r, "dev");
	do_mount(r, "tmpfs", rpath(r, "dev"), "tmpfs", MS_NOSUID | MS_NOEXEC,
		 "size=64k,nr_inodes=32,mode=0755");
	for (i = 0; i < ARRAY_SIZE(devnodes); i++) {
		snprintf(host, sizeof(host), "/dev/%s", devnodes[i]);
		snprintf(rel, sizeof(rel), "dev/%s", devnodes[i]);
		do_touch(r, rel);
		do_mount(r, host, rpath(r, rel), NULL, MS_BIND, NULL);
	}
	if (symlink("/proc/self/fd", rpath(r, "dev/fd")))
		die(r, "symlink dev/fd: %m");
	for (i = 0; i < ARRAY_SIZE(stdio_links); i++) {
		snprintf(target, sizeof(target), "/proc/self/fd/%zu", i);
		snprintf(rel, sizeof(rel), "dev/%s", stdio_links[i]);
		if (symlink(target, rpath(r, rel)))
			die(r, "symlink %s: %m", rel);
	}
	/* The tmpfs becomes read-only; the device nodes on top stay writable. */
	remount_ro(r, rpath(r, "dev"));

	/* procfs of this PID namespace: the program sees only its own processes. */
	do_mkdir(r, "proc");
	do_mount(r, "proc", rpath(r, "proc"), "proc", MS_NOSUID | MS_NODEV | MS_NOEXEC, NULL);
	for (i = 0; i < ARRAY_SIZE(masked); i++) {
		snprintf(rel, sizeof(rel), "proc/%s", masked[i]);
		mount("/dev/null", rpath(r, rel), NULL, MS_BIND, NULL);	/* best effort */
	}

	do_mkdir(r, ".oldroot");
	make_read_only(r, bound, nbound);
}

static void enter_root(struct run *r)
{
	/* The old root ends up in .oldroot and is detached right away. */
	if (syscall(SYS_pivot_root, r->rootdir, rpath(r, ".oldroot")))
		die(r, "pivot_root: %m");
	if (chdir("/"))
		die(r, "chdir /: %m");
	if (umount2("/.oldroot", MNT_DETACH))
		die(r, "umount old root: %m");
	if (rmdir("/.oldroot"))
		die(r, "rmdir /.oldroot: %m");
	remount_ro(r, "/");
	if (chdir("/work"))
		die(r, "chdir /work: %m");
}

static void set_rlimits(struct run *r)
{
	/*
	 * RLIMIT_CPU is only a backstop: whole seconds, per process. It sits one
	 * second above the budget so the exact check on cpu.stat in the
	 * supervisor always fires first. soft == hard makes the kernel send
	 * SIGKILL; PID 1 ignores SIGXCPU.
	 */
	struct rlimit cpu = { (r->opts->limits.cpu_time_ms + 999) / 1000 + 1, 0 };
	struct rlimit none = { 0, 0 };
	struct rlimit nofile = { 1024, 1024 };

	cpu.rlim_max = cpu.rlim_cur;
	if (setrlimit(RLIMIT_CPU, &cpu) || setrlimit(RLIMIT_CORE, &none) ||
	    setrlimit(RLIMIT_NOFILE, &nofile))
		die(r, "setrlimit: %m");
}

static void drop_privileges(struct run *r)
{
	struct __user_cap_header_struct hdr = { _LINUX_CAPABILITY_VERSION_3, 0 };
	struct __user_cap_data_struct data[2] = { { 0 } };
	int cap;

	/* Without no_new_privs a setuid binary could regain privilege. */
	if (prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0))
		die(r, "PR_SET_NO_NEW_PRIVS: %m");
	/* Bounding set first (needs CAP_SETPCAP), then the three process sets. */
	for (cap = 0; cap < 64; cap++) {
		if (prctl(PR_CAPBSET_DROP, cap, 0, 0, 0)) {
			if (errno == EINVAL)
				break;	/* past the last capability of this kernel */
			die(r, "PR_CAPBSET_DROP %d: %m", cap);
		}
	}
	prctl(PR_CAP_AMBIENT, PR_CAP_AMBIENT_CLEAR_ALL, 0, 0, 0);
	if (syscall(SYS_capset, &hdr, data))
		die(r, "capset: %m");
}

void child_main(struct run *r)
{
	/* The program's whole environment; nothing is inherited from the caller. */
	static char e_path[] = "PATH=/usr/bin:/bin";
	static char e_home[] = "HOME=/work";
	static char e_hash[] = "PYTHONHASHSEED=0";
	static char e_lang[] = "LC_ALL=C.UTF-8";
	static char e_tz[] = "TZ=UTC";
	static char *const envp[] = { e_path, e_home, e_hash, e_lang, e_tz, NULL };
	struct msg m = { 0 };
	sigset_t none;
	int nfd = -1;

	close(r->sock[0]);
	/* If the supervisor is SIGKILLed the sandbox goes with it. */
	prctl(PR_SET_PDEATHSIG, SIGKILL);

	if (recv(r->sock[1], &m, sizeof(m), 0) != (ssize_t)sizeof(m) || m.type != MSG_GO)
		_exit(SBX_EXIT_INTERNAL);

	sigemptyset(&none);
	sigprocmask(SIG_SETMASK, &none, NULL);
	signal(SIGPIPE, SIG_DFL);
	umask(022);

	if (sethostname("sandbox", 7))
		die(r, "sethostname: %m");
	/* Keep mount events from propagating back to the supervisor's namespace. */
	if (mount(NULL, "/", NULL, MS_REC | MS_PRIVATE, NULL))
		die(r, "make mounts private: %m");

	build_root(r);
	enter_root(r);
	set_rlimits(r);

	if (dup2(r->in_r, STDIN_FILENO) < 0 || dup2(r->out_fd, STDOUT_FILENO) < 0 ||
	    dup2(r->err_fd, STDERR_FILENO) < 0)
		die(r, "dup2: %m");
	/* Everything else (pipe ends, pidfd, setup channel) is closed by execve(). */
	if (syscall(SYS_close_range, 3U, ~0U, CLOSE_RANGE_CLOEXEC))
		die(r, "close_range: %m");

	drop_privileges(r);

	if (filter_install(&nfd))
		die(r, "%s", sbx_error());
	/*
	 * execve() closes the listener, so the supervisor has to copy it first
	 * (pidfd_getfd) and acknowledge. Only recv/send are used from here on.
	 */
	m = (struct msg){ .type = MSG_NOTIFY_FD, .value = nfd };
	if (send(r->sock[1], &m, sizeof(m), MSG_NOSIGNAL) != (ssize_t)sizeof(m) ||
	    recv(r->sock[1], &m, sizeof(m), 0) != (ssize_t)sizeof(m) || m.type != MSG_ACK)
		_exit(SBX_EXIT_INTERNAL);

	execve(r->opts->cmd[0], r->opts->cmd, envp);
	die(r, "execve %s: %m", r->opts->cmd[0]);
}
