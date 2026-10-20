package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/submissions"
)

// cancelSubmission handles DELETE /api/v1/submissions/{id} (FR-16): a student cancels their own
// submission while it is still QUEUED. A lecturer or admin who can see the submission gets 403; anyone
// else gets 404 (NFR-10). The row is kept with status CANCELLED, not deleted.
func (a *API) cancelSubmission(w http.ResponseWriter, r *http.Request, user auth.User) {
	submissionID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	submission, _, err := a.submissions.GetVisible(r.Context(), submissionID, user.ID, user.Role)
	if errors.Is(err, submissions.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "check submission cancel access", err)
		return
	}
	if submission.UserID != user.ID {
		writeError(w, http.StatusForbidden, "forbidden", "Hanya pemilik submission yang dapat membatalkannya.")
		return
	}

	err = a.submissions.Cancel(r.Context(), submissionID, user.ID)
	switch {
	case errors.Is(err, submissions.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	case errors.Is(err, submissions.ErrCancelRunning):
		writeError(w, http.StatusConflict, "not_cancellable", "Submission yang sedang dinilai tidak dapat dibatalkan.")
		return
	case errors.Is(err, submissions.ErrCancelStatus):
		writeError(w, http.StatusConflict, "invalid_status", "Hanya submission berstatus QUEUED yang dapat dibatalkan.")
		return
	case err != nil:
		internalError(w, "cancel submission", err)
		return
	}

	// The submission is CANCELLED now, so a worker that dequeued the job a moment ago finds it so when it
	// claims it and drops the job. Removing the job only keeps a dead entry out of the queue; if that
	// fails, the entry is dropped the same way when its turn comes.
	removeCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()
	if _, err := a.queue.Remove(removeCtx, submissionID); err != nil {
		log.Printf("remove cancelled submission %d from the queue: %v", submissionID, err)
	}

	cancelled, _, err := a.submissions.GetVisible(r.Context(), submissionID, user.ID, user.Role)
	if err != nil {
		internalError(w, "read cancelled submission", err)
		return
	}
	writeJSON(w, http.StatusOK, cancelled)
}
