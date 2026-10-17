package sandbox_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/sandbox"
)

// fakeSandboxExec writes a shell script that stands for sandbox-exec: it records its arguments in a
// file, refuses to run if the scheduler's environment reached it, and then exits with status exit
// after printing out on standard error. It returns the script path and the record path.
func fakeSandboxExec(t *testing.T, exit int, out string) (script, record string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}
	dir := t.TempDir()
	record = filepath.Join(dir, "args")
	script = filepath.Join(dir, "sandbox-exec")
	body := fmt.Sprintf(`#!/bin/sh
if env | grep -q SCHEDULER_SECRET; then echo "environment leaked" >&2; exit 3; fi
echo "$@" > %q
printf '%%s\n' %q >&2
exit %d
`, record, out, exit)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script, record
}

func TestSweepRunsSandboxExecWithTheRightArguments(t *testing.T) {
	t.Setenv("SCHEDULER_SECRET", "must-not-reach-sandbox-exec")
	script, record := fakeSandboxExec(t, 0, "")
	s := sandbox.Sweeper{Binary: script, CgroupRoot: "/sys/fs/cgroup/autograder", RunDir: "/var/lib/autograder/runs"}
	if err := s.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	want := "--sweep --cgroup-root /sys/fs/cgroup/autograder --run-dir /var/lib/autograder/runs"
	if got := strings.TrimSpace(string(raw)); got != want {
		t.Fatalf("arguments = %q, want %q", got, want)
	}
}

func TestSweepOmitsRunDirWhenNotSet(t *testing.T) {
	script, record := fakeSandboxExec(t, 0, "")
	if err := (sandbox.Sweeper{Binary: script, CgroupRoot: "/cg"}).Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	raw, _ := os.ReadFile(record)
	if got := strings.TrimSpace(string(raw)); got != "--sweep --cgroup-root /cg" {
		t.Fatalf("arguments = %q", got)
	}
}

func TestSweepFailureCarriesTheOutput(t *testing.T) {
	script, _ := fakeSandboxExec(t, 70, "sweep: cgroup root is not writable")
	err := sandbox.Sweeper{Binary: script, CgroupRoot: "/cg"}.Sweep(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cgroup root is not writable") {
		t.Fatalf("err = %v, want it to carry sandbox-exec's output", err)
	}
}

func TestSweepNeedsBinaryAndRoot(t *testing.T) {
	if err := (sandbox.Sweeper{CgroupRoot: "x"}).Sweep(context.Background()); err == nil {
		t.Error("Sweep without a binary succeeded")
	}
	if err := (sandbox.Sweeper{Binary: "x"}).Sweep(context.Background()); err == nil {
		t.Error("Sweep without a cgroup root succeeded")
	}
}

func TestSweepWithAMissingBinaryFails(t *testing.T) {
	if err := (sandbox.Sweeper{Binary: "/nonexistent/sandbox-exec", CgroupRoot: "/cg"}).Sweep(context.Background()); err == nil {
		t.Fatal("Sweep succeeded with no binary")
	}
}
