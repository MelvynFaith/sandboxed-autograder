package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/submissions"
)

// regradeSubmission handles POST /api/v1/submissions/{id}/regrade (FR-17): the lecturer of the
// submission's course, or an admin, has one submission graded again. The student who owns it can see
// it but not regrade it (403); anyone else cannot see it (404, NFR-10).
func (a *API) regradeSubmission(w http.ResponseWriter, r *http.Request, user auth.User) {
	submissionID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	// GetVisible's second result is true for an admin and for the lecturer of the course, the two who
	// may manage the submission, and false for the student who owns it.
	_, canManage, err := a.submissions.GetVisible(r.Context(), submissionID, user.ID, user.Role)
	if errors.Is(err, submissions.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "check submission regrade access", err)
		return
	}
	if !canManage {
		writeError(w, http.StatusForbidden, "forbidden", "Hanya dosen pengampu atau administrator yang dapat menilai ulang submission.")
		return
	}

	job, err := a.submissions.Regrade(r.Context(), submissionID)
	switch {
	case errors.Is(err, submissions.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	case errors.Is(err, submissions.ErrRegradeStatus):
		writeError(w, http.StatusConflict, "invalid_status", "Hanya submission berstatus COMPLETED atau FAILED yang dapat dinilai ulang.")
		return
	case errors.Is(err, submissions.ErrRegradeCodeDeleted):
		writeError(w, http.StatusGone, "code_deleted", "Berkas kode sudah dihapus karena masa retensi.")
		return
	case err != nil:
		internalError(w, "regrade submission", err)
		return
	}
	a.enqueueRegraded(r.Context(), job)

	regraded, _, err := a.submissions.GetVisible(r.Context(), submissionID, user.ID, user.Role)
	if err != nil {
		internalError(w, "read regraded submission", err)
		return
	}
	writeJSON(w, http.StatusAccepted, regraded)
}

// regradeAssignment handles POST /api/v1/assignments/{id}/regrade (FR-17): every graded submission of
// the assignment whose code still exists is graded again. A student who can see the assignment gets
// 403; one who cannot, and a lecturer of another course, get 404.
func (a *API) regradeAssignment(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	canManage, err := a.courses.CanManageAssignment(r.Context(), assignmentID, user.ID, user.Role)
	if err != nil {
		internalError(w, "check assignment regrade access", err)
		return
	}
	if !canManage {
		canSee, err := a.courses.CanAccessAssignment(r.Context(), assignmentID, user.ID, user.Role)
		if err != nil {
			internalError(w, "check assignment access for regrade", err)
			return
		}
		if canSee {
			writeError(w, http.StatusForbidden, "forbidden", "Hanya dosen pengampu atau administrator yang dapat menilai ulang assignment.")
		} else {
			writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		}
		return
	}

	jobs, skipped, err := a.submissions.RegradeAssignment(r.Context(), assignmentID)
	if err != nil {
		internalError(w, "regrade assignment", err)
		return
	}
	a.enqueueRegraded(r.Context(), jobs...)
	writeJSON(w, http.StatusAccepted, struct {
		AssignmentID int64 `json:"assignment_id"`
		Queued       int   `json:"queued"`
		Skipped      int   `json:"skipped"`
	}{AssignmentID: assignmentID, Queued: len(jobs), Skipped: skipped})
}

// enqueueRegraded puts regraded submissions in the queue, in the order given. The database already says
// QUEUED, so a job the queue cannot take is not a failure of the request: the worker pool's periodic
// reconcile adds every QUEUED submission the queue lacks. ErrDuplicate means the queue already holds it.
func (a *API) enqueueRegraded(ctx context.Context, jobs ...queue.Job) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	for _, job := range jobs {
		if err := a.queue.Enqueue(ctx, job); err != nil && !errors.Is(err, queue.ErrDuplicate) {
			log.Printf("enqueue regraded submission %d: %v; the periodic reconcile will queue it", job.SubmissionID, err)
		}
	}
}
