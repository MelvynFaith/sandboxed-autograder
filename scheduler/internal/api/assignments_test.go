package api

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
)

func TestAssignmentResourceLimitAndTestCaseLifecycle(t *testing.T) {
	env := newTestEnvironment(t)
	_, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	_, otherLecturerAccess := createCourseTestUser(t, env, "Other Lecturer", "other@example.test", auth.RoleLecturer)
	student, studentAccess := createCourseTestUser(t, env, "Student", "student@example.test", auth.RoleStudent)
	_, otherStudentAccess := createCourseTestUser(t, env, "Other Student", "other-student@example.test", auth.RoleStudent)
	_, adminAccess := createCourseTestUser(t, env, "Admin", "admin@example.test", auth.RoleAdmin)

	courseResponse, course := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Algorithms", "code": "CS101", "term": "2026",
	}, lecturerAccess)
	if courseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create course = %d, %#v", courseResponse.StatusCode, course)
	}
	courseID := int64Field(t, course, "id")
	coursePath := fmt.Sprintf("/api/v1/courses/%d", courseID)
	enrolled, _ := env.request(t, http.MethodPost, coursePath+"/enrollments", map[string]string{
		"email": student.Email,
	}, lecturerAccess)
	if enrolled.StatusCode != http.StatusNoContent {
		t.Fatalf("enroll student = %d; want 204", enrolled.StatusCode)
	}

	assignmentResponse, assignment := env.request(t, http.MethodPost, coursePath+"/assignments", map[string]string{
		"title": "Sorting", "description": "Sort the input.",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
	}, lecturerAccess)
	if assignmentResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create assignment = %d, %#v", assignmentResponse.StatusCode, assignment)
	}
	assignmentID := int64Field(t, assignment, "id")
	assignmentPath := fmt.Sprintf("/api/v1/assignments/%d", assignmentID)
	courseAssignmentPath := coursePath + "/assignments"
	if assignment["title"] != "Sorting" || int64Field(t, assignment, "course_id") != courseID {
		t.Fatalf("assignment response = %#v", assignment)
	}

	resourceLimit := map[string]int{
		"cpu_time_ms": 2000, "memory_mb": 64, "max_processes": 5,
		"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
	}
	missingLimit, missingLimitPayload := env.request(t, http.MethodGet, assignmentPath+"/resource-limit", nil, lecturerAccess)
	if missingLimit.StatusCode != http.StatusNotFound {
		t.Fatalf("get unset resource limit = %d, %#v; want 404", missingLimit.StatusCode, missingLimitPayload)
	}
	assertError(t, missingLimitPayload, "not_found")
	limitResponse, limit := env.request(t, http.MethodPut, assignmentPath+"/resource-limit", resourceLimit, lecturerAccess)
	if limitResponse.StatusCode != http.StatusOK {
		t.Fatalf("set resource limit = %d, %#v", limitResponse.StatusCode, limit)
	}
	if int64Field(t, limit, "assignment_id") != assignmentID || limit["memory_mb"] != float64(64) {
		t.Fatalf("resource limit response = %#v", limit)
	}
	getLimit, getLimitPayload := env.request(t, http.MethodGet, assignmentPath+"/resource-limit", nil, lecturerAccess)
	if getLimit.StatusCode != http.StatusOK || getLimitPayload["tmpfs_size_mb"] != float64(32) {
		t.Fatalf("get resource limit = %d, %#v", getLimit.StatusCode, getLimitPayload)
	}

	submissionIDs := make([]int64, 0, 2)
	for i := 0; i < 2; i++ {
		var submissionID int64
		err := env.pool.QueryRow(t.Context(), `
			INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes)
			VALUES ($1, $2, $3, 'solution.py', 1) RETURNING id
		`, student.ID, assignmentID, fmt.Sprintf("test/path/%d", i)).Scan(&submissionID)
		if err != nil {
			t.Fatalf("insert submission %d: %v", i, err)
		}
		submissionIDs = append(submissionIDs, submissionID)
	}

	visibleCase := map[string]any{
		"input": "1 2", "expected_output": "2 1", "weight": 2,
		"is_hidden": false, "match_mode": "EXACT",
	}
	createdCase, visiblePayload := env.request(t, http.MethodPost, assignmentPath+"/testcases", visibleCase, lecturerAccess)
	if createdCase.StatusCode != http.StatusCreated {
		t.Fatalf("create visible test case = %d, %#v", createdCase.StatusCode, visiblePayload)
	}
	if visiblePayload["external_id"] != "tc_01" {
		t.Fatalf("generated external_id = %#v; want tc_01", visiblePayload["external_id"])
	}
	hiddenCase := map[string]any{
		"external_id": "hidden_case", "input": "secret input", "expected_output": "secret output", "weight": 3,
		"is_hidden": true, "match_mode": "WHITESPACE_INSENSITIVE",
	}
	createdHidden, hiddenPayload := env.request(t, http.MethodPost, assignmentPath+"/testcases", hiddenCase, lecturerAccess)
	if createdHidden.StatusCode != http.StatusCreated {
		t.Fatalf("create hidden test case = %d, %#v", createdHidden.StatusCode, hiddenPayload)
	}
	if hiddenPayload["external_id"] != "hidden_case" {
		t.Fatalf("provided external_id = %#v; want hidden_case", hiddenPayload["external_id"])
	}
	testCaseID := int64Field(t, visiblePayload, "id")
	hiddenTestCaseID := int64Field(t, hiddenPayload, "id")
	for _, submissionID := range submissionIDs {
		assertSubmissionStale(t, env, submissionID, true)
	}
	studentWriteCase, studentWriteCasePayload := env.request(t, http.MethodPost, assignmentPath+"/testcases", visibleCase, studentAccess)
	if studentWriteCase.StatusCode != http.StatusForbidden {
		t.Fatalf("student create test case = %d, %#v; want 403", studentWriteCase.StatusCode, studentWriteCasePayload)
	}
	invalidCase, invalidCasePayload := env.request(t, http.MethodPost, assignmentPath+"/testcases", map[string]any{
		"input": "", "expected_output": "", "weight": 1, "is_hidden": false, "match_mode": "APPROXIMATE",
	}, lecturerAccess)
	if invalidCase.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid match_mode = %d, %#v; want 400", invalidCase.StatusCode, invalidCasePayload)
	}

	duplicateCase := map[string]any{
		"external_id": "hidden_case", "input": "duplicate", "expected_output": "duplicate",
		"weight": 1, "is_hidden": false, "match_mode": "EXACT",
	}
	duplicateResponse, duplicatePayload := env.request(t, http.MethodPost, assignmentPath+"/testcases", duplicateCase, lecturerAccess)
	if duplicateResponse.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate external_id = %d, %#v; want 409", duplicateResponse.StatusCode, duplicatePayload)
	}
	assertError(t, duplicatePayload, "test_case_external_id_exists")
	duplicateUpdate, duplicateUpdatePayload := env.request(t, http.MethodPut,
		"/api/v1/testcases/"+strconv.FormatInt(testCaseID, 10), duplicateCase, lecturerAccess)
	if duplicateUpdate.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate external_id on update = %d, %#v; want 409", duplicateUpdate.StatusCode, duplicateUpdatePayload)
	}
	missingUpdate, missingUpdatePayload := env.request(t, http.MethodPut, "/api/v1/testcases/999999", duplicateCase, lecturerAccess)
	if missingUpdate.StatusCode != http.StatusNotFound {
		t.Fatalf("update missing test case = %d, %#v; want 404", missingUpdate.StatusCode, missingUpdatePayload)
	}
	missingDelete, missingDeletePayload := env.request(t, http.MethodDelete, "/api/v1/testcases/999999", nil, lecturerAccess)
	if missingDelete.StatusCode != http.StatusNotFound {
		t.Fatalf("delete missing test case = %d, %#v; want 404", missingDelete.StatusCode, missingDeletePayload)
	}

	studentAssignment, studentAssignmentPayload := env.request(t, http.MethodGet, assignmentPath, nil, studentAccess)
	if studentAssignment.StatusCode != http.StatusOK || studentAssignmentPayload["title"] != "Sorting" {
		t.Fatalf("enrolled student assignment = %d, %#v", studentAssignment.StatusCode, studentAssignmentPayload)
	}
	studentList, studentListPayload := env.request(t, http.MethodGet, "/api/v1/assignments?page=1&page_size=10", nil, studentAccess)
	if studentList.StatusCode != http.StatusOK || len(pageItems(t, studentListPayload)) != 1 {
		t.Fatalf("student assignment list = %d, %#v", studentList.StatusCode, studentListPayload)
	}
	studentCourseList, studentCoursePayload := env.request(t, http.MethodGet, courseAssignmentPath+"?page=1&page_size=10", nil, studentAccess)
	if studentCourseList.StatusCode != http.StatusOK || len(pageItems(t, studentCoursePayload)) != 1 {
		t.Fatalf("student course assignment list = %d, %#v", studentCourseList.StatusCode, studentCoursePayload)
	}

	studentCases, studentCasesPayload := env.request(t, http.MethodGet, assignmentPath+"/testcases", nil, studentAccess)
	studentItems := pageItems(t, studentCasesPayload)
	if studentCases.StatusCode != http.StatusOK || len(studentItems) != 1 {
		t.Fatalf("student visible test cases = %d, %#v", studentCases.StatusCode, studentCasesPayload)
	}
	publicCase, ok := studentItems[0].(map[string]any)
	if !ok || publicCase["input"] != "1 2" || publicCase["expected_output"] != "2 1" {
		t.Fatalf("student test case = %#v", studentItems[0])
	}
	if _, exists := studentCasesPayload["secret input"]; exists {
		t.Fatal("hidden test input was exposed")
	}

	lecturerCases, lecturerCasesPayload := env.request(t, http.MethodGet, assignmentPath+"/testcases", nil, lecturerAccess)
	if lecturerCases.StatusCode != http.StatusOK || len(pageItems(t, lecturerCasesPayload)) != 2 {
		t.Fatalf("lecturer test cases = %d, %#v", lecturerCases.StatusCode, lecturerCasesPayload)
	}

	if _, err := env.pool.Exec(t.Context(), `UPDATE submissions SET stale_grade = false WHERE assignment_id = $1`, assignmentID); err != nil {
		t.Fatalf("reset stale grades before update: %v", err)
	}
	updated, updatedPayload := env.request(t, http.MethodPut,
		"/api/v1/testcases/"+strconv.FormatInt(testCaseID, 10),
		map[string]any{
			"external_id": "tc_updated", "input": "3 4", "expected_output": "4 3", "weight": 4,
			"is_hidden": false, "match_mode": "EXACT",
		}, lecturerAccess)
	if updated.StatusCode != http.StatusOK || updatedPayload["external_id"] != "tc_updated" {
		t.Fatalf("update test case = %d, %#v", updated.StatusCode, updatedPayload)
	}
	for _, submissionID := range submissionIDs {
		assertSubmissionStale(t, env, submissionID, true)
	}
	if _, err := env.pool.Exec(t.Context(), `UPDATE submissions SET stale_grade = false WHERE assignment_id = $1`, assignmentID); err != nil {
		t.Fatalf("reset stale grades before delete: %v", err)
	}
	deleted, deletedPayload := env.request(t, http.MethodDelete,
		"/api/v1/testcases/"+strconv.FormatInt(testCaseID, 10), nil, lecturerAccess)
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("delete test case = %d, %#v", deleted.StatusCode, deletedPayload)
	}
	for _, submissionID := range submissionIDs {
		assertSubmissionStale(t, env, submissionID, true)
	}
	var visibleStillExists bool
	if err := env.pool.QueryRow(t.Context(), `SELECT EXISTS (SELECT 1 FROM test_cases WHERE id = $1)`, testCaseID).Scan(&visibleStillExists); err != nil {
		t.Fatalf("check physically deleted test case: %v", err)
	}
	if visibleStillExists {
		t.Fatal("test case without results was not physically deleted")
	}
	afterDelete, afterDeletePayload := env.request(t, http.MethodGet, assignmentPath+"/testcases", nil, lecturerAccess)
	if afterDelete.StatusCode != http.StatusOK || len(pageItems(t, afterDeletePayload)) != 1 {
		t.Fatalf("test cases after retirement = %d, %#v", afterDelete.StatusCode, afterDeletePayload)
	}
	if _, err := env.pool.Exec(t.Context(), `
		INSERT INTO results (submission_id, testcase_id, status, exec_time_ms, memory_kb, memory_source, score)
		VALUES ($1, $2, 'PASSED', 1, 1, 'CGROUP_PEAK', 3)
	`, submissionIDs[0], hiddenTestCaseID); err != nil {
		t.Fatalf("insert result for soft-delete test: %v", err)
	}
	if _, err := env.pool.Exec(t.Context(), `UPDATE submissions SET stale_grade = false WHERE assignment_id = $1`, assignmentID); err != nil {
		t.Fatalf("reset stale grades before soft delete: %v", err)
	}
	deletedHidden, deletedHiddenPayload := env.request(t, http.MethodDelete,
		"/api/v1/testcases/"+strconv.FormatInt(hiddenTestCaseID, 10), nil, lecturerAccess)
	if deletedHidden.StatusCode != http.StatusNoContent {
		t.Fatalf("delete test case with result = %d, %#v", deletedHidden.StatusCode, deletedHiddenPayload)
	}
	var hiddenRetired bool
	if err := env.pool.QueryRow(t.Context(), `SELECT is_deleted FROM test_cases WHERE id = $1`, hiddenTestCaseID).Scan(&hiddenRetired); err != nil {
		t.Fatalf("check soft-deleted test case: %v", err)
	}
	if !hiddenRetired {
		t.Fatal("test case with results was not soft deleted")
	}
	afterSoftDelete, afterSoftDeletePayload := env.request(t, http.MethodGet, assignmentPath+"/testcases", nil, lecturerAccess)
	if afterSoftDelete.StatusCode != http.StatusOK || len(pageItems(t, afterSoftDeletePayload)) != 0 {
		t.Fatalf("test cases after soft deletion = %d, %#v", afterSoftDelete.StatusCode, afterSoftDeletePayload)
	}
	for _, submissionID := range submissionIDs {
		assertSubmissionStale(t, env, submissionID, true)
	}

	updatedAssignment, updatedAssignmentPayload := env.request(t, http.MethodPut, assignmentPath, map[string]string{
		"title": "Sorting updated", "description": "Updated.",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
	}, lecturerAccess)
	if updatedAssignment.StatusCode != http.StatusOK || updatedAssignmentPayload["title"] != "Sorting updated" {
		t.Fatalf("update assignment = %d, %#v", updatedAssignment.StatusCode, updatedAssignmentPayload)
	}

	for _, request := range []struct {
		name   string
		method string
		path   string
		body   any
		token  string
		want   int
	}{
		{"student write forbidden", http.MethodPost, courseAssignmentPath, map[string]string{
			"title": "No", "description": "", "deadline_at": "2026-10-20T12:00:00Z",
			"late_until_at": "2026-10-21T12:00:00Z",
		}, studentAccess, http.StatusForbidden},
		{"unenrolled student hidden", http.MethodGet, assignmentPath, nil, otherStudentAccess, http.StatusNotFound},
		{"other lecturer hidden", http.MethodGet, assignmentPath, nil, otherLecturerAccess, http.StatusNotFound},
		{"missing token", http.MethodGet, assignmentPath, nil, "", http.StatusUnauthorized},
		{"admin may read", http.MethodGet, assignmentPath, nil, adminAccess, http.StatusOK},
	} {
		t.Run(request.name, func(t *testing.T) {
			response, _ := env.request(t, request.method, request.path, request.body, request.token)
			if response.StatusCode != request.want {
				t.Fatalf("%s status = %d; want %d", request.name, response.StatusCode, request.want)
			}
		})
	}
}

