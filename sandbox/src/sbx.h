/* SPDX-License-Identifier: MIT */
/*
 * Declarations shared by the sandbox-exec modules.
 */
#ifndef SBX_H
#define SBX_H

#include <limits.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <sys/types.h>

#define __printf(a, b)	__attribute__((format(printf, a, b)))
#define __noreturn	__attribute__((noreturn))
#define ARRAY_SIZE(a)	(sizeof(a) / sizeof((a)[0]))

#define SBX_EXIT_USAGE		2
#define SBX_EXIT_INTERNAL	70
#define SBX_REPORT_VERSION	1

/* Memory reserved for the interpreter on top of the work tmpfs. */
#define SBX_HEADROOM_MB		16
/* Largest stdin accepted for the program. */
#define SBX_MAX_INPUT		(4U << 20)
/* uid and gid of the program inside its user namespace. */
#define SBX_ID			65534U

/* util.c */

void sbx_log(const char *fmt, ...) __printf(1, 2);
/* Records the first error message only, so the root cause survives cleanup. Returns -1. */
int sbx_fail(const char *fmt, ...) __printf(1, 2);
const char *sbx_error(void);

uint64_t now_ns(void);
/* snprintf() for paths: returns -1 and records an error on truncation. */
int pathf(char *dst, size_t size, const char *fmt, ...) __printf(3, 4);
int write_all(int fd, const void *buf, size_t len);
/* Single write(2) of str: what procfs and cgroupfs expect. */
int write_file(const char *path, const char *str);
ssize_t pread_str(int fd, char *buf, size_t size);
/* Looks up "key value" in a flat-keyed cgroup file. */
bool kv_get(const char *text, const char *key, int64_t *val);
void close_fd(int *fd);
/* True if name is "sbx-<pid>-..." and <pid> is alive. */
bool owner_alive(const char *name);

/* opts.c */

/* Same names and units as the ResourceLimit entity. */
struct limits {
	uint64_t cpu_time_ms;
	uint64_t timeout_s;
	uint64_t memory_mb;
	uint64_t max_processes;
	uint64_t max_output_bytes;	/* stdout and stderr together */
	uint64_t tmpfs_size_mb;
};

struct copy_spec {
	const char *src;	/* host path */
	const char *dest;	/* relative to /work */
};

struct opts {
	struct limits limits;
	const char *cgroup_root;
	const char *run_dir;
	const char *result_path;	/* NULL: report on stdout */
	struct copy_spec *copies;
	size_t ncopies;
	char **cmd;
	bool sweep;
};

/* Returns 0, or -1 after printing a message (the caller exits with SBX_EXIT_USAGE). */
int opts_parse(int argc, char **argv, struct opts *o);

/* cgroup.c */

enum cg_file {
	CG_PROCS,
	CG_CPU_STAT,
	CG_MEM_CURRENT,
	CG_MEM_PEAK,
	CG_MEM_EVENTS,
	CG_PIDS_EVENTS,
	CG_KILL,
	CG_NFILES,
};

struct cgroup {
	char path[PATH_MAX];
	bool created;
	int fd[CG_NFILES];	/* -1 when absent (memory.peak and cgroup.kill are optional) */
};

void cgroup_init(struct cgroup *cg);
int cgroup_create(struct cgroup *cg, const char *root, const char *name, const struct limits *l);
int cgroup_add_pid(struct cgroup *cg, pid_t pid);
int64_t cgroup_cpu_usec(const struct cgroup *cg);
int64_t cgroup_mem_current(const struct cgroup *cg);
int64_t cgroup_mem_peak(const struct cgroup *cg);
/* Counter from one of the *.events files, 0 when absent. */
int64_t cgroup_counter(const struct cgroup *cg, enum cg_file file, const char *key);
void cgroup_kill(const struct cgroup *cg);
void cgroup_destroy(struct cgroup *cg);
/* Removes empty sbx-* cgroups of dead runs. Returns their number, or -1. */
int cgroup_sweep(const char *root);

/* run.c, supervisor.c, report.c */

enum term_reason {
	TR_NONE,
	TR_EXITED,
	TR_SIGNALED,
	TR_TIMEOUT_WALL,
	TR_TIMEOUT_CPU,
	TR_MEMORY_LIMIT,
	TR_OUTPUT_LIMIT,
	TR_SECURITY_VIOLATION,
	TR_INTERNAL_ERROR,
};

enum mem_source {
	MEMSRC_NONE,
	MEMSRC_CGROUP_PEAK,
	MEMSRC_CGROUP_SAMPLED,
	MEMSRC_RUSAGE,
};

struct result {
	enum term_reason reason;
	int exit_code;		/* -1: did not exit normally */
	int term_signal;	/* 0: not killed by a signal */
	uint64_t wall_ms;
	uint64_t cpu_ms;
	uint64_t memory_kb;
	enum mem_source mem_source;
	char *out;		/* stdout, then stderr, at most max_output_bytes together */
	size_t out_len;
	char *err;
	size_t err_len;
	bool output_truncated;
	bool pids_limit_hit;
	bool has_violation;
	long syscall_nr;
};

struct run {
	const struct opts *opts;
	char rundir[PATH_MAX];	/* <run-dir>/sbx-<pid>-XXXXXX */
	char rootdir[PATH_MAX];	/* <rundir>/root: mount point of the child's root tmpfs */
	char workdir[PATH_MAX];	/* <rundir>/work: capped tmpfs made by the supervisor */
	struct cgroup cg;
	bool work_mounted;
	int out_fd;		/* program stdout: unnamed file in the work tmpfs */
	int err_fd;
	int in_r;		/* stdin pipe, read end for the child */
	int in_w;		/* write end, fed by the supervisor */
	int sock[2];		/* setup channel: [0] supervisor, [1] child */
	pid_t pid;
	int pidfd;
	int notify_fd;		/* seccomp listener */
	const unsigned char *input;
	size_t input_len;
};

/*
 * Setup channel between supervisor and child: SOCK_SEQPACKET, CLOEXEC, so
 * the program never sees it and EOF on the supervisor side means execve()
 * succeeded.
 */
enum msg_type {
	MSG_GO = 1,	/* supervisor: id maps written, child is in its cgroup */
	MSG_ERR,	/* child: setup or execve failed, text says why */
	MSG_NOTIFY_FD,	/* child: seccomp listener fd number in value */
	MSG_ACK,	/* supervisor: listener fd duplicated, execve() may proceed */
};

struct msg {
	int type;
	int value;
	char text[200];
};

int run_execute(const struct opts *o, const unsigned char *input, size_t len, struct result *res);
int run_sweep(const struct opts *o);
void result_free(struct result *res);

/* Never returns; failures are sent as MSG_ERR and end in _exit(SBX_EXIT_INTERNAL). */
void child_main(struct run *r) __noreturn;
/* Installs the seccomp allowlist; *notify_fd receives the listener (CLOEXEC). */
int filter_install(int *notify_fd);

void supervisor_signals(void);
/* Watchdog loop. Returns once the child is reaped; -1 on internal error. */
int supervise(struct run *r, struct result *res);

int report_write(const struct result *res, const char *path);

#endif /* SBX_H */
