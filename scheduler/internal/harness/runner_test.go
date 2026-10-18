package harness_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
)

// The Harness under test is this test binary, started again with HARNESS_HELPER set to what it should
// do. That avoids depending on Python or a shell, and lets the "Harness" assert on how it was started.
const helperEnv = "HARNESS_HELPER"

// secret stands for a hidden test case's expected output. It must never reach a command line.
const secret = "HIDDEN-EXPECTED-OUTPUT"

func TestHelperProcess(t *testing.T) {
	mode := os.Getenv(helperEnv)
	if mode == "" {
		t.Skip("only runs as the fake Harness")
	}
	helper(mode)
	os.Exit(0)
}

func helperFail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(3)
}

// helper is the fake Harness. Failures it detects exit with status 3 and a message on stderr, which
// the test sees in the error from Run.
func helper(mode string) {
	args := map[string]string{}
	for i, a := range os.Args {
		if (a == "--definition" || a == "--code") && i+1 < len(os.Args) {
			args[a] = os.Args[i+1]
		}
	}

	switch mode {
	case "fail":
		fmt.Fprint(os.Stderr, "boom: Traceback (most recent call last)")
		os.Exit(2)
	case "flood":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 100<<10)+"THE-END")
		os.Exit(1)
	case "garbage":
		fmt.Print("not json")
		return
	case "huge":
		fmt.Print(strings.Repeat("x", 1<<20))
		return
	case "hang":
		time.Sleep(time.Hour)
	case "hang-with-child":
		// Start a grandchild like sandbox-exec would be, tell the test its pid, and hang.
		child := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
		child.Env = []string{helperEnv + "=hang"}
		if err := child.Start(); err != nil {
			helperFail("start child: %v", err)
		}
		if err := os.WriteFile(os.Getenv("CHILD_PID_FILE"), fmt.Appendf(nil, "%d", child.Process.Pid), 0o600); err != nil {
			helperFail("write pid: %v", err)
		}
		time.Sleep(time.Hour)
	}

	// The remaining modes read the definition the way the real Harness does, after checking how
	// they were started.
	for _, a := range os.Args {
		if strings.Contains(a, secret) {
			helperFail("hidden data on the command line: %q", a)
		}
	}
	if _, ok := os.LookupEnv("SCHEDULER_SECRET"); ok {
		helperFail("the scheduler's environment leaked into the Harness")
	}
	path := args["--definition"]
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		helperFail("definition file %q: %v, want mode 0600", path, err)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil || info.Mode().Perm() != 0o700 {
		helperFail("definition directory: %v, want mode 0700", err)
	}
	if args["--code"] != "/work/main.py" {
		helperFail("--code = %q", args["--code"])
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		helperFail("read definition: %v", err)
	}
	var def harness.Definition
	if err := json.Unmarshal(raw, &def); err != nil {
		helperFail("decode definition: %v", err)
	}

	rep := harness.Report{SubmissionID: def.SubmissionID}
	for _, tc := range def.TestCases {
		rep.MaxScore += tc.Weight
		rep.TotalScore += tc.Weight
		rep.Results = append(rep.Results, harness.Result{
			TestCaseID: tc.ID, ExternalID: tc.ExternalID, Status: harness.StatusPassed, Score: tc.Weight,
			ActualOutput: tc.Input, ExecTimeMS: 10, MemoryKB: 2048, MemorySource: harness.MemoryCgroupPeak,
		})
	}
	switch mode {
	case "ok":
	case "progress", "progress-flood":
		fd, err := strconv.Atoi(os.Getenv(harness.EnvProgressFD))
		if err != nil {
			helperFail("%s: %v", harness.EnvProgressFD, err)
		}
		f := os.NewFile(uintptr(fd), "progress")
		lines := []string{
			`{"current_test": 1}`,
			`not json`,
			`{"current_test": 1}`, // does not count up
			`{"current_test": 3}`, // more than there are
			`{"current_test": ` + strings.Repeat("9", 300) + `}`, // longer than a line may be
			`{"current_test": 2}`,
		}
		if mode == "progress-flood" {
			// Far more than a pipe holds; the Harness must not block on it.
			lines = append(lines, strings.Repeat(`{"current_test": 0}`+"\n", 1<<16))
		}
		for _, l := range lines {
			if _, err := f.WriteString(l + "\n"); err != nil {
				helperFail("write progress: %v", err)
			}
		}
		f.Close()
	case "wrong-total":
		rep.TotalScore++
	case "nul":
		rep.Results[0].ActualOutput = "a\x00b"
	default:
		helperFail("unknown mode %q", mode)
	}
	if err := json.NewEncoder(os.Stdout).Encode(rep); err != nil {
		helperFail("encode report: %v", err)
	}
}

