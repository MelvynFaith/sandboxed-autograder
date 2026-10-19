package main

import (
	"slices"
	"strings"
	"testing"
)

// env returns a getenv over vars, with the three API variables set unless vars overrides them.
func env(vars map[string]string) func(string) string {
	all := map[string]string{
		"DATABASE_URL": "postgres://u@localhost/db",
		"JWT_SECRET":   strings.Repeat("k", 32),
		"HTTP_ADDR":    ":8080",
	}
	for k, v := range vars {
		all[k] = v
	}
	return func(k string) string { return all[k] }
}

func TestLoadConfig(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{
		"CODE_DIR":        "/var/lib/autograder/submissions",
		"HARNESS_COMMAND": "/opt/grader/venv/bin/python  -m harness",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.workers != defaultWorkers || cfg.sandboxExec != defaultSandboxExec {
		t.Errorf("workers %d, sandbox-exec %q; want the defaults", cfg.workers, cfg.sandboxExec)
	}
	if want := []string{"/opt/grader/venv/bin/python", "-m", "harness"}; !slices.Equal(cfg.harnessCommand, want) {
		t.Errorf("harness command %q, want %q", cfg.harnessCommand, want)
	}
}

func TestLoadConfigAPIOnly(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{"WORKERS": "0"}))
	if err != nil {
		t.Fatalf("WORKERS=0 needs no grading settings: %v", err)
	}
	if cfg.workers != 0 {
		t.Errorf("workers %d, want 0", cfg.workers)
	}
}

func TestLoadConfigRejects(t *testing.T) {
	grading := map[string]string{"CODE_DIR": "/srv/code", "HARNESS_COMMAND": "/usr/bin/python3 -m harness"}
	with := func(k, v string) map[string]string {
		m := map[string]string{k: v}
		for gk, gv := range grading {
			if gk != k {
				m[gk] = gv
			}
		}
		return m
	}
	for name, vars := range map[string]map[string]string{
		"no database":              {"DATABASE_URL": ""},
		"no secret":                {"JWT_SECRET": ""},
		"no address":               {"HTTP_ADDR": ""},
		"workers not a number":     with("WORKERS", "four"),
		"negative workers":         with("WORKERS", "-1"),
		"no code directory":        with("CODE_DIR", ""),
		"relative code directory":  with("CODE_DIR", "submissions"),
		"no harness command":       with("HARNESS_COMMAND", "   "),
		"relative harness command": with("HARNESS_COMMAND", "python3 -m harness"),
		"relative sandbox-exec":    with("SANDBOX_EXEC", "sandbox-exec"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadConfig(env(vars)); err == nil {
				t.Fatal("loadConfig accepted it")
			}
		})
	}
}

func TestSeedAdminUsage(t *testing.T) {
	full := map[string]string{"DATABASE_URL": "postgres://u@localhost/db", adminPasswordEnv: "a long enough password"}
	without := func(k string) func(string) string {
		return func(key string) string {
			if key == k {
				return ""
			}
			return full[key]
		}
	}
	for name, c := range map[string]struct {
		args   []string
		getenv func(string) string
	}{
		"no flags":         {nil, without("")},
		"no email":         {[]string{"--name", "Admin"}, without("")},
		"stray argument":   {[]string{"--name", "Admin", "--email", "a@example.test", "extra"}, without("")},
		"password as flag": {[]string{"--name", "Admin", "--email", "a@example.test", "--password", "x"}, without("")},
		"no password":      {[]string{"--name", "Admin", "--email", "a@example.test"}, without(adminPasswordEnv)},
		"no database":      {[]string{"--name", "Admin", "--email", "a@example.test"}, without("DATABASE_URL")},
	} {
		t.Run(name, func(t *testing.T) {
			err := seedAdmin(c.args, c.getenv)
			if err == nil || !strings.Contains(err.Error(), "usage: autograder seed-admin") {
				t.Fatalf("got %v, want the usage message", err)
			}
		})
	}
}

func TestExpireCodeUsage(t *testing.T) {
	env := func(databaseURL, codeDir string) func(string) string {
		return func(key string) string {
			return map[string]string{"DATABASE_URL": databaseURL, "CODE_DIR": codeDir}[key]
		}
	}
	const db = "postgres://u@localhost/db"
	for name, c := range map[string]struct {
		args   []string
		getenv func(string) string
	}{
		"stray argument":    {[]string{"now"}, env(db, "/var/lib/autograder/submissions")},
		"unknown flag":      {[]string{"--days", "1"}, env(db, "/var/lib/autograder/submissions")},
		"no database":       {nil, env("", "/var/lib/autograder/submissions")},
		"no code directory": {nil, env(db, "")},
		"relative code dir": {[]string{"--dry-run"}, env(db, "submissions")},
	} {
		t.Run(name, func(t *testing.T) {
			err := expireCode(c.args, c.getenv)
			if err == nil || !strings.Contains(err.Error(), "usage: autograder expire-code") {
				t.Fatalf("got %v, want the usage message", err)
			}
		})
	}
}
