package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
)

func TestCourseAndEnrollmentEndpoints(t *testing.T) {
	env := newTestEnvironment(t)
	lecturer, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	_, otherLecturerAccess := createCourseTestUser(t, env, "Other lecturer", "other@example.test", auth.RoleLecturer)
	student, studentAccess := createCourseTestUser(t, env, "Student", "student@example.test", auth.RoleStudent)
	otherStudent, otherStudentAccess := createCourseTestUser(t, env, "Other student", "other-student@example.test", auth.RoleStudent)
	_, adminAccess := createCourseTestUser(t, env, "Admin", "admin@example.test", auth.RoleAdmin)

	created, coursePayload := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "  Programming  ", "code": "  CS101  ", "term": "  2026-Fall  ",
	}, lecturerAccess)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create course status = %d, body = %#v", created.StatusCode, coursePayload)
	}
	courseID := int64Field(t, coursePayload, "id")
	if coursePayload["name"] != "Programming" || coursePayload["code"] != "CS101" ||
		coursePayload["term"] != "2026-Fall" || int64Field(t, coursePayload, "lecturer_id") != lecturer.ID {
		t.Fatalf("created course response = %#v", coursePayload)
	}

	adminCreated, adminCourse := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Admin course", "code": "ADMIN101", "term": "2026", "lecturer_email": lecturer.Email,
	}, adminAccess)
	if adminCreated.StatusCode != http.StatusCreated || int64Field(t, adminCourse, "lecturer_id") != lecturer.ID {
		t.Fatalf("admin create course status = %d, body = %#v", adminCreated.StatusCode, adminCourse)
	}

	studentCourses, studentCoursesPayload := env.request(t, http.MethodGet, "/api/v1/courses?page=1&page_size=10", nil, studentAccess)
	if studentCourses.StatusCode != http.StatusOK || len(pageItems(t, studentCoursesPayload)) != 0 {
		t.Fatalf("unenrolled student courses = %d, %#v", studentCourses.StatusCode, studentCoursesPayload)
	}
	lecturerCourses, lecturerCoursesPayload := env.request(t, http.MethodGet, "/api/v1/courses", nil, lecturerAccess)
	if lecturerCourses.StatusCode != http.StatusOK || len(pageItems(t, lecturerCoursesPayload)) != 2 {
		t.Fatalf("lecturer course list = %d, %#v", lecturerCourses.StatusCode, lecturerCoursesPayload)
	}
	otherLecturerCourses, otherLecturerCoursesPayload := env.request(t, http.MethodGet, "/api/v1/courses", nil, otherLecturerAccess)
	if otherLecturerCourses.StatusCode != http.StatusOK || len(pageItems(t, otherLecturerCoursesPayload)) != 0 {
		t.Fatalf("other lecturer course list = %d, %#v", otherLecturerCourses.StatusCode, otherLecturerCoursesPayload)
	}
	adminCourses, adminCoursesPayload := env.request(t, http.MethodGet, "/api/v1/courses?page=1&page_size=1", nil, adminAccess)
	if adminCourses.StatusCode != http.StatusOK || adminCoursesPayload["has_next"] != true ||
		adminCoursesPayload["page"] != float64(1) || adminCoursesPayload["page_size"] != float64(1) {
		t.Fatalf("admin paginated courses = %d, %#v", adminCourses.StatusCode, adminCoursesPayload)
	}

	coursePath := "/api/v1/courses/" + strconv.FormatInt(courseID, 10)
	enrollmentPath := coursePath + "/enrollments"
	assignmentsPath := coursePath + "/assignments"
	enrollments, enrollmentsPayload := env.request(t, http.MethodGet, enrollmentPath, nil, lecturerAccess)
	if enrollments.StatusCode != http.StatusOK || len(pageItems(t, enrollmentsPayload)) != 0 {
		t.Fatalf("empty enrollments = %d, %#v", enrollments.StatusCode, enrollmentsPayload)
	}

	unknownEmail, unknownEmailPayload := env.request(t, http.MethodPost, enrollmentPath, map[string]string{
		"email": "missing@example.test",
	}, lecturerAccess)
	if unknownEmail.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown enrollment email status = %d, body = %#v", unknownEmail.StatusCode, unknownEmailPayload)
	}
	assertError(t, unknownEmailPayload, "user_not_found")

	enrollLecturer, enrollLecturerPayload := env.request(t, http.MethodPost, enrollmentPath, map[string]string{
		"email": lecturer.Email,
	}, lecturerAccess)
	if enrollLecturer.StatusCode != http.StatusBadRequest {
		t.Fatalf("enroll lecturer status = %d, body = %#v", enrollLecturer.StatusCode, enrollLecturerPayload)
	}
	assertError(t, enrollLecturerPayload, "invalid_enrollment_user")

	added, _ := env.request(t, http.MethodPost, enrollmentPath, map[string]string{"email": student.Email}, lecturerAccess)
	if added.StatusCode != http.StatusNoContent {
		t.Fatalf("add enrollment status = %d; want 204", added.StatusCode)
	}
	for _, request := range []struct {
		name   string
		method string
		path   string
		body   any
		token  string
	}{
		{"other lecturer GET", http.MethodGet, enrollmentPath, nil, otherLecturerAccess},
		{"other lecturer POST", http.MethodPost, enrollmentPath, map[string]string{"email": otherStudent.Email}, otherLecturerAccess},
		{"other lecturer DELETE", http.MethodDelete, enrollmentPath + "/" + strconv.FormatInt(student.ID, 10), nil, otherLecturerAccess},
		{"student GET", http.MethodGet, enrollmentPath, nil, studentAccess},
		{"student POST", http.MethodPost, enrollmentPath, map[string]string{"email": otherStudent.Email}, studentAccess},
		{"student DELETE", http.MethodDelete, enrollmentPath + "/" + strconv.FormatInt(student.ID, 10), nil, studentAccess},
	} {
		response, payload := env.request(t, request.method, request.path, request.body, request.token)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s enrollment access = %d, %#v; want 404", request.name, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}
	duplicate, duplicatePayload := env.request(t, http.MethodPost, enrollmentPath, map[string]string{
		"email": student.Email,
	}, lecturerAccess)
	if duplicate.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate enrollment status = %d, body = %#v", duplicate.StatusCode, duplicatePayload)
	}
	assertError(t, duplicatePayload, "enrollment_exists")

	enrolled, enrolledPayload := env.request(t, http.MethodGet, enrollmentPath+"?page=1&page_size=10", nil, lecturerAccess)
	items := pageItems(t, enrolledPayload)
	if enrolled.StatusCode != http.StatusOK || len(items) != 1 {
		t.Fatalf("enrollment list = %d, %#v", enrolled.StatusCode, enrolledPayload)
	}
	enrollment, ok := items[0].(map[string]any)
	if !ok || enrollment["user_id"] != float64(student.ID) || enrollment["email"] != student.Email {
		t.Fatalf("listed enrollment = %#v", items[0])
	}

	studentCourses, studentCoursesPayload = env.request(t, http.MethodGet, "/api/v1/courses", nil, studentAccess)
	items = pageItems(t, studentCoursesPayload)
	if studentCourses.StatusCode != http.StatusOK || len(items) != 1 {
		t.Fatalf("enrolled student courses = %d, %#v", studentCourses.StatusCode, studentCoursesPayload)
	}
	course, ok := items[0].(map[string]any)
	if !ok || int64Field(t, course, "id") != courseID {
		t.Fatalf("student course = %#v", items[0])
	}
	assignments, assignmentsPayload := env.request(t, http.MethodGet, assignmentsPath, nil, studentAccess)
	if assignments.StatusCode != http.StatusOK || len(pageItems(t, assignmentsPayload)) != 0 ||
		assignmentsPayload["has_next"] != false {
		t.Fatalf("empty course assignments = %d, %#v", assignments.StatusCode, assignmentsPayload)
	}
	ownerAssignments, ownerAssignmentsPayload := env.request(t, http.MethodGet, assignmentsPath, nil, lecturerAccess)
	if ownerAssignments.StatusCode != http.StatusOK || len(pageItems(t, ownerAssignmentsPayload)) != 0 {
		t.Fatalf("course owner assignments = %d, %#v", ownerAssignments.StatusCode, ownerAssignmentsPayload)
	}
	adminAssignments, adminAssignmentsPayload := env.request(t, http.MethodGet, assignmentsPath, nil, adminAccess)
	if adminAssignments.StatusCode != http.StatusOK || len(pageItems(t, adminAssignmentsPayload)) != 0 {
		t.Fatalf("admin course assignments = %d, %#v", adminAssignments.StatusCode, adminAssignmentsPayload)
	}

	for _, request := range []struct {
		path string
	}{
		{enrollmentPath},
		{assignmentsPath},
	} {
		response, payload := env.request(t, http.MethodGet, request.path, nil, otherStudentAccess)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("unenrolled student GET %s = %d, body = %#v; want 404", request.path, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}
	for _, request := range []struct {
		path string
	}{
		{enrollmentPath},
		{assignmentsPath},
	} {
		response, payload := env.request(t, http.MethodGet, request.path, nil, otherLecturerAccess)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("other lecturer GET %s = %d, body = %#v; want 404", request.path, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}

	deleted, _ := env.request(t, http.MethodDelete,
		enrollmentPath+"/"+strconv.FormatInt(student.ID, 10), nil, lecturerAccess)
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("delete enrollment status = %d; want 204", deleted.StatusCode)
	}
	studentCourses, studentCoursesPayload = env.request(t, http.MethodGet, "/api/v1/courses", nil, studentAccess)
	if studentCourses.StatusCode != http.StatusOK || len(pageItems(t, studentCoursesPayload)) != 0 {
		t.Fatalf("student courses after unenrollment = %d, %#v", studentCourses.StatusCode, studentCoursesPayload)
	}

	adminAdded, _ := env.request(t, http.MethodPost, enrollmentPath, map[string]string{"email": student.Email}, adminAccess)
	if adminAdded.StatusCode != http.StatusNoContent {
		t.Fatalf("admin add enrollment status = %d; want 204", adminAdded.StatusCode)
	}
	adminDeleted, _ := env.request(t, http.MethodDelete,
		enrollmentPath+"/"+strconv.FormatInt(student.ID, 10), nil, adminAccess)
	if adminDeleted.StatusCode != http.StatusNoContent {
		t.Fatalf("admin delete enrollment status = %d; want 204", adminDeleted.StatusCode)
	}

	demoted, demotedPayload := env.request(t, http.MethodPut,
		"/api/v1/admin/users/"+strconv.FormatInt(lecturer.ID, 10)+"/role",
		map[string]string{"role": auth.RoleStudent}, adminAccess)
	if demoted.StatusCode != http.StatusOK || demotedPayload["role"] != auth.RoleStudent {
		t.Fatalf("demote course owner = %d, %#v", demoted.StatusCode, demotedPayload)
	}
	demotedCourses, demotedCoursesPayload := env.request(t, http.MethodGet, "/api/v1/courses", nil, lecturerAccess)
	if demotedCourses.StatusCode != http.StatusOK || len(pageItems(t, demotedCoursesPayload)) != 0 {
		t.Fatalf("demoted owner course list = %d, %#v; want empty", demotedCourses.StatusCode, demotedCoursesPayload)
	}
	for _, request := range []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"demoted owner list", http.MethodGet, enrollmentPath, nil},
		{"demoted owner add", http.MethodPost, enrollmentPath, map[string]string{"email": student.Email}},
		{"demoted owner remove", http.MethodDelete, enrollmentPath + "/" + strconv.FormatInt(student.ID, 10), nil},
		{"demoted owner assignments", http.MethodGet, assignmentsPath, nil},
	} {
		response, payload := env.request(t, request.method, request.path, request.body, lecturerAccess)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s = %d, %#v; want 404", request.name, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}
}

