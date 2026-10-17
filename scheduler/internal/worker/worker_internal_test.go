package worker

import (
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
)

func TestBackstop(t *testing.T) {
	p := &Pool{cfg: Config{TestOverhead: 2 * time.Second, Margin: 30 * time.Second, Lease: 30 * time.Minute}}
	def := func(tests, timeoutS int) *harness.Definition {
		return &harness.Definition{
			ResourceLimit: harness.ResourceLimit{TimeoutS: timeoutS},
			TestCases:     make([]harness.TestCase, tests),
		}
	}
	tests := []struct {
		name string
		def  *harness.Definition
		want time.Duration
	}{
		{"every test may use its timeout and the overhead", def(5, 5), 5*(5+2)*time.Second + 30*time.Second},
		{"no test cases leaves the margin", def(0, 5), 30 * time.Second},
		{"never more than half the lease", def(1000, 10), 15 * time.Minute},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := p.backstop(tc.def); got != tc.want {
				t.Fatalf("backstop = %v, want %v", got, tc.want)
			}
		})
	}
}
