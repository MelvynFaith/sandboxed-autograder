package api

import (
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/courses"
	"github.com/jackc/pgx/v5"
)

func (a *API) createAssignment(w http.ResponseWriter, r *http.Request, user auth.User) {
	courseID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Course tidak ditemukan.")
		return
	}
	if !a.requireCourseManager(w, r, user, courseID) {
		return
	}
	request, ok := decodeAssignmentRequest(w, r)
	if !ok {
		return
	}
	if request.ResourceLimit == nil || !request.ResourceLimit.valid() {
		writeError(w, http.StatusBadRequest, "invalid_request", "resource_limit wajib diisi dengan batas sumber daya yang valid.")
		return
	}
	assignment, err := a.courses.CreateAssignment(
		r.Context(), courseID, request.Title, request.Description, request.DeadlineAt, request.LateUntilAt,
		courses.ResourceLimit{
			CPUTimeMS: request.ResourceLimit.CPUTimeMS, MemoryMB: request.ResourceLimit.MemoryMB,
			MaxProcesses: request.ResourceLimit.MaxProcesses,
			TimeoutS:     request.ResourceLimit.TimeoutS, MaxOutputBytes: request.ResourceLimit.MaxOutputBytes,
			TmpfsSizeMB: request.ResourceLimit.TmpfsSizeMB,
		},
	)
	if err != nil {
		internalError(w, "create assignment", err)
		return
	}
	writeJSON(w, http.StatusCreated, assignment)
}

func (a *API) listAssignments(w http.ResponseWriter, r *http.Request, user auth.User) {
	page, pageSize, ok := parsePage(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "page dan page_size harus bilangan bulat positif; page_size maksimal 100.")
		return
	}
	result, err := a.courses.ListVisibleAssignments(r.Context(), user.Role, user.ID, page, pageSize)
	if err != nil {
		internalError(w, "list assignments", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) getAssignment(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireAssignmentAccess(w, r, user, assignmentID) {
		return
	}
	assignment, err := a.courses.AssignmentByID(r.Context(), assignmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "get assignment", err)
		return
	}
	writeJSON(w, http.StatusOK, assignment)
}