func TestCourseEndpointValidationAndNotFoundResponses(t *testing.T) {
	env := newTestEnvironment(t)
	lecturer, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	otherLecturer, _ := createCourseTestUser(t, env, "Other lecturer", "other@example.test", auth.RoleLecturer)
	student, studentAccess := createCourseTestUser(t, env, "Student", "student@example.test", auth.RoleStudent)
	_, adminAccess := createCourseTestUser(t, env, "Admin", "admin@example.test", auth.RoleAdmin)

	for _, request := range []struct {
		name string
		body map[string]string
	}{
		{"missing code", map[string]string{"name": "Course", "term": "2026"}},
		{"empty code", map[string]string{"name": "Course", "code": "  ", "term": "2026"}},
		{"missing term", map[string]string{"name": "Course", "code": "CS101"}},
		{"empty name", map[string]string{"name": " ", "code": "CS101", "term": "2026"}},
		{"long name", map[string]string{"name": strings.Repeat("n", 201), "code": "CS101", "term": "2026"}},
		{"long code", map[string]string{"name": "Course", "code": strings.Repeat("c", 33), "term": "2026"}},
		{"long term", map[string]string{"name": "Course", "code": "CS101", "term": strings.Repeat("t", 21)}},
	} {
		t.Run(request.name, func(t *testing.T) {
			response, payload := env.request(t, http.MethodPost, "/api/v1/courses", request.body, lecturerAccess)
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("invalid course status = %d, body = %#v; want 400", response.StatusCode, payload)
			}
			assertError(t, payload, "invalid_request")
			message := errorMessage(t, payload)
			if message == "" || strings.Contains(message, "required") {
				t.Fatalf("validation message must be non-empty Indonesian text, got %q", message)
			}
		})
	}
	lecturerAssignOther, lecturerAssignOtherPayload := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Course", "code": "CS102", "term": "2026", "lecturer_email": otherLecturer.Email,
	}, lecturerAccess)
	if lecturerAssignOther.StatusCode != http.StatusBadRequest {
		t.Fatalf("lecturer creating for another user = %d, %#v; want 400", lecturerAssignOther.StatusCode, lecturerAssignOtherPayload)
	}
	assertError(t, lecturerAssignOtherPayload, "invalid_request")
	ownEmailAccepted, ownEmailPayload := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Course", "code": "CS103", "term": "2026", "lecturer_email": lecturer.Email,
	}, lecturerAccess)
	if ownEmailAccepted.StatusCode != http.StatusCreated || int64Field(t, ownEmailPayload, "lecturer_id") != lecturer.ID {
		t.Fatalf("lecturer creating for self = %d, %#v; want own lecturer_id", ownEmailAccepted.StatusCode, ownEmailPayload)
	}

	for _, request := range []struct {
		name string
		body map[string]string
		want int
		code string
	}{
		{"admin missing lecturer_email", map[string]string{
			"name": "Admin course", "code": "ADMIN101", "term": "2026",
		}, http.StatusBadRequest, "invalid_request"},
		{"admin lecturer_email is student", map[string]string{
			"name": "Admin course", "code": "ADMIN102", "term": "2026", "lecturer_email": student.Email,
		}, http.StatusBadRequest, "invalid_request"},
		{"admin lecturer_email unknown", map[string]string{
			"name": "Admin course", "code": "ADMIN103", "term": "2026", "lecturer_email": "missing@example.test",
		}, http.StatusNotFound, "user_not_found"},
	} {
		t.Run(request.name, func(t *testing.T) {
			response, payload := env.request(t, http.MethodPost, "/api/v1/courses", request.body, adminAccess)
			if response.StatusCode != request.want {
				t.Fatalf("admin create status = %d, %#v; want %d", response.StatusCode, payload, request.want)
			}
			assertError(t, payload, request.code)
		})
	}

	created, _ := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Course", "code": "CS101", "term": "2026",
	}, lecturerAccess)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create initial course status = %d", created.StatusCode)
	}
	duplicateCode, duplicateCodePayload := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Another", "code": "CS101", "term": "2027",
	}, lecturerAccess)
	if duplicateCode.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate code status = %d, body = %#v", duplicateCode.StatusCode, duplicateCodePayload)
	}
	assertError(t, duplicateCodePayload, "course_code_exists")

	studentCreate, studentCreatePayload := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Course", "code": "CS202", "term": "2026",
	}, studentAccess)
	if studentCreate.StatusCode != http.StatusForbidden {
		t.Fatalf("student create course status = %d, body = %#v; want 403", studentCreate.StatusCode, studentCreatePayload)
	}
	assertError(t, studentCreatePayload, "forbidden")

	invalidPage, invalidPagePayload := env.request(t, http.MethodGet, "/api/v1/courses?page_size=0", nil, studentAccess)
	if invalidPage.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid course page_size status = %d, body = %#v; want 400", invalidPage.StatusCode, invalidPagePayload)
	}
	assertError(t, invalidPagePayload, "invalid_pagination")

	unauthenticated, unauthenticatedPayload := env.request(t, http.MethodGet, "/api/v1/courses", nil, "")
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated course list status = %d; want 401", unauthenticated.StatusCode)
	}
	assertError(t, unauthenticatedPayload, "unauthorized")

	for _, path := range []string{
		"/api/v1/courses/not-a-number/assignments",
		"/api/v1/courses/not-a-number/enrollments",
		"/api/v1/courses/999999999/enrollments",
		"/api/v1/courses/1/enrollments/not-a-number",
	} {
		method := http.MethodGet
		if strings.Contains(path, "/enrollments/not-a-number") {
			method = http.MethodDelete
		}
		response, payload := env.request(t, method, path, nil, lecturerAccess)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %s status = %d; want 404, body = %#v", method, path, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}
}

