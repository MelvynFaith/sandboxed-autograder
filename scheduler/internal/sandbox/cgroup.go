package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// controllers are the ones sandbox-exec writes limits for: memory.max and memory.swap.max, pids.max
	// and cpu.max (SRS 3.2.1.3).
	controllers = "+memory +pids +cpu"

	// leaf is the child cgroup the scheduler moves itself into, so that its own cgroup holds no process
	// and can enable controllers for the sandboxes (cgroup v2 "no internal process" rule).
	leaf = "worker"
)

// Delegate prepares the cgroup this process was started in for sandbox-exec and returns its path, to
// be passed as --cgroup-root. The cgroup must be delegated to the service user, which the systemd unit
// does with Delegate=yes (SRS 3.2.1.1, risk R11).
//
// It creates the leaf cgroup <root>/worker, moves this process into it, and enables the memory, pids
// and cpu controllers on <root>. Everything the process starts afterwards, the Harness and through it
// sandbox-exec, also runs in the leaf, below the delegated root that sandbox-exec moves its child under.
// Call it once, before any goroutine starts a process.
func Delegate() (string, error) {
	return delegate("/proc/self/cgroup", "/sys/fs/cgroup", os.Getpid())
}

// delegate is Delegate with the files it reads and the pid it moves as parameters, for tests.
func delegate(selfCgroup, mount string, pid int) (string, error) {
	b, err := os.ReadFile(selfCgroup)
	if err != nil {
		return "", fmt.Errorf("sandbox: find own cgroup: %w", err)
	}
	// On the unified hierarchy the file has one line, "0::/path"; a hybrid host adds v1 lines.
	var rel string
	found := false
	for line := range strings.Lines(string(b)) {
		if p, ok := strings.CutPrefix(strings.TrimRight(line, "\n"), "0::"); ok {
			rel, found = p, true
			break
		}
	}
	if !found || !strings.HasPrefix(rel, "/") {
		return "", errors.New("sandbox: this process is not in a cgroup v2 unified hierarchy")
	}
	root := filepath.Join(mount, rel)

	hint := "; is the service started with Delegate=yes (infra/systemd/autograder.service)?"
	if err := os.Mkdir(filepath.Join(root, leaf), 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", fmt.Errorf("sandbox: create leaf cgroup: %w%s", err, hint)
	}
	if err := write(filepath.Join(root, leaf, "cgroup.procs"), strconv.Itoa(pid)); err != nil {
		return "", fmt.Errorf("sandbox: move into leaf cgroup: %w%s", err, hint)
	}
	// EBUSY here means another process is still in root, for example when the binary was started from
	// a shell's own cgroup rather than a unit or scope of its own.
	if err := write(filepath.Join(root, "cgroup.subtree_control"), controllers); err != nil {
		return "", fmt.Errorf("sandbox: enable %s in %s: %w%s", controllers, root, err, hint)
	}
	return root, nil
}

// write writes s to an existing file: cgroup interface files are created by the kernel, never by us.
func write(path, s string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString(s)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