func definition() *harness.Definition {
	return &harness.Definition{
		SchemaVersion: harness.DefinitionVersion,
		SubmissionID:  128,
		ResourceLimit: harness.ResourceLimit{
			CPUTimeMS: 2000, MemoryMB: 64, MaxProcesses: 5, TimeoutS: 5, MaxOutputBytes: 1 << 20, TmpfsSizeMB: 32,
		},
		TestCases: []harness.TestCase{
			{ID: 501, ExternalID: "tc_01", Input: "5\n", ExpectedOutput: secret + "\n", Weight: 20, MatchMode: harness.MatchExact},
			{ID: 502, ExternalID: "tc_02", Input: "0\n", ExpectedOutput: "1\n", Weight: 30, MatchMode: harness.MatchWhitespaceInsensitive},
		},
	}
}

// runner returns a Runner that starts the fake Harness in the given mode, with a private temp
// directory the test can inspect afterwards.
func runner(t *testing.T, mode string, extraEnv ...string) (*harness.Runner, string) {
	t.Helper()
	t.Setenv("SCHEDULER_SECRET", "must-not-reach-the-harness")
	tmp := t.TempDir()
	return &harness.Runner{
		Command: []string{os.Args[0], "-test.run=^TestHelperProcess$", "--"},
		Env:     append([]string{helperEnv + "=" + mode}, extraEnv...),
		TempDir: tmp,
	}, tmp
}

func assertEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("%s still holds %d entries; the definition file was not removed", dir, len(entries))
	}
}

func TestRunReturnsTheCheckedReport(t *testing.T) {
	r, tmp := runner(t, "ok")
	rep, err := r.Run(context.Background(), definition(), "/work/main.py", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.SubmissionID != 128 || rep.TotalScore != 50 || rep.MaxScore != 50 || len(rep.Results) != 2 {
		t.Fatalf("report = %+v", rep)
	}
	if rep.Results[0].ActualOutput != "5\n" {
		t.Errorf("the Harness did not receive the test input: %q", rep.Results[0].ActualOutput)
	}
	assertEmpty(t, tmp)
}

func TestRunReportsProgress(t *testing.T) {
	for _, mode := range []string{"progress", "progress-flood"} {
		t.Run(mode, func(t *testing.T) {
			r, _ := runner(t, mode)
			var mu sync.Mutex
			var got []int
			rep, err := r.Run(context.Background(), definition(), "/work/main.py", func(n int) {
				mu.Lock()
				got = append(got, n)
				mu.Unlock()
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if rep.TotalScore != 50 {
				t.Fatalf("report = %+v", rep)
			}
			// Run has returned, so no call can still be in flight.
			if !slices.Equal(got, []int{1, 2}) {
				t.Fatalf("progress %v, want [1 2]: the valid, increasing lines only", got)
			}
		})
	}
}

func TestRunStripsNULFromOutput(t *testing.T) {
	r, _ := runner(t, "nul")
	rep, err := r.Run(context.Background(), definition(), "/work/main.py", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := rep.Results[0].ActualOutput; got != "a�b" {
		t.Fatalf("output = %q, want the NUL replaced", got)
	}
}

func TestRunFailures(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		max     int64
		wantErr error
		wantMsg string
	}{
		{"non-zero exit", "fail", 0, harness.ErrHarness, "Traceback"},
		{"totals that do not add up", "wrong-total", 0, harness.ErrReport, "totals"},
		{"not JSON", "garbage", 0, harness.ErrReport, "invalid"},
		{"report over the limit", "huge", 1 << 10, harness.ErrReport, "larger than"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, tmp := runner(t, tc.mode)
			r.MaxReportBytes = tc.max
			_, err := r.Run(context.Background(), definition(), "/work/main.py", nil)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("err = %q, want it to mention %q", err, tc.wantMsg)
			}
			assertEmpty(t, tmp)
		})
	}
}

func TestRunKeepsOnlyTheTailOfStderr(t *testing.T) {
	r, _ := runner(t, "flood")
	_, err := r.Run(context.Background(), definition(), "/work/main.py", nil)
	if !errors.Is(err, harness.ErrHarness) {
		t.Fatalf("err = %v, want ErrHarness", err)
	}
	// The helper writes 100 KiB, then the Go runtime may add a line of its own on exit under -cover,
	// so the marker is near the end rather than at it.
	if msg := err.Error(); !strings.Contains(msg, "THE-END") || len(msg) > 5<<10 {
		t.Fatalf("error is %d bytes; want the tail of stderr, with THE-END, under 5 KiB", len(msg))
	}
}

func TestRunWithoutCommand(t *testing.T) {
	if _, err := (&harness.Runner{}).Run(context.Background(), definition(), "x", nil); err == nil {
		t.Fatal("Run without a command succeeded")
	}
}

func TestRunKillsAHangingHarnessAtTheDeadline(t *testing.T) {
	r, tmp := runner(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := r.Run(ctx, definition(), "/work/main.py", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Run took %v after a 300ms deadline", elapsed)
	}
	assertEmpty(t, tmp)
}