func createCourseTestUser(t *testing.T, env *testEnvironment, name, email, role string) (auth.User, string) {
	t.Helper()
	user, err := env.store.CreateUser(context.Background(), name, email, "course-test-hash", role)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.NewTokens([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	access, err := tokens.IssueAccessToken(user)
	if err != nil {
		t.Fatal(err)
	}
	return user, access
}

func pageItems(t *testing.T, payload map[string]any) []any {
	t.Helper()
	items, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("page items have type %T: %#v", payload["items"], payload)
	}
	return items
}

func errorMessage(t *testing.T, payload map[string]any) string {
	t.Helper()
	errorObject, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error payload = %#v", payload)
	}
	message, _ := errorObject["message"].(string)
	return message
}

func TestCoursePaginationRejectsInvalidPageSizeForManagerEndpoints(t *testing.T) {
	env := newTestEnvironment(t)
	_, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	created, payload := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Course", "code": "CS101", "term": "2026",
	}, lecturerAccess)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create course status = %d, body = %#v", created.StatusCode, payload)
	}
	courseID := int64Field(t, payload, "id")
	for _, path := range []string{
		fmt.Sprintf("/api/v1/courses/%d/enrollments?page_size=0", courseID),
		fmt.Sprintf("/api/v1/courses/%d/assignments?page_size=0", courseID),
	} {
		response, responsePayload := env.request(t, http.MethodGet, path, nil, lecturerAccess)
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("GET %s status = %d; want 400, body = %#v", path, response.StatusCode, responsePayload)
		}
		assertError(t, responsePayload, "invalid_pagination")
	}
}
