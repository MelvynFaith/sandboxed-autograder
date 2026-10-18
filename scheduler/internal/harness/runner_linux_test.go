package harness_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// alive reports whether pid is a running process. A zombie is dead: it only awaits its parent.
func alive(pid int) bool {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	s := string(stat)
	return s[strings.LastIndexByte(s, ')')+2] != 'Z'
}

// sandbox-exec processes are started by the Harness, so killing only the Harness would leave them
// running past the deadline.
func TestRunKillsTheProcessesTheHarnessStarted(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	r, _ := runner(t, "hang-with-child", "CHILD_PID_FILE="+pidFile)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(30*time.Second, cancel) // a guard against a test that never starts its child
	go func() {
		// Cancel once the Harness has started its child and recorded the pid.
		for ctx.Err() == nil {
			if raw, err := os.ReadFile(pidFile); err == nil && len(raw) > 0 {
				cancel()
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	if _, err := r.Run(ctx, definition(), "/work/main.py", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("the Harness never started its child: %v", err)
	}
	pid, err := strconv.Atoi(string(raw))
	if err != nil {
		t.Fatalf("pid file holds %q", raw)
	}
	for deadline := time.Now().Add(5 * time.Second); alive(pid); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("process %d, started by the Harness, survived the kill", pid)
		}
	}
}
