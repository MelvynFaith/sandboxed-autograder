// SPDX-License-Identifier: MIT
/*
 * Command line parsing and validation of the resource limits.
 */
#include <ctype.h>
#include <errno.h>
#include <getopt.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "sbx.h"

/* Numeric options come first so that OPT_NUM + index selects the limit. */
static const struct {
	const char *name;
	size_t offset;
} num_opts[] = {
	{ "cpu-time-ms",	offsetof(struct limits, cpu_time_ms) },
	{ "timeout-s",		offsetof(struct limits, timeout_s) },
	{ "memory-mb",		offsetof(struct limits, memory_mb) },
	{ "max-processes",	offsetof(struct limits, max_processes) },
	{ "max-output-bytes",	offsetof(struct limits, max_output_bytes) },
	{ "tmpfs-size-mb",	offsetof(struct limits, tmpfs_size_mb) },
};

enum {
	OPT_NUM = 256,
	OPT_CGROUP_ROOT = OPT_NUM + ARRAY_SIZE(num_opts),
	OPT_COPY,
	OPT_RUN_DIR,
	OPT_RESULT,
	OPT_SWEEP,
	OPT_HELP,
};

static const char usage_text[] =
	"usage: sandbox-exec --cgroup-root DIR --cpu-time-ms N --timeout-s N --memory-mb N\n"
	"                    --max-processes N --max-output-bytes N --tmpfs-size-mb N\n"
	"                    [--copy SRC[:DEST]]... [--run-dir DIR] [--result FILE]\n"
	"                    -- CMD [ARG...]\n"
	"       sandbox-exec --sweep --cgroup-root DIR [--run-dir DIR]\n";

static int parse_u64(const char *name, const char *s, uint64_t *out)
{
	unsigned long long v;
	char *end;

	errno = 0;
	v = strtoull(s, &end, 10);
	if (!isdigit((unsigned char)*s) || errno || *end) {
		sbx_log("--%s: invalid number '%s'", name, s);
		return -1;
	}
	*out = v;
	return 0;
}

/* DEST must stay inside /work: relative, no empty, "." or ".." components. */
static bool dest_ok(const char *d)
{
	if (*d == '/')
		return false;
	for (;;) {
		size_t n = strcspn(d, "/");

		if (!n || (n == 1 && d[0] == '.') || (n == 2 && d[0] == '.' && d[1] == '.'))
			return false;
		if (!d[n])
			return true;
		d += n + 1;
	}
}

static int add_copy(struct opts *o, char *spec)
{
	struct copy_spec *c;
	char *colon = strrchr(spec, ':');
	const char *slash;

	c = realloc(o->copies, (o->ncopies + 1) * sizeof(*c));
	if (!c) {
		sbx_log("out of memory");
		return -1;
	}
	o->copies = c;
	c += o->ncopies;
	c->src = spec;
	if (colon && colon != spec) {
		*colon = '\0';
		c->dest = colon + 1;
	} else {
		slash = strrchr(spec, '/');
		c->dest = slash ? slash + 1 : spec;
	}
	if (!dest_ok(c->dest)) {
		sbx_log("--copy: bad destination '%s'", c->dest);
		return -1;
	}
	o->ncopies++;
	return 0;
}

static int limits_check(const struct limits *l)
{
	if (!l->cpu_time_ms || !l->timeout_s || !l->memory_mb || !l->max_processes ||
	    !l->max_output_bytes || !l->tmpfs_size_mb)
		return sbx_fail("every limit must be greater than zero");
	if (l->timeout_s > 3600 || l->cpu_time_ms > 3600 * 1000ULL)
		return sbx_fail("cpu-time-ms and timeout-s are capped at one hour");
	if (l->memory_mb > 1U << 20 || l->tmpfs_size_mb > 1U << 20 ||
	    l->max_processes > 1U << 20 || l->max_output_bytes > 1ULL << 40)
		return sbx_fail("a limit is out of range");
	/* The tmpfs must fill before the cgroup runs out of memory, or a disk bomb
	 * is misreported as a memory bomb; and it must hold both output files. */
	if (2 * l->max_output_bytes >= l->tmpfs_size_mb << 20)
		return sbx_fail("invariant violated: 2 * max_output_bytes < tmpfs_size_mb * 2^20");
	if (l->tmpfs_size_mb + SBX_HEADROOM_MB > l->memory_mb)
		return sbx_fail("invariant violated: tmpfs_size_mb + %d <= memory_mb",
				SBX_HEADROOM_MB);
	return 0;
}

int opts_parse(int argc, char **argv, struct opts *o)
{
	struct option lopts[ARRAY_SIZE(num_opts) + 7] = {
		{ "cgroup-root", required_argument, NULL, OPT_CGROUP_ROOT },
		{ "copy", required_argument, NULL, OPT_COPY },
		{ "run-dir", required_argument, NULL, OPT_RUN_DIR },
		{ "result", required_argument, NULL, OPT_RESULT },
		{ "sweep", no_argument, NULL, OPT_SWEEP },
		{ "help", no_argument, NULL, OPT_HELP },
	};
	unsigned int seen = 0;
	size_t i, n = 6;
	int c;

	memset(o, 0, sizeof(*o));
	o->run_dir = "/tmp";
	for (i = 0; i < ARRAY_SIZE(num_opts); i++, n++) {
		lopts[n].name = num_opts[i].name;
		lopts[n].has_arg = required_argument;
		lopts[n].val = OPT_NUM + (int)i;
	}

	/* "+": stop at the first non-option so CMD may follow without "--". */
	while ((c = getopt_long(argc, argv, "+", lopts, NULL)) != -1) {
		switch (c) {
		case OPT_CGROUP_ROOT:
			o->cgroup_root = optarg;
			break;
		case OPT_COPY:
			if (add_copy(o, optarg))
				return -1;
			break;
		case OPT_RUN_DIR:
			o->run_dir = optarg;
			break;
		case OPT_RESULT:
			o->result_path = optarg;
			break;
		case OPT_SWEEP:
			o->sweep = true;
			break;
		case OPT_HELP:
			fputs(usage_text, stdout);
			exit(0);
		default:
			if (c < OPT_NUM || c >= OPT_NUM + (int)ARRAY_SIZE(num_opts)) {
				fputs(usage_text, stderr);
				return -1;
			}
			i = (size_t)(c - OPT_NUM);
			if (parse_u64(num_opts[i].name, optarg,
				      (uint64_t *)((char *)&o->limits + num_opts[i].offset)))
				return -1;
			seen |= 1U << i;
		}
	}

	if (!o->cgroup_root) {
		sbx_log("--cgroup-root is required");
		return -1;
	}
	if (o->sweep)
		return 0;
	if (seen != (1U << ARRAY_SIZE(num_opts)) - 1) {
		sbx_log("all six limit options are required");
		fputs(usage_text, stderr);
		return -1;
	}
	if (limits_check(&o->limits)) {
		sbx_log("%s", sbx_error());
		return -1;
	}
	if (optind >= argc || argv[optind][0] != '/') {
		sbx_log("CMD must be an absolute path inside the sandbox");
		return -1;
	}
	o->cmd = &argv[optind];
	return 0;
}
