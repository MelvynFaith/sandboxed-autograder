package api

import (
	"errors"
	"net/http"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/submissions"
)

// getSubmissionCode serves FR-20: the source of a submission to its owner, the lecturer of its
// course and an admin. Everyone else gets 404, so the existence of the submission does not leak
// (NFR-10); the access check comes before the retention check for the same reason.
func (a *API) getSubmissionCode(w http.ResponseWriter, r *http.Request, user auth.User) {
	submissionID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	code, err := a.submissions.Code(r.Context(), a.codeDir, submissionID, user.ID, user.Role)
	if errors.Is(err, submissions.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	if errors.Is(err, submissions.ErrCodeDeleted) {
		writeError(w, http.StatusGone, "code_deleted", "Berkas kode sudah dihapus karena masa retensi.")
		return
	}
	if err != nil {
		internalError(w, "get submission code", err)
		return
	}
	writeJSON(w, http.StatusOK, code)
}
