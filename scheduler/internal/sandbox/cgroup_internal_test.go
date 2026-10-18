package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCgroup builds a directory that looks like the delegated cgroup of a unit: it has the interface
// files the kernel would create, including those of the leaf, which a real mkdir would create too. It
// returns the /proc/self/cgroup stand-in, the mount point and the unit cgroup's directory.
func fakeCgroup(t *testing.T, procSelf string) (selfCgroup, mount, unit string) {
	t.Helper()
	dir := t.TempDir()
	mount = filepath.Join(dir, "cgroup")
	unit = filepath.Join(mount, "system.slice", "autograder.service")
	for _, f := range []string{
		filepath.Join(unit, "cgroup.subtree_control"),
		filepath.Join(unit, leaf, "cgroup.procs"),
	} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	selfCgroup = filepath.Join(dir, "self-cgroup")
	if err := os.WriteFile(selfCgroup, []byte(procSelf), 0o644); err != nil {
		t.Fatal(err)
	}
	return selfCgroup, mount, unit
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDelegate(t *testing.T) {
	for name, procSelf := range map[string]string{
		"unified": "0::/system.slice/autograder.service\n",
		"hybrid":  "12:pids:/system.slice/autograder.service\n0::/system.slice/autograder.service\n",
	} {
		t.Run(name, func(t *testing.T) {
			self, mount, unit := fakeCgroup(t, procSelf)
			root, err := delegate(self, mount, 4242)
			if err != nil {
				t.Fatal(err)
			}
			if root != unit {
				t.Errorf("root %q, want %q", root, unit)
			}
			if got := read(t, filepath.Join(unit, leaf, "cgroup.procs")); got != "4242" {
				t.Errorf("leaf cgroup.procs %q, want the pid", got)
			}
			if got := read(t, filepath.Join(unit, "cgroup.subtree_control")); got != controllers {
				t.Errorf("subtree_control %q, want %q", got, controllers)
			}
		})
	}
}

func TestDelegateRefuses(t *testing.T) {
	for name, procSelf := range map[string]string{
		"cgroup v1 only": "12:pids:/user.slice\n1:name=systemd:/user.slice\n",
		"empty":          "",
		"relative path":  "0::system.slice\n",
	} {
		t.Run(name, func(t *testing.T) {
			self, mount, _ := fakeCgroup(t, procSelf)
			if root, err := delegate(self, mount, 1); err == nil {
				t.Fatalf("delegate succeeded with root %q", root)
			}
		})
	}
}

// A cgroup that cannot be written, here because the unit was not delegated and the files are missing,
// fails with the hint that names the unit setting.
func TestDelegateNotDelegated(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "self-cgroup")
	if err := os.WriteFile(self, []byte("0::/missing.service\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := delegate(self, dir, 1)
	if err == nil || !strings.Contains(err.Error(), "Delegate=yes") {
		t.Fatalf("got %v, want an error that mentions Delegate=yes", err)
	}
}
