package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"
)

// ErrHarness means the Harness could not be run, or ran and failed: it did not start, exited with a
// non-zero status, or wrote an unusable standard output. The message ends with the tail of its
// standard error. A deadline or cancellation is reported as the context's own error instead.
var ErrHarness = errors.New("harness: run failed")

const (
	// DefaultMaxReportBytes bounds the report. One result carries at most max_output_bytes of program
	// output (1 MiB in the SRS example), so this allows dozens of full-size results.
	DefaultMaxReportBytes = 64 << 20

	// stderrTail is how much of the Harness's standard error is kept for the error message.
	stderrTail = 4 << 10

	// killGrace is how long to wait, after killing the Harness on a deadline, for its pipes to close.
	killGrace = 5 * time.Second
)

// Runner starts the Harness. The zero value is not usable: Command is required. A Runner may be shared
// by concurrent workers.
type Runner struct {
	// Command is the Harness program followed by the arguments that precede --definition, for example
	// {"/opt/grader/venv/bin/python", "-m", "grader"}.
	Command []string

	// Env is the whole environment of the Harness, as "KEY=value" strings. It is never inherited from
	// the scheduler. An empty Env gives an empty environment.
	Env []string

	// TempDir is the parent of the private directory that holds the definition file. Empty means
	// os.TempDir. The private directory has mode 0700 wherever it is, but a parent that only the
	// service user can list also hides that a grading run is in progress.
	TempDir string

	// MaxReportBytes bounds the Harness's standard output. Zero means DefaultMaxReportBytes.
	MaxReportBytes int64
}

// Run grades one submission. It writes def to a private file, starts the Harness with that file's path
// and codePath, waits for it, and returns the checked report. The file is removed on every path out.
//
// If ctx ends first, the Harness and the processes it started are killed and Run returns an error that
// wraps ctx.Err(). Errors that wrap ErrHarness or ErrReport mean the pipeline failed for this
// submission; other errors mean the worker could not prepare the run.
func (r *Runner) Run(ctx context.Context, def *Definition, codePath string) (*Report, error) {
	if len(r.Command) == 0 {
		return nil, errors.New("harness: no command configured")
	}
	limit := r.MaxReportBytes
	if limit <= 0 {
		limit = DefaultMaxReportBytes
	}

	// MkdirTemp creates the directory with mode 0700, so the file inside cannot be reached by others
	// even before its own mode applies.
	dir, err := os.MkdirTemp(r.TempDir, "harness-")
	if err != nil {
		return nil, fmt.Errorf("harness: %w", err)
	}
	defer os.RemoveAll(dir)

	defPath := filepath.Join(dir, "definition.json")
	if err := writePrivate(defPath, def); err != nil {
		return nil, fmt.Errorf("harness: write definition: %w", err)
	}

	args := append(slices.Clone(r.Command[1:]), "--definition", defPath, "--code", codePath)
	cmd := exec.CommandContext(ctx, r.Command[0], args...)
	cmd.Dir = dir
	cmd.Env = append([]string{}, r.Env...) // non-nil: an empty environment, not the scheduler's
	cmd.WaitDelay = killGrace
	configureKill(cmd)

	stdout := &boundedWriter{limit: limit}
	stderr := &tailWriter{keep: stderrTail}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		switch {
		case ctx.Err() != nil:
			return nil, fmt.Errorf("harness: %w", ctx.Err())
		case stdout.over:
			return nil, fmt.Errorf("%w: report larger than %d bytes", ErrReport, limit)
		}
		return nil, fmt.Errorf("%w: %v; stderr: %s", ErrHarness, err, stderr.String())
	}

	report, err := Parse(stdout.buf)
	if err != nil {
		return nil, err
	}
	if err := report.Check(def); err != nil {
		return nil, err
	}
	return report, nil
}

// writePrivate creates path with mode 0600, failing if it exists, and writes v as JSON.
func writePrivate(path string, v any) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	return json.NewEncoder(f).Encode(v)
}

// boundedWriter collects up to limit bytes and fails the write that would exceed it, which makes
// exec stop reading and the Harness see a closed pipe.
type boundedWriter struct {
	buf   []byte
	limit int64
	over  bool
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if int64(len(w.buf))+int64(len(p)) > w.limit {
		w.over = true
		return 0, errors.New("report too large")
	}
	w.buf = append(w.buf, p...)
	return len(p), nil
}

// tailWriter keeps the last keep bytes written to it and never fails.
type tailWriter struct {
	buf  []byte
	keep int
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	if len(w.buf) > w.keep {
		w.buf = slices.Delete(w.buf, 0, len(w.buf)-w.keep)
	}
	return len(p), nil
}

func (w *tailWriter) String() string { return string(w.buf) }