func assertSubmissionStale(t *testing.T, env *testEnvironment, submissionID int64, want bool) {
	t.Helper()
	var stale bool
	if err := env.pool.QueryRow(t.Context(), `SELECT stale_grade FROM submissions WHERE id = $1`, submissionID).Scan(&stale); err != nil {
		t.Fatalf("read stale_grade for submission %d: %v", submissionID, err)
	}
	if stale != want {
		t.Fatalf("submission %d stale_grade = %t; want %t", submissionID, stale, want)
	}
}

func TestAssignmentValidationAndProtectedDelete(t *testing.T) {
	env := newTestEnvironment(t)
	_, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	student, _ := createCourseTestUser(t, env, "Student", "student@example.test", auth.RoleStudent)
	_, otherLecturerAccess := createCourseTestUser(t, env, "Other", "other@example.test", auth.RoleLecturer)

	courseResponse, course := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Algorithms", "code": "CS101", "term": "2026",
	}, lecturerAccess)
	if courseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create course = %d, %#v", courseResponse.StatusCode, course)
	}
	courseID := int64Field(t, course, "id")
	coursePath := fmt.Sprintf("/api/v1/courses/%d", courseID)
	assignmentBody := map[string]string{
		"title": "Sorting", "description": "Sort the input.",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
	}
	created, assignment := env.request(t, http.MethodPost, coursePath+"/assignments", assignmentBody, lecturerAccess)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create assignment = %d, %#v", created.StatusCode, assignment)
	}
	assignmentID := int64Field(t, assignment, "id")
	assignmentPath := fmt.Sprintf("/api/v1/assignments/%d", assignmentID)

	for _, request := range []struct {
		name string
		body map[string]string
	}{
		{"late window before deadline", map[string]string{
			"title": "Invalid", "deadline_at": "2026-10-21T12:00:00Z",
			"late_until_at": "2026-10-20T12:00:00Z",
		}},
		{"malformed deadline", map[string]string{
			"title": "Invalid", "deadline_at": "tomorrow",
			"late_until_at": "2026-10-21T12:00:00Z",
		}},
	} {
		t.Run(request.name, func(t *testing.T) {
			response, payload := env.request(t, http.MethodPost, coursePath+"/assignments", request.body, lecturerAccess)
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("invalid assignment status = %d, %#v; want 400", response.StatusCode, payload)
			}
			assertError(t, payload, "invalid_assignment")
		})
	}

	invalidLimits := []struct {
		name string
		body map[string]int
	}{
		{"tmpfs exceeds memory allowance", map[string]int{
			"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
			"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 49,
		}},
		{"output exceeds tmpfs allowance", map[string]int{
			"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
			"timeout_s": 5, "max_output_bytes": 16 << 20, "tmpfs_size_mb": 32,
		}},
	}
	for _, request := range invalidLimits {
		t.Run(request.name, func(t *testing.T) {
			response, payload := env.request(t, http.MethodPut, assignmentPath+"/resource-limit", request.body, lecturerAccess)
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("invalid resource limits status = %d, %#v; want 400", response.StatusCode, payload)
			}
			assertError(t, payload, "invalid_resource_limit")
		})
	}

	studentEnrollment, _ := env.request(t, http.MethodPost, coursePath+"/enrollments", map[string]string{
		"email": student.Email,
	}, lecturerAccess)
	if studentEnrollment.StatusCode != http.StatusNoContent {
		t.Fatalf("enroll student = %d; want 204", studentEnrollment.StatusCode)
	}
	submissionInsert, err := env.pool.Exec(t.Context(), `
		INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes)
		VALUES ($1, $2, 'test/path', 'solution.py', 1)
	`, student.ID, assignmentID)
	if err != nil || submissionInsert.RowsAffected() != 1 {
		t.Fatalf("insert submission for protected delete: rows=%d err=%v", submissionInsert.RowsAffected(), err)
	}
	deleted, deletePayload := env.request(t, http.MethodDelete, assignmentPath, nil, lecturerAccess)
	if deleted.StatusCode != http.StatusConflict {
		t.Fatalf("delete assignment with submission = %d, %#v; want 409", deleted.StatusCode, deletePayload)
	}
	assertError(t, deletePayload, "conflict")

	otherLecturerWrite, otherPayload := env.request(t, http.MethodPut, assignmentPath, assignmentBody, otherLecturerAccess)
	if otherLecturerWrite.StatusCode != http.StatusNotFound {
		t.Fatalf("other lecturer update = %d, %#v; want 404", otherLecturerWrite.StatusCode, otherPayload)
	}
	assertError(t, otherPayload, "not_found")
}
