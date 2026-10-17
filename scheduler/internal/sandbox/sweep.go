// Package sandbox holds the scheduler's one direct use of sandbox-exec: cleaning up after dead runs.
// Student code is run by the Grading Harness, never from here (SRS 3.3).
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"time"
)

// sweepTimeout bounds one sweep. It removes a handful of empty cgroups and directories.
const sweepTimeout = time.Minute

// maxOutput is how much of the sweep's output goes into an error message.
const maxOutput = 4 << 10

// Sweeper runs `sandbox-exec --sweep`, which removes the empty sbx-* cgroups and the run directories
// that crashed runs left behind (SRS NFR-04). A run killed with the scheduler leaves them; the pid in
// their names lets sandbox-exec tell them from a live run's.
type Sweeper struct {
	// Binary is the path of sandbox-exec, for example /usr/local/bin/sandbox-exec.
	Binary string
	// CgroupRoot is the delegated cgroup the sandboxes are created under.
	CgroupRoot string
	// RunDir is the directory that holds the per-run work directories. Empty means sandbox-exec's own
	// default.
	RunDir string
}

// Sweep runs once and returns when sandbox-exec exits. A failure carries the end of its output.
func (s Sweeper) Sweep(ctx context.Context) error {
	if s.Binary == "" || s.CgroupRoot == "" {
		return errors.New("sandbox: sweep needs the sandbox-exec path and the cgroup root")
	}
	ctx, cancel := context.WithTimeout(ctx, sweepTimeout)
	defer cancel()

	args := []string{"--sweep", "--cgroup-root", s.CgroupRoot}
	if s.RunDir != "" {
		args = append(args, "--run-dir", s.RunDir)
	}
	cmd := exec.CommandContext(ctx, s.Binary, args...)
	cmd.Env = []string{} // nothing from the scheduler's environment
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > maxOutput {
			out = slices.Clone(out[len(out)-maxOutput:])
		}
		return fmt.Errorf("sandbox: sweep: %w: %s", err, out)
	}
	return nil
}
