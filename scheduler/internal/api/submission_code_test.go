package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
)

const codeTestSource = "print('hello ünïcode')\n"

// uploadCodeSubmission stores one submission of the fixture's student and returns its code URL.
func uploadCodeSubmission(t *testing.T, env *testEnvironment, fixture submissionFixture) (int64, string) {
	t.Helper()
	response, payload := env.upload(t, fixture.assignmentID, "hello.py", []byte(codeTestSource), fixture.studentAccess)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("upload code submission = %d, %#v", response.StatusCode, payload)
	}
	id := int64Field(t, payload, "id")
	return id, fmt.Sprintf("/api/v1/submissions/%d/code", id)
}

func TestSubmissionCodeAccess(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	submissionID, path := uploadCodeSubmission(t, env, fixture)

	for name, token := range map[string]string{
		"owner":           fixture.studentAccess,
		"course lecturer": fixture.lecturerAccess,
		"admin":           fixture.adminAccess,
	} {
		response, payload := env.request(t, http.MethodGet, path, nil, token)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s code = %d, %#v; want 200", name, response.StatusCode, payload)
		}
		if int64Field(t, payload, "submission_id") != submissionID ||
			payload["original_filename"] != "hello.py" || payload["code"] != codeTestSource ||
			len(payload) != 3 {
			t.Fatalf("%s code payload = %#v", name, payload)
		}
	}

	for name, token := range map[string]string{
		"other student":  fixture.otherStudentAuth,
		"other lecturer": fixture.otherLecturerAuth,
	} {
		response, payload := env.request(t, http.MethodGet, path, nil, token)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s code = %d, %#v; want 404", name, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
		if _, leaked := payload["code"]; leaked {
			t.Fatalf("%s got the code with the 404: %#v", name, payload)
		}
	}

	noToken, noTokenPayload := env.request(t, http.MethodGet, path, nil, "")
	if noToken.StatusCode != http.StatusUnauthorized {
		t.Fatalf("code without token = %d, %#v; want 401", noToken.StatusCode, noTokenPayload)
	}
	assertError(t, noTokenPayload, "unauthorized")
	for _, missing := range []string{"/api/v1/submissions/999999/code", "/api/v1/submissions/abc/code", "/api/v1/submissions/0/code"} {
		response, payload := env.request(t, http.MethodGet, missing, nil, fixture.adminAccess)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s = %d, %#v; want 404", missing, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}

	// Access follows the role the user has now: a course lecturer demoted to student loses the
	// code, and the owner promoted to lecturer of another course keeps their own.
	for _, change := range []struct {
		userID int64
		role   string
	}{{fixture.lecturer.ID, auth.RoleStudent}, {fixture.student.ID, auth.RoleLecturer}} {
		response, payload := env.request(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d/role", change.userID),
			map[string]string{"role": change.role}, fixture.adminAccess)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("change role of %d to %s = %d, %#v", change.userID, change.role, response.StatusCode, payload)
		}
	}
	demoted, demotedPayload := env.request(t, http.MethodGet, path, nil, fixture.lecturerAccess)
	if demoted.StatusCode != http.StatusNotFound {
		t.Fatalf("demoted course lecturer code = %d, %#v; want 404", demoted.StatusCode, demotedPayload)
	}
	assertError(t, demotedPayload, "not_found")
	promoted, promotedPayload := env.request(t, http.MethodGet, path, nil, fixture.studentAccess)
	if promoted.StatusCode != http.StatusOK || promotedPayload["code"] != codeTestSource {
		t.Fatalf("promoted owner's own code = %d, %#v; want 200", promoted.StatusCode, promotedPayload)
	}
}

