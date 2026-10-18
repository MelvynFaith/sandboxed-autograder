// Package harness runs the Grading Harness for one submission and checks what it returns.
//
// The worker calls Runner.Run once per submission (SRS 3.3, steps 4 to 8). The Harness is a separate
// program written in Python, so this package is the whole interface between the two: what the worker
// hands over, how it starts the process, and what it accepts back.
//
// # Contract with the Harness
//
// The worker starts the program named by Runner.Command with two more arguments:
//
//	<command...> --definition PATH --code PATH
//
// --definition names a JSON file with the assignment definition (type Definition): the test cases
// with their hidden expected output, the resource limits and the comparison modes. The file is created
// in a private directory (mode 0700) with mode 0600, is never passed on the command line, and is
// deleted when the Harness exits, so the expected output of a hidden test case cannot be read from
// /proc/<pid>/cmdline or by another user (SRS 3.2.3). --code names the student's source file.
//
// The Harness environment is exactly Runner.Env. Nothing is inherited from the scheduler, whose own
// environment holds the database password and the token key. The scheduler passes the Harness what it
// needs to find the sandbox: the path of sandbox-exec in EnvSandboxExec and the delegated cgroup to
// give sandbox-exec as --cgroup-root in EnvCgroupRoot.
//
// The Harness may report progress for the dashboard (FR-08). EnvProgressFD names an open file
// descriptor, a pipe; before it starts each test case the Harness writes one line to it,
// {"current_test": n}, where n counts from 1 in the order of Definition.TestCases. Lines that are not
// of that form, or that do not count up, are ignored, and a Harness that writes nothing still works:
// progress is for display only. The descriptor must not be passed on to sandbox-exec (Python's
// subprocess closes it by default).
//
// The Harness writes one JSON document to standard output (type Report, SRS Appendix C) and exits 0.
// Standard error is free text for the log; only its last few kilobytes are kept. Any other exit status,
// no output, output that is not valid, or output that contradicts the definition makes the submission
// FAILED rather than COMPLETED (SRS 3.3, step 8).
//
// The Harness never opens a database connection, and it is the worker, not the Harness, that writes
// Result rows.
package harness

// Environment variables the scheduler sets for the Harness (see the contract above). The Harness reads
// the same names (grader/harness/runner.py).
const (
	EnvSandboxExec = "SANDBOX_EXEC"
	EnvCgroupRoot  = "SANDBOX_CGROUP_ROOT"
)

// Result.status values (SRS 7.1).
const (
	StatusPassed              = "PASSED"
	StatusFailed              = "FAILED"
	StatusTimeout             = "TIMEOUT"
	StatusMemoryLimitExceeded = "MEMORY_LIMIT_EXCEEDED"
	StatusRuntimeError        = "RUNTIME_ERROR"
	StatusSyntaxError         = "SYNTAX_ERROR"
	StatusSecurityViolation   = "SECURITY_VIOLATION"
)

// Result.memory_source values: where memory_kb came from (SRS 3.2.1.3).
const (
	MemoryCgroupPeak    = "CGROUP_PEAK"
	MemoryCgroupSampled = "CGROUP_SAMPLED"
	MemoryRusage        = "RUSAGE"
)

// TestCase.match_mode values (SRS 7.1).
const (
	MatchExact                 = "EXACT"
	MatchWhitespaceInsensitive = "WHITESPACE_INSENSITIVE"
)

// DefinitionVersion is the schema_version the worker writes.
const DefinitionVersion = 1

// Definition is everything the Harness needs to grade one submission. Field names are the SRS
// attribute names.
type Definition struct {
	SchemaVersion int           `json:"schema_version"`
	SubmissionID  int64         `json:"submission_id"`
	ResourceLimit ResourceLimit `json:"resource_limit"`
	TestCases     []TestCase    `json:"test_cases"`
}

// ResourceLimit is the assignment's ResourceLimit row, without its keys.
type ResourceLimit struct {
	CPUTimeMS      int `json:"cpu_time_ms"`
	MemoryMB       int `json:"memory_mb"`
	MaxProcesses   int `json:"max_processes"`
	TimeoutS       int `json:"timeout_s"`
	MaxOutputBytes int `json:"max_output_bytes"`
	TmpfsSizeMB    int `json:"tmpfs_size_mb"`
}

// TestCase is one test case. ID is the database id, which the report repeats; ExternalID is the name
// used in the YAML file. Whether a test case is hidden is a display rule for the API and is not sent.
type TestCase struct {
	ID             int64  `json:"id"`
	ExternalID     string `json:"external_id"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	Weight         int    `json:"weight"`
	MatchMode      string `json:"match_mode"`
}

// Report is the Harness output (SRS Appendix C).
type Report struct {
	SubmissionID int64    `json:"submission_id"`
	TotalScore   int      `json:"total_score"`
	MaxScore     int      `json:"max_score"`
	Results      []Result `json:"results"`
}

// Result is the outcome of one test case.
type Result struct {
	TestCaseID   int64  `json:"testcase_id"`
	ExternalID   string `json:"external_id"`
	Status       string `json:"status"`
	Score        int    `json:"score"`
	ActualOutput string `json:"actual_output"`
	IsTruncated  bool   `json:"is_truncated"`
	ExecTimeMS   int    `json:"exec_time_ms"`
	MemoryKB     int    `json:"memory_kb"`
	MemorySource string `json:"memory_source"`
	// ErrorMessage is a displayable cause of failure, absent when there is none (SRS 7.1).
	ErrorMessage *string `json:"error_message,omitempty"`
	// Violation is present for SECURITY_VIOLATION and carries the denied syscall for the audit log (FR-12).
	Violation *Violation `json:"violation,omitempty"`
}

// Violation names the syscall the sandbox refused.
type Violation struct {
	SyscallNr int `json:"syscall_nr"`
}
