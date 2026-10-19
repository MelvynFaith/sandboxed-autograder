package api

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/courses"
)

func TestAssignmentResourceLimitAndTestCaseLifecycle(t *testing.T) {
	env := newTestEnvironment(t)
	lecturer, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
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
	if int64Field(t, course, "lecturer_id") != lecturer.ID {
		t.Fatalf("course lecturer_id = %d; want lecturer %d", int64Field(t, course, "lecturer_id"), lecturer.ID)
	}
	coursePath := fmt.Sprintf("/api/v1/courses/%d", courseID)
	enrolled, _ := env.request(t, http.MethodPost, coursePath+"/enrollments", map[string]string{
		"email": student.Email,
	}, lecturerAccess)
	if enrolled.StatusCode != http.StatusNoContent {
		t.Fatalf("enroll student = %d; want 204", enrolled.StatusCode)
	}

	resourceLimit := map[string]int{
		"cpu_time_ms": 2000, "memory_mb": 64, "max_processes": 5,
		"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
	}
	assignmentResponse, assignment := env.request(t, http.MethodPost, coursePath+"/assignments", map[string]any{
		"title": "Sorting", "description": "Sort the input.",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
		"resource_limit": resourceLimit,
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

	getLimit, getLimitPayload := env.request(t, http.MethodGet, assignmentPath+"/resource-limit", nil, lecturerAccess)
	if getLimit.StatusCode != http.StatusOK || getLimitPayload["tmpfs_size_mb"] != float64(32) {
		t.Fatalf("get resource limit after create = %d, %#v", getLimit.StatusCode, getLimitPayload)
	}
	if int64Field(t, getLimitPayload, "assignment_id") != assignmentID || getLimitPayload["memory_mb"] != float64(64) {
		t.Fatalf("resource limit after create = %#v", getLimitPayload)
	}
	updatedLimit := map[string]int{
		"cpu_time_ms": 3000, "memory_mb": 80, "max_processes": 7,
		"timeout_s": 8, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
	}
	limitResponse, limit := env.request(t, http.MethodPut, assignmentPath+"/resource-limit", updatedLimit, lecturerAccess)
	if limitResponse.StatusCode != http.StatusOK {
		t.Fatalf("update resource limit = %d, %#v", limitResponse.StatusCode, limit)
	}
	if int64Field(t, limit, "assignment_id") != assignmentID || limit["memory_mb"] != float64(80) {
		t.Fatalf("resource limit response = %#v", limit)
	}
	getLimit, getLimitPayload = env.request(t, http.MethodGet, assignmentPath+"/resource-limit", nil, lecturerAccess)
	if getLimit.StatusCode != http.StatusOK || getLimitPayload["memory_mb"] != float64(80) {
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
	if studentWriteCase.StatusCode != http.StatusNotFound {
		t.Fatalf("student create test case = %d, %#v; want 404", studentWriteCase.StatusCode, studentWriteCasePayload)
	}
	assertError(t, studentWriteCasePayload, "not_found")
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
	assertNoHiddenTestCaseData(t, studentItems)
	if studentCases.StatusCode != http.StatusOK || len(studentItems) != 1 {
		t.Fatalf("student visible test cases = %d, %#v", studentCases.StatusCode, studentCasesPayload)
	}
	publicCase, ok := studentItems[0].(map[string]any)
	if !ok || publicCase["input"] != "1 2" || publicCase["expected_output"] != "2 1" || publicCase["is_hidden"] != false {
		t.Fatalf("student test case = %#v", studentItems[0])
	}
	studentAssignmentDetail, studentAssignmentDetailPayload := env.request(t, http.MethodGet, assignmentPath, nil, studentAccess)
	if studentAssignmentDetail.StatusCode != http.StatusOK {
		t.Fatalf("student assignment detail = %d, %#v", studentAssignmentDetail.StatusCode, studentAssignmentDetailPayload)
	}
	assertNoHiddenTestCaseData(t, studentAssignmentDetailPayload)

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
	var visibleRetired bool
	if err := env.pool.QueryRow(t.Context(), `SELECT is_deleted FROM test_cases WHERE id = $1`, testCaseID).Scan(&visibleRetired); err != nil {
		t.Fatalf("check soft-deleted test case without result: %v", err)
	}
	if !visibleRetired {
		t.Fatal("test case on assignment with submissions was not soft deleted")
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
		{"student write hidden", http.MethodPost, courseAssignmentPath, map[string]string{
			"title": "No", "description": "", "deadline_at": "2026-10-20T12:00:00Z",
			"late_until_at": "2026-10-21T12:00:00Z",
		}, studentAccess, http.StatusNotFound},
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

func assertNoHiddenTestCaseData(t *testing.T, value any) {
	t.Helper()
	switch item := value.(type) {
	case map[string]any:
		if item["is_hidden"] == true {
			t.Fatalf("hidden test case was exposed: %#v", item)
		}
		for key, child := range item {
			if text, ok := child.(string); ok &&
				(key == "external_id" && text == "hidden_case" ||
					key == "input" && text == "secret input" ||
					key == "expected_output" && text == "secret output") {
				t.Fatalf("hidden test case field %q was exposed: %#v", key, text)
			}
			assertNoHiddenTestCaseData(t, child)
		}
	case []any:
		for _, child := range item {
			assertNoHiddenTestCaseData(t, child)
		}
	}
}

func TestCreateAssignmentRollsBackWhenResourceLimitInsertFails(t *testing.T) {
	env := newTestEnvironment(t)
	_, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	courseResponse, course := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Algorithms", "code": "CS101", "term": "2026",
	}, lecturerAccess)
	if courseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create course = %d, %#v", courseResponse.StatusCode, course)
	}
	courseID := int64Field(t, course, "id")
	_, err := courses.NewStore(env.pool).CreateAssignment(
		t.Context(), courseID, "Invalid resource limit", "",
		time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC),
		courses.ResourceLimit{
			CPUTimeMS: 1000, MemoryMB: 64, MaxProcesses: 5, TimeoutS: 5,
			MaxOutputBytes: 1 << 20, TmpfsSizeMB: 49,
		},
	)
	if err == nil {
		t.Fatal("create assignment with database-invalid resource limit succeeded")
	}
	var assignmentCount int
	if err := env.pool.QueryRow(t.Context(), `SELECT count(*) FROM assignments WHERE course_id = $1`, courseID).Scan(&assignmentCount); err != nil {
		t.Fatalf("count assignments after failed create: %v", err)
	}
	if assignmentCount != 0 {
		t.Fatalf("failed create left %d partial assignment rows; want 0", assignmentCount)
	}
}

func TestDeleteTestCaseDuringRunningSubmissionSoftDeletes(t *testing.T) {
	env := newTestEnvironment(t)
	lecturer, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	student, _ := createCourseTestUser(t, env, "Student", "student@example.test", auth.RoleStudent)
	courseResponse, course := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Algorithms", "code": "CS101", "term": "2026",
	}, lecturerAccess)
	if courseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create course = %d, %#v", courseResponse.StatusCode, course)
	}
	if int64Field(t, course, "lecturer_id") != lecturer.ID {
		t.Fatalf("course lecturer_id = %d; want lecturer %d", int64Field(t, course, "lecturer_id"), lecturer.ID)
	}
	coursePath := fmt.Sprintf("/api/v1/courses/%d", int64Field(t, course, "id"))
	limit := map[string]int{
		"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
		"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
	}
	assignmentResponse, assignment := env.request(t, http.MethodPost, coursePath+"/assignments", map[string]any{
		"title": "Running test", "description": "",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
		"resource_limit": limit,
	}, lecturerAccess)
	if assignmentResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create assignment = %d, %#v", assignmentResponse.StatusCode, assignment)
	}
	assignmentID := int64Field(t, assignment, "id")
	var submissionID int64
	err := env.pool.QueryRow(t.Context(), `
		INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes, status)
		VALUES ($1, $2, 'test/path/running', 'solution.py', 1, 'RUNNING')
		RETURNING id
	`, student.ID, assignmentID).Scan(&submissionID)
	if err != nil {
		t.Fatalf("insert running submission: %v", err)
	}
	caseResponse, testCase := env.request(t, http.MethodPost,
		fmt.Sprintf("/api/v1/assignments/%d/testcases", assignmentID),
		map[string]any{
			"external_id": "running_case", "input": "1", "expected_output": "1",
			"weight": 1, "is_hidden": false, "match_mode": "EXACT",
		}, lecturerAccess)
	if caseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create running test case = %d, %#v", caseResponse.StatusCode, testCase)
	}
	testCaseID := int64Field(t, testCase, "id")
	deleted, payload := env.request(t, http.MethodDelete,
		fmt.Sprintf("/api/v1/testcases/%d", testCaseID), nil, lecturerAccess)
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("delete running test case = %d, %#v", deleted.StatusCode, payload)
	}
	var isDeleted bool
	if err := env.pool.QueryRow(t.Context(), `SELECT is_deleted FROM test_cases WHERE id = $1`, testCaseID).Scan(&isDeleted); err != nil {
		t.Fatalf("read running test case after delete: %v", err)
	}
	if !isDeleted {
		t.Fatal("test case for a running submission was not soft deleted")
	}
	assertSubmissionStale(t, env, submissionID, true)
	if _, err := env.pool.Exec(t.Context(), `
		INSERT INTO results (submission_id, testcase_id, status, exec_time_ms, memory_kb, memory_source, score)
		VALUES ($1, $2, 'PASSED', 1, 1, 'CGROUP_PEAK', 1)
	`, submissionID, testCaseID); err != nil {
		t.Fatalf("insert result after running test case delete: %v", err)
	}

	assignmentResponse, assignment = env.request(t, http.MethodPost, coursePath+"/assignments", map[string]any{
		"title": "No submissions", "description": "",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
		"resource_limit": limit,
	}, lecturerAccess)
	if assignmentResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create assignment without submissions = %d, %#v", assignmentResponse.StatusCode, assignment)
	}
	assignmentID = int64Field(t, assignment, "id")
	caseResponse, testCase = env.request(t, http.MethodPost,
		fmt.Sprintf("/api/v1/assignments/%d/testcases", assignmentID),
		map[string]any{
			"external_id": "unused_case", "input": "2", "expected_output": "2",
			"weight": 1, "is_hidden": false, "match_mode": "EXACT",
		}, lecturerAccess)
	if caseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create test case without submissions = %d, %#v", caseResponse.StatusCode, testCase)
	}
	testCaseID = int64Field(t, testCase, "id")
	deleted, payload = env.request(t, http.MethodDelete,
		fmt.Sprintf("/api/v1/testcases/%d", testCaseID), nil, lecturerAccess)
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("delete test case without submissions = %d, %#v", deleted.StatusCode, payload)
	}
	var stillExists bool
	if err := env.pool.QueryRow(t.Context(), `SELECT EXISTS (SELECT 1 FROM test_cases WHERE id = $1)`, testCaseID).Scan(&stillExists); err != nil {
		t.Fatalf("check hard-deleted test case: %v", err)
	}
	if stillExists {
		t.Fatal("test case without submissions was not physically deleted")
	}
}

