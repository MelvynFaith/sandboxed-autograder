//go:build !unix

package harness

import "os/exec"

// configureKill keeps the default behaviour, which kills only the Harness itself. The scheduler runs
// on Linux; this exists so the package still builds on a developer's other machine.
func configureKill(*exec.Cmd) {}
