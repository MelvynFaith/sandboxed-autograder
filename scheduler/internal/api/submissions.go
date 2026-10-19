package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/ratelimit"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/submissions"
)

func (a *API) createSubmission(w http.ResponseWriter, r *http.Request, user auth.User) {
	if user.Role != auth.RoleStudent {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	if err := r.ParseMultipartForm(maxSubmissionBytes); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "Ukuran upload melebihi batas 256 KiB.")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "Body harus berupa multipart/form-data dengan field assignment_id dan file.")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if r.MultipartForm == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Body harus berupa multipart/form-data.")
		return
	}
	assignmentValues := r.MultipartForm.Value["assignment_id"]
	if len(assignmentValues) != 1 {
		writeError(w, http.StatusBadRequest, "invalid_request", "assignment_id wajib diisi.")
		return
	}
	assignmentID, err := strconv.ParseInt(assignmentValues[0], 10, 64)
	if err != nil || assignmentID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "assignment_id harus bilangan bulat positif.")
		return
	}
	fileHeaders := r.MultipartForm.File["file"]
	if len(fileHeaders) != 1 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Tepat satu berkas harus dikirim pada field file.")
		return
	}
	fileHeader := fileHeaders[0]
	filename := filepath.Base(strings.ReplaceAll(fileHeader.Filename, "\\", "/"))
	if filename == "." || filename == "" || !strings.EqualFold(filepath.Ext(filename), ".py") {
		writeError(w, http.StatusBadRequest, "invalid_file", "Berkas submission harus memiliki ekstensi .py.")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		internalError(w, "open uploaded submission", err)
		return
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxSubmissionBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		internalError(w, "read uploaded submission", readErr)
		return
	}
	if closeErr != nil {
		internalError(w, "close uploaded submission", closeErr)
		return
	}
	if len(data) > maxSubmissionBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "Ukuran upload melebihi batas 256 KiB.")
		return
	}

	uploaded, err := os.CreateTemp(a.codeDir, "submission-*.py")
	if err != nil {
		internalError(w, "create submission file", err)
		return
	}
	codePath := filepath.Base(uploaded.Name())
	if _, err := uploaded.Write(data); err != nil {
		if closeErr := uploaded.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		if removeErr := os.Remove(uploaded.Name()); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("remove incomplete submission: %w", removeErr))
		}
		internalError(w, "write submission file", err)
		return
	}
	if err := uploaded.Sync(); err != nil {
		if closeErr := uploaded.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		if removeErr := os.Remove(uploaded.Name()); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("remove incomplete submission: %w", removeErr))
		}
		internalError(w, "sync submission file", err)
		return
	}
	if err := uploaded.Close(); err != nil {
		if removeErr := os.Remove(uploaded.Name()); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("remove incomplete submission: %w", removeErr))
		}
		internalError(w, "close submission file", err)
		return
	}

	currentSettings, err := a.settings.Get(r.Context())
	if err != nil {
		if removeErr := os.Remove(uploaded.Name()); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("remove rejected submission file: %w", removeErr))
		}
		internalError(w, "load submission rate limits", err)
		return
	}
	limits := ratelimit.Limits{
		MaxActive:        currentSettings.MaxActiveSubmissions,
		MinInterval:      time.Duration(currentSettings.MinSubmissionIntervalS) * time.Second,
		MaxPerAssignment: currentSettings.MaxSubmissionsPerAssignment,
	}
	submission, err := a.submissions.Create(
		r.Context(), user.ID, assignmentID, filename, codePath, len(data), limits,
	)
	if err != nil {
		if removeErr := os.Remove(uploaded.Name()); removeErr != nil {
			internalError(w, "remove rejected submission file", errors.Join(err, removeErr))
			return
		}
		if errors.Is(err, submissions.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
			return
		}
		if errors.Is(err, submissions.ErrDeadlinePassed) {
			writeError(w, http.StatusConflict, "deadline_passed", "Batas akhir pengumpulan sudah lewat.")
			return
		}
		var limitErr *ratelimit.Error
		if errors.As(err, &limitErr) {
			writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", limitErr.Error())
			return
		}
		internalError(w, "create submission", err)
		return
	}

	enqueueCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()
	if err := a.queue.Enqueue(enqueueCtx, queue.Job{SubmissionID: submission.ID, UserID: user.ID}); err != nil &&
		!errors.Is(err, queue.ErrDuplicate) {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
		deleted, cleanupErr := a.submissions.DeleteQueued(cleanupCtx, submission.ID)
		cleanupCancel()
		if deleted {
			if removeErr := os.Remove(uploaded.Name()); removeErr != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove unqueued submission file: %w", removeErr))
			}
		}
		internalError(w, "enqueue submission", errors.Join(err, cleanupErr))
		return
	}
	writeJSON(w, http.StatusCreated, submission)
}

func (a *API) listSubmissions(w http.ResponseWriter, r *http.Request, user auth.User) {
	page, pageSize, ok := parsePage(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "page dan page_size harus bilangan bulat positif; page_size maksimal 100.")
		return
	}
	result, err := a.submissions.ListByUser(r.Context(), user.ID, page, pageSize)
	if err != nil {
		internalError(w, "list submissions", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) getSubmission(w http.ResponseWriter, r *http.Request, user auth.User) {
	submissionID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	submission, err := a.submissions.GetVisible(r.Context(), submissionID, user.ID, user.Role)
	if errors.Is(err, submissions.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "get submission", err)
		return
	}
	writeJSON(w, http.StatusOK, submission)
}

func (a *API) getSubmissionReport(w http.ResponseWriter, r *http.Request, user auth.User) {
	submissionID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	submission, err := a.submissions.GetVisible(r.Context(), submissionID, user.ID, user.Role)
	if errors.Is(err, submissions.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "check submission report access", err)
		return
	}
	report, err := a.submissions.Report(r.Context(), submission, user.Role)
	if err != nil {
		internalError(w, "get submission report", err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}
