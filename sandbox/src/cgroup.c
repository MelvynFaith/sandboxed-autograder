// SPDX-License-Identifier: MIT
/*
 * One cgroup v2 sub-cgroup per execution.
 *
 * The parent cgroup must be delegated to the service user, have the memory,
 * pids and cpu controllers enabled in cgroup.subtree_control and hold no
 * processes itself (the cgroup v2 "no internal process" rule).
 */
#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <inttypes.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

#include "sbx.h"

static const struct {
	const char *name;
	int flags;
	bool required;
} cg_files[CG_NFILES] = {
	[CG_PROCS]	= { "cgroup.procs",	O_WRONLY, true },
	[CG_CPU_STAT]	= { "cpu.stat",		O_RDONLY, true },
	[CG_MEM_CURRENT] = { "memory.current",	O_RDONLY, true },
	[CG_MEM_PEAK]	= { "memory.peak",	O_RDONLY, false },	/* Linux 5.19 */
	[CG_MEM_EVENTS]	= { "memory.events",	O_RDONLY, true },
	[CG_PIDS_EVENTS] = { "pids.events",	O_RDONLY, true },
	[CG_KILL]	= { "cgroup.kill",	O_WRONLY, false },	/* Linux 5.14 */
};

void cgroup_init(struct cgroup *cg)
{
	int i;

	memset(cg, 0, sizeof(*cg));
	for (i = 0; i < CG_NFILES; i++)
		cg->fd[i] = -1;
}

static __printf(4, 5) int ctl_write(const struct cgroup *cg, const char *name, bool required,
				    const char *fmt, ...)
{
	char path[PATH_MAX], val[64];
	va_list ap;

	va_start(ap, fmt);
	vsnprintf(val, sizeof(val), fmt, ap);
	va_end(ap);
	if (pathf(path, sizeof(path), "%s/%s", cg->path, name))
		return -1;
	if (!write_file(path, val))
		return 0;
	if (errno == ENOENT && !required)
		return 0;
	if (errno == ENOENT)
		return sbx_fail("%s missing: enable memory, pids and cpu in the parent's "
				"cgroup.subtree_control", name);
	return sbx_fail("write %s to %s: %m", val, path);
}

int cgroup_create(struct cgroup *cg, const char *root, const char *name, const struct limits *l)
{
	char path[PATH_MAX];
	int i;

	cgroup_init(cg);
	if (pathf(cg->path, sizeof(cg->path), "%s/%s", root, name))
		return -1;
	/* mkdir(2) in cgroupfs creates the cgroup. */
	if (mkdir(cg->path, 0755))
		return sbx_fail("mkdir %s: %m", cg->path);
	cg->created = true;

	if (ctl_write(cg, "memory.max", true, "%" PRIu64, l->memory_mb << 20) ||
	    /* Without swap.max=0 a process can exceed memory.max by paging out. */
	    ctl_write(cg, "memory.swap.max", false, "0") ||
	    /* On OOM, kill every task of the cgroup together. */
	    ctl_write(cg, "memory.oom.group", false, "1") ||
	    ctl_write(cg, "pids.max", true, "%" PRIu64, l->max_processes) ||
	    /* Bandwidth only: one CPU. The CPU-time budget is enforced by the supervisor. */
	    ctl_write(cg, "cpu.max", true, "100000 100000"))
		return -1;

	for (i = 0; i < CG_NFILES; i++) {
		if (pathf(path, sizeof(path), "%s/%s", cg->path, cg_files[i].name))
			return -1;
		cg->fd[i] = open(path, cg_files[i].flags | O_CLOEXEC);
		if (cg->fd[i] < 0 && (cg_files[i].required || errno != ENOENT))
			return sbx_fail("open %s: %m", path);
	}
	return 0;
}

int cgroup_add_pid(struct cgroup *cg, pid_t pid)
{
	char buf[16];
	int n = snprintf(buf, sizeof(buf), "%d", (int)pid);

	/*
	 * The kernel requires write access to cgroup.procs of the common
	 * ancestor of source and destination, so the caller must itself live
	 * below the delegated cgroup.
	 */
	if (write(cg->fd[CG_PROCS], buf, (size_t)n) != n)
		return sbx_fail("move pid %d into %s: %m", (int)pid, cg->path);
	return 0;
}

static int64_t read_int(int fd)
{
	char buf[64];

	if (fd < 0 || pread_str(fd, buf, sizeof(buf)) < 0)
		return -1;
	return strtoll(buf, NULL, 10);
}

int64_t cgroup_cpu_usec(const struct cgroup *cg)
{
	char buf[512];
	int64_t v;

	if (pread_str(cg->fd[CG_CPU_STAT], buf, sizeof(buf)) < 0 || !kv_get(buf, "usage_usec", &v))
		return -1;
	return v;
}

int64_t cgroup_mem_current(const struct cgroup *cg)
{
	return read_int(cg->fd[CG_MEM_CURRENT]);
}

int64_t cgroup_mem_peak(const struct cgroup *cg)
{
	return read_int(cg->fd[CG_MEM_PEAK]);
}

int64_t cgroup_counter(const struct cgroup *cg, enum cg_file file, const char *key)
{
	char buf[512];
	int64_t v;

	if (cg->fd[file] < 0 || pread_str(cg->fd[file], buf, sizeof(buf)) < 0 ||
	    !kv_get(buf, key, &v))
		return 0;
	return v;
}

void cgroup_kill(const struct cgroup *cg)
{
	if (cg->fd[CG_KILL] >= 0 && write(cg->fd[CG_KILL], "1", 1) < 0)
		return;	/* nothing left to kill, or the group is already gone */
}

void cgroup_destroy(struct cgroup *cg)
{
	int i;

	if (!cg->created)
		return;
	cgroup_kill(cg);
	for (i = 0; i < CG_NFILES; i++)
		close_fd(&cg->fd[i]);
	/* rmdir fails with EBUSY until exiting tasks are reaped. */
	for (i = 0; i < 400; i++) {
		if (!rmdir(cg->path) || errno == ENOENT) {
			cg->created = false;
			return;
		}
		if (errno != EBUSY)
			break;
		usleep(5000);
	}
	sbx_log("cannot remove cgroup %s: %m", cg->path);
}

int cgroup_sweep(const char *root)
{
	struct dirent *e;
	int removed = 0;
	DIR *d;

	d = opendir(root);
	if (!d)
		return sbx_fail("opendir %s: %m", root);
	while ((e = readdir(d))) {
		char path[PATH_MAX], kill_path[PATH_MAX + 16];
		int i;

		if (strncmp(e->d_name, "sbx-", 4) || owner_alive(e->d_name))
			continue;
		if (pathf(path, sizeof(path), "%s/%s", root, e->d_name))
			continue;
		snprintf(kill_path, sizeof(kill_path), "%s/cgroup.kill", path);
		write_file(kill_path, "1");
		for (i = 0; i < 100; i++) {
			if (!rmdir(path)) {
				removed++;
				break;
			}
			if (errno != EBUSY)
				break;
			usleep(5000);
		}
	}
	closedir(d);
	return removed;
}