func TestSubmissionCodeDeletedByRetention(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	submissionID, path := uploadCodeSubmission(t, env, fixture)
	if _, err := env.pool.Exec(t.Context(),
		`UPDATE submissions SET code_deleted_at = now() WHERE id = $1`, submissionID); err != nil {
		t.Fatalf("mark code deleted: %v", err)
	}

	for name, token := range map[string]string{
		"owner":           fixture.studentAccess,
		"course lecturer": fixture.lecturerAccess,
		"admin":           fixture.adminAccess,
	} {
		response, payload := env.request(t, http.MethodGet, path, nil, token)
		if response.StatusCode != http.StatusGone {
			t.Fatalf("%s code after retention = %d, %#v; want 410", name, response.StatusCode, payload)
		}
		assertError(t, payload, "code_deleted")
		if message, _ := payload["error"].(map[string]any)["message"].(string); message != "Berkas kode sudah dihapus karena masa retensi." {
			t.Fatalf("%s 410 message = %q", name, message)
		}
	}
	// 410 is only for those who may read the code: the others still get 404, not a hint that the
	// submission exists.
	for name, token := range map[string]string{
		"other student":  fixture.otherStudentAuth,
		"other lecturer": fixture.otherLecturerAuth,
	} {
		response, payload := env.request(t, http.MethodGet, path, nil, token)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s code after retention = %d, %#v; want 404", name, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}
}

func TestSubmissionCodeUnreadableFileIsServerFaultWithoutPaths(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	disableSubmissionRateLimits(t, env)

	// A file outside the code directory that a stored path must never reach.
	outside := filepath.Join(filepath.Dir(env.codeDir), "outside-secret.py")
	if err := os.WriteFile(outside, []byte("OUTSIDE-SECRET"), 0o600); err != nil {
		t.Fatalf("write file outside the code directory: %v", err)
	}
	// A file inside it that is larger than the upload limit cannot come from the API.
	oversized := "oversized.py"
	if err := os.WriteFile(filepath.Join(env.codeDir, oversized), []byte(strings.Repeat("x", maxSubmissionBytes+1)), 0o600); err != nil {
		t.Fatalf("write oversized file: %v", err)
	}

	for _, tc := range []struct {
		name     string
		codePath string
		hide     []string
	}{
		{name: "parent traversal", codePath: "../outside-secret.py", hide: []string{"outside-secret", "OUTSIDE-SECRET", ".."}},
		{name: "nested traversal", codePath: "a/../../outside-secret.py", hide: []string{"outside-secret", "OUTSIDE-SECRET"}},
		{name: "absolute path", codePath: outside, hide: []string{"outside-secret", "OUTSIDE-SECRET"}},
		{name: "parent directory itself", codePath: "..", hide: []string{".."}},
		{name: "file missing on disk", codePath: "gone.py", hide: []string{"gone.py", env.codeDir}},
		{name: "file larger than the upload limit", codePath: oversized, hide: []string{oversized, env.codeDir}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			submissionID, path := uploadCodeSubmission(t, env, fixture)
			if _, err := env.pool.Exec(t.Context(),
				`UPDATE submissions SET code_path = $2 WHERE id = $1`, submissionID, tc.codePath); err != nil {
				t.Fatalf("set stored code path: %v", err)
			}
			response, payload := env.request(t, http.MethodGet, path, nil, fixture.lecturerAccess)
			if response.StatusCode != http.StatusInternalServerError {
				t.Fatalf("code with stored path %q = %d, %#v; want 500", tc.codePath, response.StatusCode, payload)
			}
			assertError(t, payload, "internal_error")
			if _, leaked := payload["code"]; leaked {
				t.Fatalf("code returned for stored path %q: %#v", tc.codePath, payload)
			}
			body := fmt.Sprint(payload)
			for _, secret := range tc.hide {
				if strings.Contains(body, secret) {
					t.Fatalf("response for stored path %q leaks %q: %s", tc.codePath, secret, body)
				}
			}
			if strings.Contains(body, "no such file") || strings.Contains(body, "/") {
				t.Fatalf("response for stored path %q leaks an operating-system error or a path: %s", tc.codePath, body)
			}
		})
	}
}
