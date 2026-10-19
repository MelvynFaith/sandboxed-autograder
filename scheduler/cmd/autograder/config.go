package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// defaultWorkers is the worker count of the SRS performance model (Bagian 5.1, NFR-01a: W = 4).
	defaultWorkers = 4

	// defaultSandboxExec is where infra/README.md installs sandbox-exec. The AppArmor profile that
	// lets it create user namespaces on Ubuntu 24.04 attaches to this path only.
	defaultSandboxExec = "/usr/local/bin/sandbox-exec"
)

// config is the process configuration, read from the environment (infra/README.md lists it).
type config struct {
	databaseURL string
	jwtSecret   string
	httpAddr    string

	// workers is the number of submissions graded at once; 0 runs the API alone.
	workers int
	// codeDir holds the stored submission files; a submission's code_path is relative to it.
	codeDir string
	// harnessCommand is the Harness program and the arguments before --definition.
	harnessCommand []string
	// sandboxExec is the path of sandbox-exec, for the Harness and for the start-up sweep.
	sandboxExec string
}

// loadConfig reads the configuration through getenv, which is os.Getenv outside tests.
func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		databaseURL: getenv("DATABASE_URL"),
		jwtSecret:   getenv("JWT_SECRET"),
		httpAddr:    getenv("HTTP_ADDR"),
		workers:     defaultWorkers,
		codeDir:     getenv("CODE_DIR"),
		// Split on spaces, so no element can contain one.
		harnessCommand: strings.Fields(getenv("HARNESS_COMMAND")),
		sandboxExec:    getenv("SANDBOX_EXEC"),
	}
	if cfg.databaseURL == "" || cfg.jwtSecret == "" || cfg.httpAddr == "" {
		return config{}, errors.New("DATABASE_URL, JWT_SECRET, and HTTP_ADDR environment variables are required")
	}
	if s := getenv("WORKERS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return config{}, fmt.Errorf("WORKERS is %q, want a whole number, 0 to grade nothing", s)
		}
		cfg.workers = n
	}
	if cfg.codeDir == "" {
		return config{}, errors.New("CODE_DIR is required to store uploaded submissions")
	}
	if !filepath.IsAbs(cfg.codeDir) {
		return config{}, fmt.Errorf("CODE_DIR %q is not an absolute path", cfg.codeDir)
	}
	if cfg.sandboxExec == "" {
		cfg.sandboxExec = defaultSandboxExec
	}
	if cfg.workers == 0 {
		return cfg, nil
	}

	// The paths are handed to other programs and resolved against their working directories, so
	// relative ones would point somewhere else.
	switch {
	case len(cfg.harnessCommand) == 0:
		return config{}, errors.New("HARNESS_COMMAND is required unless WORKERS=0")
	case !filepath.IsAbs(cfg.harnessCommand[0]):
		return config{}, fmt.Errorf("HARNESS_COMMAND starts with %q, not an absolute path", cfg.harnessCommand[0])
	case !filepath.IsAbs(cfg.sandboxExec):
		return config{}, fmt.Errorf("SANDBOX_EXEC %q is not an absolute path", cfg.sandboxExec)
	}
	return cfg, nil
}
