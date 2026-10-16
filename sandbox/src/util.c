// SPDX-License-Identifier: MIT
/*
 * Logging, error recording and small file helpers.
 */
#include <errno.h>
#include <fcntl.h>
#include <signal.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

#include "sbx.h"

static char errbuf[256];

void sbx_log(const char *fmt, ...)
{
	int saved = errno;
	va_list ap;

	fputs("sandbox-exec: ", stderr);
	errno = saved;	/* for %m */
	va_start(ap, fmt);
	vfprintf(stderr, fmt, ap);
	va_end(ap);
	fputc('\n', stderr);
}

int sbx_fail(const char *fmt, ...)
{
	va_list ap;

	if (errbuf[0])
		return -1;
	va_start(ap, fmt);
	vsnprintf(errbuf, sizeof(errbuf), fmt, ap);
	va_end(ap);
	return -1;
}

const char *sbx_error(void)
{
	return errbuf;
}

uint64_t now_ns(void)
{
	struct timespec ts;

	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (uint64_t)ts.tv_sec * 1000000000ULL + (uint64_t)ts.tv_nsec;
}

int pathf(char *dst, size_t size, const char *fmt, ...)
{
	va_list ap;
	int n;

	va_start(ap, fmt);
	n = vsnprintf(dst, size, fmt, ap);
	va_end(ap);
	if (n < 0 || (size_t)n >= size)
		return sbx_fail("path too long: %.64s...", dst);
	return 0;
}

int write_all(int fd, const void *buf, size_t len)
{
	const char *p = buf;

	while (len) {
		ssize_t n = write(fd, p, len);

		if (n < 0) {
			if (errno == EINTR)
				continue;
			return -1;
		}
		p += n;
		len -= (size_t)n;
	}
	return 0;
}

int write_file(const char *path, const char *str)
{
	size_t len = strlen(str);
	int fd, err;
	ssize_t n;

	fd = open(path, O_WRONLY | O_CLOEXEC);
	if (fd < 0)
		return -1;
	n = write(fd, str, len);
	err = n < 0 ? errno : EIO;
	close(fd);
	if (n != (ssize_t)len) {
		errno = err;
		return -1;
	}
	return 0;
}

ssize_t pread_str(int fd, char *buf, size_t size)
{
	ssize_t n = pread(fd, buf, size - 1, 0);

	if (n >= 0)
		buf[n] = '\0';
	return n;
}

bool kv_get(const char *text, const char *key, int64_t *val)
{
	size_t klen = strlen(key);
	const char *p = text;

	while (*p) {
		if (!strncmp(p, key, klen) && p[klen] == ' ') {
			*val = strtoll(p + klen + 1, NULL, 10);
			return true;
		}
		p = strchr(p, '\n');
		if (!p)
			break;
		p++;
	}
	return false;
}

void close_fd(int *fd)
{
	if (*fd >= 0) {
		close(*fd);
		*fd = -1;
	}
}

bool owner_alive(const char *name)
{
	char *end;
	long pid;

	if (strncmp(name, "sbx-", 4))
		return false;
	pid = strtol(name + 4, &end, 10);
	if (end == name + 4 || *end != '-' || pid <= 0)
		return false;
	return !kill((pid_t)pid, 0) || errno == EPERM;
}
