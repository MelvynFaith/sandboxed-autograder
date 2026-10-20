package api

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/store"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/submissions"
)

func TestCancelQueuedSubmission(t *testing.T) {
	env := newTestEnvironment(t)
	f := newRegradeFixture(t, env)
	addTestCase(t, env, f, "tc_01", 1)

	submissionID := uploadAsStudent(t, env, f, f.assignmentID)
	path := fmt.Sprintf("/api/v1/submissions/%d", submissionID)
	response, payload := env.request(t, http.MethodDelete, path, nil, f.studentAccess)
	if response.StatusCode != http.StatusOK || payload["status"] != "CANCELLED" || int64Field(t, payload, "id") != submissionID {
		t.Fatalf("cancel a QUEUED submission = %d, %#v; want 200 CANCELLED", response.StatusCode, payload)
	}
	// The row is kept with its history, not deleted.
	if got := readState(t, env, submissionID); got.Status != "CANCELLED" {
		t.Fatalf("after cancel = %+v", got)
	}
	detail, detailPayload := env.request(t, http.MethodGet, path, nil, f.studentAccess)
	if detail.StatusCode != http.StatusOK || detailPayload["status"] != "CANCELLED" {
		t.Fatalf("detail after cancel = %d, %#v", detail.StatusCode, detailPayload)
	}
	// The job left the queue.
	if removed, err := env.jobs.Remove(t.Context(), submissionID); err != nil || removed {
		t.Fatalf("the queue still holds the cancelled job: removed = %t, err = %v", removed, err)
	}
	// A worker that had dequeued the job a moment earlier cannot run it: Claim only takes QUEUED.
	pipeline := store.NewPipeline(env.pool)
	if claim, err := pipeline.Claim(t.Context(), submissionID); err != store.ErrNotQueued || claim != nil {
		t.Fatalf("Claim of a cancelled submission = %v, %v; want ErrNotQueued", claim, err)
	}
	if got := readState(t, env, submissionID); got.Status != "CANCELLED" || got.Attempts != 0 {
		t.Fatalf("the refused claim changed the submission: %+v", got)
	}

	// Cancelling again is not an error of the pipeline but of the state.
	again, againPayload := env.request(t, http.MethodDelete, path, nil, f.studentAccess)
	if again.StatusCode != http.StatusConflict {
		t.Fatalf("cancel twice = %d, %#v; want 409", again.StatusCode, againPayload)
	}
	assertError(t, againPayload, "invalid_status")
}

func TestCancelRefusesWhatIsNotQueued(t *testing.T) {
	env := newTestEnvironment(t)
	f := newRegradeFixture(t, env)

	for _, tc := range []struct {
		status string
		code   string
	}{
		{"RUNNING", "not_cancellable"},
		{"COMPLETED", "invalid_status"},
		{"FAILED", "invalid_status"},
		{"CANCELLED", "invalid_status"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			submissionID := uploadAsStudent(t, env, f, f.assignmentID)
			setStatus(t, env, submissionID, tc.status)
			response, payload := env.request(t, http.MethodDelete, fmt.Sprintf("/api/v1/submissions/%d", submissionID), nil, f.studentAccess)
			if response.StatusCode != http.StatusConflict {
				t.Fatalf("cancel a %s submission = %d, %#v; want 409", tc.status, response.StatusCode, payload)
			}
			assertError(t, payload, tc.code)
			if tc.status == "RUNNING" {
				if message := errorMessage(t, payload); message != "Submission yang sedang dinilai tidak dapat dibatalkan." {
					t.Fatalf("not_cancellable message = %q", message)
				}
			}
			if got := readState(t, env, submissionID).Status; got != tc.status {
				t.Fatalf("a refused cancel moved %s to %s", tc.status, got)
			}
		})
	}
}

func TestCancelAccess(t *testing.T) {
	env := newTestEnvironment(t)
	f := newRegradeFixture(t, env)
	for _, request := range []struct {
		name  string
		token string
		want  int
		code  string
	}{
		{"course lecturer", f.lecturerAccess, http.StatusForbidden, "forbidden"},
		{"admin", f.adminAccess, http.StatusForbidden, "forbidden"},
		{"other student", f.otherStudentAuth, http.StatusNotFound, "not_found"},
		{"other lecturer", f.otherLecturerAuth, http.StatusNotFound, "not_found"},
		{"missing token", "", http.StatusUnauthorized, ""},
		{"owner", f.studentAccess, http.StatusOK, ""},
	} {
		t.Run(request.name, func(t *testing.T) {
			submissionID := uploadAsStudent(t, env, f, f.assignmentID)
			response, payload := env.request(t, http.MethodDelete, fmt.Sprintf("/api/v1/submissions/%d", submissionID), nil, request.token)
			if response.StatusCode != request.want {
				t.Fatalf("cancel as %s = %d, %#v; want %d", request.name, response.StatusCode, payload, request.want)
			}
			if request.code != "" {
				assertError(t, payload, request.code)
			}
			want := "QUEUED"
			if request.want == http.StatusOK {
				want = "CANCELLED"
			}
			if got := readState(t, env, submissionID).Status; got != want {
				t.Fatalf("status after %s = %s; want %s", request.name, got, want)
			}
		})
	}

	for _, id := range []string{"0", "abc", "999999"} {
		response, payload := env.request(t, http.MethodDelete, "/api/v1/submissions/"+id, nil, f.studentAccess)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("cancel submission %q = %d, %#v; want 404", id, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}
}

// A cancel and a worker's Claim race for the same QUEUED submission: exactly one wins, and the
// submission never ends up both cancelled and running. The race is run against the store, because
// the HTTP round trip would let the Claim win every time.
func TestCancelRacesWithClaim(t *testing.T) {
	env := newTestEnvironment(t)
	f := newRegradeFixture(t, env)
	addTestCase(t, env, f, "tc_01", 1)
	pipeline := store.NewPipeline(env.pool)
	submissionStore := submissions.NewStore(env.pool)

	var cancelWins, claimWins int
	for i := range 100 {
		submissionID := uploadAsStudent(t, env, f, f.assignmentID)
		var wg sync.WaitGroup
		var cancelErr, claimErr error
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			cancelErr = submissionStore.Cancel(t.Context(), submissionID, f.student.ID)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, claimErr = pipeline.Claim(t.Context(), submissionID)
		}()
		close(start)
		wg.Wait()

		got := readState(t, env, submissionID).Status
		switch {
		case cancelErr == nil && claimErr == store.ErrNotQueued && got == "CANCELLED":
			cancelWins++
		case cancelErr == submissions.ErrCancelRunning && claimErr == nil && got == "RUNNING":
			claimWins++
		default:
			t.Fatalf("round %d: cancel = %v, claim = %v, status = %s; want exactly one winner", i, cancelErr, claimErr, got)
		}
	}
	t.Logf("cancel won %d rounds, claim won %d", cancelWins, claimWins)
}