func TestAssignmentValidationAndProtectedDelete(t *testing.T) {
	env := newTestEnvironment(t)
	lecturer, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	student, studentAccess := createCourseTestUser(t, env, "Student", "student@example.test", auth.RoleStudent)
	_, otherLecturerAccess := createCourseTestUser(t, env, "Other", "other@example.test", auth.RoleLecturer)

	courseResponse, course := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Algorithms", "code": "CS101", "term": "2026",
	}, lecturerAccess)
	if courseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create course = %d, %#v", courseResponse.StatusCode, course)
	}
	courseID := int64Field(t, course, "id")
	coursePath := fmt.Sprintf("/api/v1/courses/%d", courseID)
	resourceLimit := map[string]int{
		"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
		"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
	}
	assignmentBody := map[string]any{
		"title": "Sorting", "description": "Sort the input.",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
		"resource_limit": resourceLimit,
	}
	created, assignment := env.request(t, http.MethodPost, coursePath+"/assignments", assignmentBody, lecturerAccess)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create assignment = %d, %#v", created.StatusCode, assignment)
	}
	if int64Field(t, course, "lecturer_id") != lecturer.ID {
		t.Fatalf("course lecturer_id = %d; want lecturer %d", int64Field(t, course, "lecturer_id"), lecturer.ID)
	}
	assignmentID := int64Field(t, assignment, "id")
	assignmentPath := fmt.Sprintf("/api/v1/assignments/%d", assignmentID)
	caseResponse, testCase := env.request(t, http.MethodPost, assignmentPath+"/testcases", map[string]any{
		"external_id": "tc_01", "input": "1", "expected_output": "1",
		"weight": 1, "is_hidden": false, "match_mode": "EXACT",
	}, lecturerAccess)
	if caseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create test case = %d, %#v", caseResponse.StatusCode, testCase)
	}
	testCaseID := int64Field(t, testCase, "id")
	testCasePath := fmt.Sprintf("/api/v1/testcases/%d", testCaseID)

	for _, request := range []struct {
		name string
		body map[string]any
	}{
		{"late window before deadline", map[string]any{
			"title": "Invalid", "deadline_at": "2026-10-21T12:00:00Z",
			"late_until_at": "2026-10-20T12:00:00Z",
		}},
		{"malformed deadline", map[string]any{
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

	missingCreateLimit, missingCreateLimitPayload := env.request(t, http.MethodPost, coursePath+"/assignments", map[string]any{
		"title": "Missing limit", "description": "No resource limit.",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
	}, lecturerAccess)
	if missingCreateLimit.StatusCode != http.StatusBadRequest {
		t.Fatalf("create assignment without resource limit = %d, %#v; want 400", missingCreateLimit.StatusCode, missingCreateLimitPayload)
	}
	assertError(t, missingCreateLimitPayload, "invalid_request")

	for _, request := range []struct {
		name  string
		limit map[string]int
	}{
		{"tmpfs exceeds memory allowance", map[string]int{
			"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
			"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 49,
		}},
		{"output exceeds tmpfs allowance", map[string]int{
			"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
			"timeout_s": 5, "max_output_bytes": 16 << 20, "tmpfs_size_mb": 32,
		}},
	} {
		t.Run("create invalid resource limit/"+request.name, func(t *testing.T) {
			body := map[string]any{
				"title": "Invalid limit", "description": "Invalid resource limit.",
				"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
				"resource_limit": request.limit,
			}
			response, payload := env.request(t, http.MethodPost, coursePath+"/assignments", body, lecturerAccess)
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("create assignment with invalid resource limit = %d, %#v; want 400", response.StatusCode, payload)
			}
			assertError(t, payload, "invalid_request")
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

	for _, actor := range []struct {
		name  string
		token string
	}{
		{"other lecturer", otherLecturerAccess},
		{"student", studentAccess},
	} {
		for _, request := range []struct {
			name   string
			method string
			path   string
			body   any
		}{
			{"create assignment", http.MethodPost, coursePath + "/assignments", assignmentBody},
			{"update assignment", http.MethodPut, assignmentPath, assignmentBody},
			{"delete assignment", http.MethodDelete, assignmentPath, nil},
			{"update resource limit", http.MethodPut, assignmentPath + "/resource-limit", resourceLimit},
			{"create test case", http.MethodPost, assignmentPath + "/testcases", map[string]any{
				"input": "2", "expected_output": "2", "weight": 1, "is_hidden": false, "match_mode": "EXACT",
			}},
			{"update test case", http.MethodPut, testCasePath, map[string]any{
				"external_id": "tc_02", "input": "2", "expected_output": "2",
				"weight": 1, "is_hidden": false, "match_mode": "EXACT",
			}},
			{"delete test case", http.MethodDelete, testCasePath, nil},
		} {
			response, payload := env.request(t, request.method, request.path, request.body, actor.token)
			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("%s %s = %d, %#v; want 404", actor.name+" "+request.name, request.path, response.StatusCode, payload)
			}
			assertError(t, payload, "not_found")
		}
	}
}

func TestDemotedAssignmentOwnerCannotWrite(t *testing.T) {
	env := newTestEnvironment(t)
	lecturer, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	_, adminAccess := createCourseTestUser(t, env, "Admin", "admin@example.test", auth.RoleAdmin)

	courseResponse, course := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Algorithms", "code": "CS101", "term": "2026",
	}, lecturerAccess)
	if courseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create course = %d, %#v", courseResponse.StatusCode, course)
	}
	courseID := int64Field(t, course, "id")
	if int64Field(t, course, "lecturer_id") != lecturer.ID {
		t.Fatalf("course lecturer_id = %d; want lecturer %d", int64Field(t, course, "lecturer_id"), lecturer.ID)
	}
	coursePath := fmt.Sprintf("/api/v1/courses/%d", courseID)
	limitBody := map[string]int{
		"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
		"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
	}
	assignmentBody := map[string]any{
		"title": "Sorting", "description": "Sort the input.",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
		"resource_limit": limitBody,
	}
	created, assignment := env.request(t, http.MethodPost, coursePath+"/assignments", assignmentBody, lecturerAccess)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create assignment = %d, %#v", created.StatusCode, assignment)
	}
	assignmentID := int64Field(t, assignment, "id")
	assignmentPath := fmt.Sprintf("/api/v1/assignments/%d", assignmentID)
	caseBody := map[string]any{
		"external_id": "tc_01", "input": "1", "expected_output": "1",
		"weight": 1, "is_hidden": false, "match_mode": "EXACT",
	}
	caseResponse, testCase := env.request(t, http.MethodPost, assignmentPath+"/testcases", caseBody, lecturerAccess)
	if caseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create test case = %d, %#v", caseResponse.StatusCode, testCase)
	}
	testCasePath := fmt.Sprintf("/api/v1/testcases/%d", int64Field(t, testCase, "id"))

	demoted, demotedPayload := env.request(t, http.MethodPut,
		"/api/v1/admin/users/"+strconv.FormatInt(lecturer.ID, 10)+"/role",
		map[string]string{"role": auth.RoleStudent}, adminAccess)
	if demoted.StatusCode != http.StatusOK || demotedPayload["role"] != auth.RoleStudent {
		t.Fatalf("demote assignment owner = %d, %#v", demoted.StatusCode, demotedPayload)
	}

	for _, request := range []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"update assignment", http.MethodPut, assignmentPath, assignmentBody},
		{"create assignment", http.MethodPost, coursePath + "/assignments", assignmentBody},
		{"delete assignment", http.MethodDelete, assignmentPath, nil},
		{"update resource limit", http.MethodPut, assignmentPath + "/resource-limit", limitBody},
		{"create test case", http.MethodPost, assignmentPath + "/testcases", caseBody},
		{"update test case", http.MethodPut, testCasePath, caseBody},
		{"delete test case", http.MethodDelete, testCasePath, nil},
	} {
		response, payload := env.request(t, request.method, request.path, request.body, lecturerAccess)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("demoted owner %s = %d, %#v; want 404", request.name, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}
}
