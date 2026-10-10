// SPDX-License-Identifier: MIT
/*
 * sandbox-exec: runs one command in an isolated, resource-limited
 * environment and reports how it ended. It knows nothing about test cases or
 * scores; the grading harness calls it once per test case.
 */
#include <errno.h>
#include <fcntl.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/syscall.h>
#include <unistd.h>

#include "sbx.h"

#ifndef SYS_close_range
#define SYS_close_range 436
#endif

/* Descriptors 0-2 must be open, or later open()s would land on them. */
static void fix_stdio(void)
{
	int fd;

	for (fd = 0; fd <= 2; fd++) {
		if (fcntl(fd, F_GETFD) < 0 && errno == EBADF && open("/dev/null", O_RDWR) != fd)
			_exit(SBX_EXIT_INTERNAL);
	}
}

/* stdin up to EOF. A terminal counts as empty input. */
static int read_input(unsigned char **out, size_t *len)
{
	unsigned char *buf = NULL, *tmp;
	size_t cap = 0, n = 0;
	ssize_t k;

	*out = NULL;
	*len = 0;
	if (isatty(STDIN_FILENO))
		return 0;
	for (;;) {
		if (n == cap) {
			cap = cap ? cap * 2 : 1 << 16;
			tmp = realloc(buf, cap);
			if (!tmp)
				goto fail;
			buf = tmp;
		}
		k = read(STDIN_FILENO, buf + n, cap - n);
		if (k < 0 && errno == EINTR)
			continue;
		if (k < 0)
			goto fail;
		if (!k)
			break;
		n += (size_t)k;
		if (n > SBX_MAX_INPUT) {
			errno = EFBIG;
			goto fail;
		}
	}
	*out = buf;
	*len = n;
	return 0;
fail:
	free(buf);
	return -1;
}

int main(int argc, char **argv)
{
	unsigned char *input = NULL;
	struct result res;
	struct opts o;
	size_t len = 0;
	int ret;

	fix_stdio();
	/* Nothing the caller left open above stderr may reach the sandboxed program. */
	syscall(SYS_close_range, 3U, ~0U, 0U);
	signal(SIGPIPE, SIG_IGN);

	if (opts_parse(argc, argv, &o)) {
		free(o.copies);
		return SBX_EXIT_USAGE;
	}
	if (o.sweep) {
		ret = run_sweep(&o);
		if (ret)
			sbx_log("%s", sbx_error());
		free(o.copies);
		return ret ? SBX_EXIT_INTERNAL : 0;
	}
	if (read_input(&input, &len)) {
		sbx_log("cannot read stdin (limit %u bytes): %m", SBX_MAX_INPUT);
		free(o.copies);
		return SBX_EXIT_USAGE;
	}

	supervisor_signals();
	ret = run_execute(&o, input, len, &res);
	if (ret)
		sbx_log("%s", sbx_error());
	if (report_write(&res, o.result_path)) {
		sbx_log("%s", sbx_error());
		ret = -1;
	}
	result_free(&res);
	free(input);
	free(o.copies);
	return ret ? SBX_EXIT_INTERNAL : 0;
}
