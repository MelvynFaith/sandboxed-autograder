package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/health"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/settings"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/submissions"
)

var invalidSettingsMessage = fmt.Sprintf(
	"max_active_submissions, min_submission_interval_s, dan max_submissions_per_assignment wajib "+
		"diisi bilangan bulat; max_active_submissions dan max_submissions_per_assignment 0 sampai %d, "+
		"min_submission_interval_s 0 sampai %d detik; 0 mematikan aturan tersebut.",
	settings.MaxCount, settings.MaxInterval)

// queueStatus is the response of GET /api/v1/admin/queue (FR-13, US-21). QueueDepth and Workers are
// the numbers /healthz reports; Submissions comes from the database.
type queueStatus struct {
	QueueDepth  int                      `json:"queue_depth"`
	Workers     health.WorkerCount       `json:"workers"`
	Submissions submissions.ActiveCounts `json:"submissions"`
}

// rateLimits is the body and the response of /api/v1/admin/rate-limits (FR-14).
type rateLimits struct {
	MaxActiveSubmissions        int `json:"max_active_submissions"`
	MinSubmissionIntervalS      int `json:"min_submission_interval_s"`
	MaxSubmissionsPerAssignment int `json:"max_submissions_per_assignment"`
}

// rateLimitsRequest has pointers so that a missing field is refused instead of read as 0, which
// would quietly switch its rule off.
type rateLimitsRequest struct {
	MaxActiveSubmissions        *int `json:"max_active_submissions"`
	MinSubmissionIntervalS      *int `json:"min_submission_interval_s"`
	MaxSubmissionsPerAssignment *int `json:"max_submissions_per_assignment"`
}

func rateLimitsOf(s settings.Settings) rateLimits {
	return rateLimits{
		MaxActiveSubmissions:        s.MaxActiveSubmissions,
		MinSubmissionIntervalS:      s.MinSubmissionIntervalS,
		MaxSubmissionsPerAssignment: s.MaxSubmissionsPerAssignment,
	}
}

func (a *API) listAuditLogs(w http.ResponseWriter, r *http.Request, _ auth.User) {
	page, pageSize, ok := parsePage(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "page dan page_size harus bilangan bulat positif; page_size maksimal 100.")
		return
	}
	result, err := a.auditLogs.List(r.Context(), page, pageSize)
	if err != nil {
		internalError(w, "list audit logs", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) getQueue(w http.ResponseWriter, r *http.Request, _ auth.User) {
	counts, err := a.submissions.CountActive(r.Context())
	if err != nil {
		internalError(w, "count active submissions", err)
		return
	}
	workers, depth := a.checker.Capacity()
	writeJSON(w, http.StatusOK, queueStatus{QueueDepth: depth, Workers: workers, Submissions: counts})
}

func (a *API) getRateLimits(w http.ResponseWriter, r *http.Request, _ auth.User) {
	current, err := a.settings.Get(r.Context())
	if err != nil {
		internalError(w, "get rate limits", err)
		return
	}
	writeJSON(w, http.StatusOK, rateLimitsOf(current))
}

// putRateLimits saves new FR-14 thresholds. Uploads read them on every request, so the next upload
// is checked against them. settings.Update writes the SETTINGS_CHANGED audit row naming the admin.
func (a *API) putRateLimits(w http.ResponseWriter, r *http.Request, admin auth.User) {
	var request rateLimitsRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.MaxActiveSubmissions == nil || request.MinSubmissionIntervalS == nil ||
		request.MaxSubmissionsPerAssignment == nil {
		writeError(w, http.StatusBadRequest, "invalid_settings", invalidSettingsMessage)
		return
	}
	saved, err := a.settings.Update(r.Context(), admin.ID, settings.Settings{
		MaxActiveSubmissions:        *request.MaxActiveSubmissions,
		MinSubmissionIntervalS:      *request.MinSubmissionIntervalS,
		MaxSubmissionsPerAssignment: *request.MaxSubmissionsPerAssignment,
	})
	if errors.Is(err, settings.ErrInvalid) {
		writeError(w, http.StatusBadRequest, "invalid_settings", invalidSettingsMessage)
		return
	}
	if err != nil {
		internalError(w, "update rate limits", err)
		return
	}
	writeJSON(w, http.StatusOK, rateLimitsOf(saved))
}