func (a *API) updateAssignment(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireAssignmentManager(w, r, user, assignmentID) {
		return
	}
	request, ok := decodeAssignmentRequest(w, r)
	if !ok {
		return
	}
	updated, err := a.courses.UpdateAssignment(
		r.Context(), assignmentID, request.Title, request.Description, request.DeadlineAt, request.LateUntilAt,
	)
	if err != nil {
		internalError(w, "update assignment", err)
		return
	}
	if !updated {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	assignment, err := a.courses.AssignmentByID(r.Context(), assignmentID)
	if err != nil {
		internalError(w, "get updated assignment", err)
		return
	}
	writeJSON(w, http.StatusOK, assignment)
}

func (a *API) deleteAssignment(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireAssignmentManager(w, r, user, assignmentID) {
		return
	}
	deleted, err := a.courses.DeleteAssignment(r.Context(), assignmentID)
	if err != nil {
		internalError(w, "delete assignment", err)
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) getResourceLimit(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireAssignmentAccess(w, r, user, assignmentID) {
		return
	}
	limit, err := a.courses.GetResourceLimit(r.Context(), assignmentID)
	if err != nil {
		internalError(w, "get resource limit", err)
		return
	}
	writeJSON(w, http.StatusOK, limit)
}

func (a *API) putResourceLimit(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireAssignmentManager(w, r, user, assignmentID) {
		return
	}
	var request resourceLimitRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if !request.valid() {
		writeError(w, http.StatusBadRequest, "invalid_resource_limit", "Semua batas harus bilangan bulat positif; tmpfs_size_mb + 16 harus <= memory_mb dan 2 * max_output_bytes harus < tmpfs_size_mb * 2^20.")
		return
	}

	limit, err := a.courses.UpdateResourceLimit(r.Context(), courses.ResourceLimit{
		AssignmentID: assignmentID, CPUTimeMS: request.CPUTimeMS, MemoryMB: request.MemoryMB,
		MaxProcesses: request.MaxProcesses, TimeoutS: request.TimeoutS,
		MaxOutputBytes: request.MaxOutputBytes, TmpfsSizeMB: request.TmpfsSizeMB,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "update resource limit", err)
		return
	}
	writeJSON(w, http.StatusOK, limit)
}

func (a *API) listTestCases(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireAssignmentAccess(w, r, user, assignmentID) {
		return
	}
	page, pageSize, ok := parsePage(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "page dan page_size harus bilangan bulat positif; page_size maksimal 100.")
		return
	}
	result, err := a.courses.ListTestCases(
		r.Context(), assignmentID, user.Role != auth.RoleStudent, page, pageSize,
	)
	if err != nil {
		internalError(w, "list test cases", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) createTestCase(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireAssignmentManager(w, r, user, assignmentID) {
		return
	}
	request, ok := decodeTestCaseRequest(w, r)
	if !ok {
		return
	}
	testCase, err := a.courses.CreateTestCase(r.Context(), courses.TestCase{
		AssignmentID: assignmentID, ExternalID: request.ExternalID, Input: request.Input,
		ExpectedOutput: request.ExpectedOutput, Weight: request.Weight,
		IsHidden: request.IsHidden, MatchMode: request.MatchMode,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "test_case_external_id_exists", "ID eksternal test case sudah digunakan pada assignment ini.")
			return
		}
		internalError(w, "create test case", err)
		return
	}
	writeJSON(w, http.StatusCreated, testCase)
}

func (a *API) updateTestCase(w http.ResponseWriter, r *http.Request, user auth.User) {
	testCaseID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Test case tidak ditemukan.")
		return
	}
	allowed, err := a.courses.CanManageTestCase(r.Context(), testCaseID, user.ID, user.Role)
	if err != nil {
		internalError(w, "check test case management access", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "Test case tidak ditemukan.")
		return
	}
	request, ok := decodeTestCaseRequest(w, r)
	if !ok {
		return
	}
	updated, found, err := a.courses.UpdateTestCase(r.Context(), courses.TestCase{
		ID: testCaseID, ExternalID: request.ExternalID, Input: request.Input, ExpectedOutput: request.ExpectedOutput,
		Weight: request.Weight, IsHidden: request.IsHidden, MatchMode: request.MatchMode,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "test_case_external_id_exists", "ID eksternal test case sudah digunakan pada assignment ini.")
			return
		}
		internalError(w, "update test case", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "not_found", "Test case tidak ditemukan.")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteTestCase(w http.ResponseWriter, r *http.Request, user auth.User) {
	testCaseID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Test case tidak ditemukan.")
		return
	}
	allowed, err := a.courses.CanManageTestCase(r.Context(), testCaseID, user.ID, user.Role)
	if err != nil {
		internalError(w, "check test case management access", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "Test case tidak ditemukan.")
		return
	}
	deleted, err := a.courses.DeleteTestCase(r.Context(), testCaseID)
	if err != nil {
		internalError(w, "delete test case", err)
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "not_found", "Test case tidak ditemukan.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type assignmentRequest struct {
	Title         string
	Description   string
	DeadlineAt    time.Time
	LateUntilAt   time.Time
	ResourceLimit *resourceLimitRequest
}

func decodeAssignmentRequest(w http.ResponseWriter, r *http.Request) (assignmentRequest, bool) {
	var payload struct {
		Title         string                `json:"title"`
		Description   string                `json:"description"`
		DeadlineAt    string                `json:"deadline_at"`
		LateUntilAt   string                `json:"late_until_at"`
		ResourceLimit *resourceLimitRequest `json:"resource_limit"`
	}
	if !decodeJSON(w, r, &payload) {
		return assignmentRequest{}, false
	}
	payload.Title = strings.TrimSpace(payload.Title)
	deadlineAt, deadlineErr := time.Parse(time.RFC3339, payload.DeadlineAt)
	lateUntilAt, lateUntilErr := time.Parse(time.RFC3339, payload.LateUntilAt)
	if payload.Title == "" || strings.ContainsRune(payload.Title, '\x00') ||
		strings.ContainsRune(payload.Description, '\x00') || deadlineErr != nil ||
		lateUntilErr != nil || lateUntilAt.Before(deadlineAt) {
		writeError(w, http.StatusBadRequest, "invalid_assignment", "Judul wajib diisi; deadline harus berupa waktu RFC3339 dan late_until_at harus sama dengan atau setelah deadline_at.")
		return assignmentRequest{}, false
	}
	return assignmentRequest{
		Title: payload.Title, Description: payload.Description,
		DeadlineAt: deadlineAt, LateUntilAt: lateUntilAt, ResourceLimit: payload.ResourceLimit,
	}, true
}

type resourceLimitRequest struct {
	CPUTimeMS      int `json:"cpu_time_ms"`
	MemoryMB       int `json:"memory_mb"`
	MaxProcesses   int `json:"max_processes"`
	TimeoutS       int `json:"timeout_s"`
	MaxOutputBytes int `json:"max_output_bytes"`
	TmpfsSizeMB    int `json:"tmpfs_size_mb"`
}

func (request resourceLimitRequest) valid() bool {
	return validResourceLimit(request.CPUTimeMS, request.MemoryMB, request.MaxProcesses,
		request.TimeoutS, request.MaxOutputBytes, request.TmpfsSizeMB)
}

type testCaseRequest struct {
	ExternalID     string `json:"external_id"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	Weight         int    `json:"weight"`
	IsHidden       bool   `json:"is_hidden"`
	MatchMode      string `json:"match_mode"`
}

func decodeTestCaseRequest(w http.ResponseWriter, r *http.Request) (testCaseRequest, bool) {
	var request testCaseRequest
	if !decodeJSON(w, r, &request) {
		return testCaseRequest{}, false
	}
	request.ExternalID = strings.TrimSpace(request.ExternalID)
	if strings.ContainsRune(request.ExternalID, '\x00') ||
		request.Weight <= 0 ||
		(request.MatchMode != "EXACT" && request.MatchMode != "WHITESPACE_INSENSITIVE") {
		writeError(w, http.StatusBadRequest, "invalid_test_case", "external_id tidak boleh mengandung NUL; weight harus bilangan bulat positif dan match_mode harus EXACT atau WHITESPACE_INSENSITIVE.")
		return testCaseRequest{}, false
	}
	return request, true
}

func validResourceLimit(cpuTimeMS, memoryMB, maxProcesses, timeoutS, maxOutputBytes, tmpfsSizeMB int) bool {
	const maxInt32 = int(math.MaxInt32)
	values := []int{cpuTimeMS, memoryMB, maxProcesses, timeoutS, maxOutputBytes, tmpfsSizeMB}
	for _, value := range values {
		if value <= 0 || value > maxInt32 {
			return false
		}
	}
	return int64(tmpfsSizeMB)+16 <= int64(memoryMB) &&
		2*int64(maxOutputBytes) < int64(tmpfsSizeMB)*(1<<20)
}

func (a *API) requireAssignmentAccess(w http.ResponseWriter, r *http.Request, user auth.User, assignmentID int64) bool {
	allowed, err := a.courses.CanAccessAssignment(r.Context(), assignmentID, user.ID, user.Role)
	if err != nil {
		internalError(w, "check assignment access", err)
		return false
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return false
	}
	return true
}

func (a *API) requireAssignmentManager(w http.ResponseWriter, r *http.Request, user auth.User, assignmentID int64) bool {
	allowed, err := a.courses.CanManageAssignment(r.Context(), assignmentID, user.ID, user.Role)
	if err != nil {
		internalError(w, "check assignment management access", err)
		return false
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return false
	}
	return true
}
