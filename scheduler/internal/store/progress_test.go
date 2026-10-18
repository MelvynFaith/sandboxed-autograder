package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/progress"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/store"
)

func TestProgress(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	want := func(id int64, status string, current, total int) {
		t.Helper()
		got, err := e.p.Progress(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if w := (progress.Event{SubmissionID: id, Status: status, CurrentTest: current, TotalTests: total}); got != w {
			t.Errorf("got %+v, want %+v", got, w)
		}
	}

	// Two live test cases; the soft-deleted third does not count.
	for _, status := range []string{"QUEUED", "RUNNING", "FAILED", "CANCELLED"} {
		want(e.submission(t, status, time.Now()), status, 0, 2)
	}

	id, c := e.claimed(t)
	if err := e.p.Complete(ctx, c, report(c, harness.StatusPassed, harness.StatusFailed)); err != nil {
		t.Fatal(err)
	}
	want(id, "COMPLETED", 2, 2)

	// A completed submission keeps the count it was graded with when a test case is added later.
	e.insert(t, `INSERT INTO test_cases (assignment_id, external_id, input, expected_output, weight, match_mode)
		VALUES ($1, 'tc_04', '', '', 10, 'EXACT') RETURNING id`, e.assignment)
	want(id, "COMPLETED", 2, 2)
	want(e.submission(t, "QUEUED", time.Now()), "QUEUED", 0, 3)

	if _, err := e.p.Progress(ctx, 1<<40); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown submission: %v, want ErrNotFound", err)
	}
}
