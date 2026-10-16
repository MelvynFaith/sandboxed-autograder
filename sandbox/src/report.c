// SPDX-License-Identifier: MIT
/*
 * JSON report, written to the supervisor's stdout or to a file the sandboxed
 * program cannot reach.
 */
#include <errno.h>
#include <fcntl.h>
#include <inttypes.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

#include "sbx.h"

static const char *term_reason_name(enum term_reason r)
{
	switch (r) {
	case TR_EXITED:
		return "EXITED";
	case TR_SIGNALED:
		return "SIGNALED";
	case TR_TIMEOUT_WALL:
		return "TIMEOUT_WALL";
	case TR_TIMEOUT_CPU:
		return "TIMEOUT_CPU";
	case TR_MEMORY_LIMIT:
		return "MEMORY_LIMIT";
	case TR_OUTPUT_LIMIT:
		return "OUTPUT_LIMIT";
	case TR_SECURITY_VIOLATION:
		return "SECURITY_VIOLATION";
	case TR_NONE:
	case TR_INTERNAL_ERROR:
		break;
	}
	return "INTERNAL_ERROR";
}

static const char *mem_source_name(enum mem_source s)
{
	switch (s) {
	case MEMSRC_CGROUP_PEAK:
		return "CGROUP_PEAK";
	case MEMSRC_CGROUP_SAMPLED:
		return "CGROUP_SAMPLED";
	case MEMSRC_RUSAGE:
		return "RUSAGE";
	case MEMSRC_NONE:
		break;
	}
	return NULL;
}

/* Length of the valid UTF-8 sequence at s, or 0 (overlong, surrogate, > U+10FFFF, truncated). */
static size_t utf8_len(const unsigned char *s, size_t n)
{
	size_t len, i;

	if (s[0] < 0x80)
		return 1;
	if (s[0] >= 0xc2 && s[0] <= 0xdf)
		len = 2;
	else if (s[0] >= 0xe0 && s[0] <= 0xef)
		len = 3;
	else if (s[0] >= 0xf0 && s[0] <= 0xf4)
		len = 4;
	else
		return 0;
	if (n < len)
		return 0;
	for (i = 1; i < len; i++)
		if ((s[i] & 0xc0) != 0x80)
			return 0;
	if ((s[0] == 0xe0 && s[1] < 0xa0) || (s[0] == 0xed && s[1] > 0x9f) ||
	    (s[0] == 0xf0 && s[1] < 0x90) || (s[0] == 0xf4 && s[1] > 0x8f))
		return 0;
	return len;
}

/* JSON string literal. Invalid UTF-8 becomes U+FFFD so the document is always valid. */
static void put_string(FILE *f, const char *data, size_t n)
{
	const unsigned char *s = (const unsigned char *)data;
	size_t i = 0, len;

	putc_unlocked('"', f);
	while (i < n) {
		unsigned char c = s[i];

		if (c == '"' || c == '\\') {
			putc_unlocked('\\', f);
			putc_unlocked(c, f);
		} else if (c == '\n') {
			fputs("\\n", f);
		} else if (c == '\r') {
			fputs("\\r", f);
		} else if (c == '\t') {
			fputs("\\t", f);
		} else if (c < 0x20 || c == 0x7f) {
			fprintf(f, "\\u%04x", c);
		} else if (c < 0x80) {
			putc_unlocked(c, f);
		} else if ((len = utf8_len(&s[i], n - i))) {
			fwrite(&s[i], 1, len, f);
			i += len;
			continue;
		} else {
			fputs("\\ufffd", f);
		}
		i++;
	}
	putc_unlocked('"', f);
}

static void put_nullable_string(FILE *f, const char *s)
{
	if (s)
		put_string(f, s, strlen(s));
	else
		fputs("null", f);
}

int report_write(const struct result *res, const char *path)
{
	const char *err = sbx_error();
	FILE *f = stdout;

	if (path) {
		int fd = open(path, O_WRONLY | O_CREAT | O_TRUNC | O_NOFOLLOW | O_CLOEXEC, 0600);

		if (fd < 0)
			return sbx_fail("open %s: %m", path);
		f = fdopen(fd, "w");
		if (!f) {
			close(fd);
			return sbx_fail("fdopen %s: %m", path);
		}
	}
	setvbuf(f, NULL, _IOFBF, 1 << 16);

	fprintf(f, "{\"schema_version\":%d,\"termination_reason\":\"%s\",", SBX_REPORT_VERSION,
		term_reason_name(res->reason));
	if (res->exit_code >= 0)
		fprintf(f, "\"exit_code\":%d,", res->exit_code);
	else
		fputs("\"exit_code\":null,", f);
	if (res->term_signal > 0)
		fprintf(f, "\"signal\":%d,", res->term_signal);
	else
		fputs("\"signal\":null,", f);
	fprintf(f, "\"wall_ms\":%" PRIu64 ",\"cpu_ms\":%" PRIu64 ",\"memory_kb\":%" PRIu64 ",",
		res->wall_ms, res->cpu_ms, res->memory_kb);
	fputs("\"memory_source\":", f);
	put_nullable_string(f, mem_source_name(res->mem_source));
	fputs(",\"stdout\":", f);
	put_string(f, res->out ? res->out : "", res->out_len);
	fputs(",\"stderr\":", f);
	put_string(f, res->err ? res->err : "", res->err_len);
	fprintf(f, ",\"output_truncated\":%s,", res->output_truncated ? "true" : "false");
	if (res->has_violation)
		fprintf(f, "\"violation\":{\"syscall_nr\":%ld},", res->syscall_nr);
	else
		fputs("\"violation\":null,", f);
	fprintf(f, "\"pids_limit_hit\":%s,\"error\":", res->pids_limit_hit ? "true" : "false");
	put_nullable_string(f, res->reason == TR_INTERNAL_ERROR && err[0] ? err : NULL);
	fputs("}\n", f);

	if (fflush(f) || ferror(f)) {
		if (path)
			fclose(f);
		return sbx_fail("write report: %m");
	}
	if (path && fclose(f))
		return sbx_fail("close %s: %m", path);
	return 0;
}
