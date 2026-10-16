# SPDX-License-Identifier: MIT
#
# sandbox-exec. CI runs plain `make`; it must build with -Wall -Wextra -Werror.
#
#   make          build ./sandbox-exec
#   make test     adversarial suite (Linux, cgroup v2, user namespaces)
#   make asan     rebuild with AddressSanitizer + UBSan and run the suite
#   make tidy     clang-tidy
#   make install  binary and man page under $(DESTDIR)$(PREFIX)
#   make clean
#
# Overrides: CC, BIN, BUILD_DIR, PREFIX, DESTDIR, SECCOMP_CFLAGS, SECCOMP_LIBS.

CC ?= cc
BIN ?= sandbox-exec
BUILD_DIR ?= build
PREFIX ?= /usr/local

SECCOMP_CFLAGS ?= $(shell pkg-config --cflags libseccomp 2>/dev/null)
SECCOMP_LIBS ?= $(shell pkg-config --libs libseccomp 2>/dev/null || echo -lseccomp)

WARNINGS := -Wall -Wextra -Werror -Wshadow -Wformat=2 -Wstrict-prototypes -Wmissing-prototypes \
	    -Wcast-qual -Wwrite-strings -Wundef -Wold-style-definition
HARDENING := -U_FORTIFY_SOURCE -D_FORTIFY_SOURCE=2 -fstack-protector-strong -fPIE
override CFLAGS += -std=gnu11 -D_GNU_SOURCE -O2 -g $(WARNINGS) $(HARDENING) $(SECCOMP_CFLAGS)
override LDFLAGS += -pie -Wl,-z,relro,-z,now,--as-needed
LDLIBS += $(SECCOMP_LIBS)

ifdef SANITIZE
override CFLAGS += -fsanitize=address,undefined -fno-omit-frame-pointer -fno-sanitize-recover=undefined
override LDFLAGS += -fsanitize=address,undefined
endif

SRCS := $(wildcard src/*.c)
OBJS := $(SRCS:src/%.c=$(BUILD_DIR)/%.o)

.PHONY: all test asan tidy install clean
all: $(BIN)

$(BIN): $(OBJS)
	$(CC) $(LDFLAGS) -o $@ $(OBJS) $(LDLIBS)

$(BUILD_DIR)/%.o: src/%.c src/sbx.h | $(BUILD_DIR)
	$(CC) $(CFLAGS) -c -o $@ $<

$(BUILD_DIR):
	mkdir -p $@

test: $(BIN)
	bash tests/run.sh $(abspath $(BIN))

asan:
	$(MAKE) SANITIZE=1 BUILD_DIR=$(BUILD_DIR)/asan BIN=$(BIN)-asan
	bash tests/run.sh $(abspath $(BIN)-asan)

tidy:
	clang-tidy $(SRCS) -- -std=gnu11 -D_GNU_SOURCE $(SECCOMP_CFLAGS)

install: $(BIN)
	install -D -m 0755 $(BIN) $(DESTDIR)$(PREFIX)/bin/sandbox-exec
	install -D -m 0644 sandbox-exec.1 $(DESTDIR)$(PREFIX)/share/man/man1/sandbox-exec.1

clean:
	rm -rf $(BUILD_DIR) $(BIN) $(BIN)-asan
