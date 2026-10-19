package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/courses"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/migrations"
	"github.com/golang-jwt/jwt/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testJWTSecret = "B1-test-secret-with-at-least-32-bytes"

type testEnvironment struct {
	server  *httptest.Server
	store   *auth.Store
	tickets *auth.TicketStore
	pool    *pgxpool.Pool
}

func newTestEnvironment(t *testing.T) *testEnvironment {
	t.Helper()
	baseURL := os.Getenv("AUTOGRADER_TEST_DATABASE_URL")
	if baseURL == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("AUTOGRADER_TEST_DATABASE_URL must be set in CI")
		}
		t.Skip("AUTOGRADER_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	admin, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	schema := fmt.Sprintf("api_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create isolated test schema: %v", err)
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	query := parsedURL.Query()
	query.Set("search_path", schema)
	parsedURL.RawQuery = query.Encode()

	poolConfig, err := pgxpool.ParseConfig(parsedURL.String())
	if err != nil {
		t.Fatalf("parse test pool configuration: %v", err)
	}
	poolConfig.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("create test connection pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping isolated test database: %v", err)
	}

	migrationURL := *parsedURL
	migrationURL.Scheme = "pgx5"
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("open migration source: %v", err)
	}
	migrator, err := migrate.NewWithSourceInstance("iofs", source, migrationURL.String())
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		_, _ = migrator.Close()
		pool.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	_, _ = migrator.Close()

	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Errorf("drop isolated test schema: %v", err)
		}
		if err := admin.Close(context.Background()); err != nil {
			t.Errorf("close test database connection: %v", err)
		}
	})

	tokens, err := auth.NewTokens([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	tickets, err := auth.NewTicketStore(auth.MaxTicketTTL)
	if err != nil {
		t.Fatal(err)
	}
	store := auth.NewStore(pool)
	server := httptest.NewServer(NewHandler(store, tokens, tickets, courses.NewStore(pool)))
	t.Cleanup(server.Close)
	return &testEnvironment{server: server, store: store, tickets: tickets, pool: pool}
}

func (e *testEnvironment) request(t *testing.T, method, path string, body any, token string) (*http.Response, map[string]any) {
	t.Helper()
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, e.server.URL+path, requestBody)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := e.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	payload := map[string]any{}
	if response.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatalf("decode %s %s response (status %d): %v", method, path, response.StatusCode, err)
		}
	}
	return response, payload
}

func (e *testEnvironment) register(t *testing.T, name, email, password string) map[string]any {
	t.Helper()
	response, payload := e.request(t, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"name": name, "email": email, "password": password,
	}, "")
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, body = %+v", response.StatusCode, payload)
	}
	return payload
}

func (e *testEnvironment) login(t *testing.T, email, password string) map[string]any {
	t.Helper()
	response, payload := e.request(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, body = %+v", response.StatusCode, payload)
	}
	return payload
}

func stringField(t *testing.T, object map[string]any, key string) string {
	t.Helper()
	value, ok := object[key].(string)
	if !ok || value == "" {
		t.Fatalf("field %q = %#v; want non-empty string", key, object[key])
	}
	return value
}

func int64Field(t *testing.T, object map[string]any, key string) int64 {
	t.Helper()
	number, ok := object[key].(float64)
	if !ok || number <= 0 {
		t.Fatalf("field %q = %#v; want positive number", key, object[key])
	}
	return int64(number)
}

func assertError(t *testing.T, payload map[string]any, expectedCode string) {
	t.Helper()
	envelope, ok := payload["error"].(map[string]any)
	if !ok || envelope["code"] != expectedCode || envelope["message"] == "" {
		t.Fatalf("error payload = %#v; want code %q and message", payload, expectedCode)
	}
}

func TestAuthSessionAndProfileLifecycle(t *testing.T) {
	env := newTestEnvironment(t)
	registered := env.register(t, "Student One", "STUDENT@example.test", "old-password")
	user := registered
	userID := int64Field(t, user, "id")
	if user["role"] != auth.RoleStudent || user["email"] != "student@example.test" {
		t.Fatalf("registration user = %#v; want normalized STUDENT", user)
	}

	duplicate, duplicatePayload := env.request(t, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"name": "Duplicate", "email": "student@example.test", "password": "another-password",
	}, "")
	if duplicate.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate registration status = %d; want 409", duplicate.StatusCode)
	}
	assertError(t, duplicatePayload, "email_already_exists")

	roleSelection, rolePayload := env.request(t, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"name": "Role Attempt", "email": "role@example.test", "password": "pass", "role": auth.RoleAdmin,
	}, "")
	if roleSelection.StatusCode != http.StatusBadRequest {
		t.Fatalf("registration with role field status = %d; want 400", roleSelection.StatusCode)
	}
	assertError(t, rolePayload, "invalid_request")

	tooLongName, tooLongNamePayload := env.request(t, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"name": strings.Repeat("n", 201), "email": "long-name@example.test", "password": "pass",
	}, "")
	if tooLongName.StatusCode != http.StatusBadRequest {
		t.Fatalf("201-character registration name status = %d; want 400", tooLongName.StatusCode)
	}
	assertError(t, tooLongNamePayload, "invalid_request")

	unknownLogin, unknownLoginPayload := env.request(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email": "missing@example.test", "password": "wrong",
	}, "")
	if unknownLogin.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown email login status = %d; want 401", unknownLogin.StatusCode)
	}
	assertError(t, unknownLoginPayload, "invalid_credentials")

	login := env.login(t, "student@example.test", "old-password")
	access := stringField(t, login, "access_token")
	refresh := stringField(t, login, "refresh_token")
	var storedHash string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT token_hash FROM refresh_tokens WHERE user_id = $1`, userID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash == refresh || storedHash != auth.HashRefreshToken(refresh) {
		t.Fatal("database did not store the refresh token only as its hash")
	}

	refreshed, refreshedPayload := env.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refresh,
	}, "")
	if refreshed.StatusCode != http.StatusOK {
		t.Fatalf("refresh status = %d; body = %+v", refreshed.StatusCode, refreshedPayload)
	}
	access = stringField(t, refreshedPayload, "access_token")
	refresh = stringField(t, refreshedPayload, "refresh_token")
	reusedRefresh, reusedPayload := env.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": stringField(t, login, "refresh_token"),
	}, "")
	if reusedRefresh.StatusCode != http.StatusUnauthorized {
		t.Fatalf("rotated refresh reuse status = %d; want 401", reusedRefresh.StatusCode)
	}
	assertError(t, reusedPayload, "invalid_refresh_token")

	profileResponse, profile := env.request(t, http.MethodGet, "/api/v1/users/me", nil, access)
	if profileResponse.StatusCode != http.StatusOK || profile["name"] != "Student One" {
		t.Fatalf("get profile response = %d, %#v", profileResponse.StatusCode, profile)
	}
	updatedName, updatedNamePayload := env.request(t, http.MethodPut, "/api/v1/users/me", map[string]string{
		"name": "Student Renamed",
	}, access)
	if updatedName.StatusCode != http.StatusOK || updatedNamePayload["name"] != "Student Renamed" {
		t.Fatalf("update profile = %d, %#v", updatedName.StatusCode, updatedNamePayload)
	}
	tooLongProfileName, tooLongProfileNamePayload := env.request(t, http.MethodPut, "/api/v1/users/me", map[string]string{
		"name": strings.Repeat("n", 201),
	}, access)
	if tooLongProfileName.StatusCode != http.StatusBadRequest {
		t.Fatalf("201-character profile name status = %d; want 400", tooLongProfileName.StatusCode)
	}
	assertError(t, tooLongProfileNamePayload, "invalid_request")

	wrongPassword, wrongPasswordPayload := env.request(t, http.MethodPut, "/api/v1/users/me/password", map[string]string{
		"current_password": "wrong-password", "new_password": "new-password",
	}, access)
	if wrongPassword.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong current password status = %d; want 400", wrongPassword.StatusCode)
	}
	assertError(t, wrongPasswordPayload, "invalid_password")
	stillValid, stillValidPayload := env.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refresh,
	}, "")
	if stillValid.StatusCode != http.StatusOK {
		t.Fatalf("refresh token after rejected password change status = %d, body = %+v", stillValid.StatusCode, stillValidPayload)
	}
	access = stringField(t, stillValidPayload, "access_token")
	refresh = stringField(t, stillValidPayload, "refresh_token")
	parallelSession := env.login(t, "student@example.test", "old-password")
	parallelRefresh := stringField(t, parallelSession, "refresh_token")

	passwordChange, _ := env.request(t, http.MethodPut, "/api/v1/users/me/password", map[string]string{
		"current_password": "old-password", "new_password": "new-password",
	}, access)
	if passwordChange.StatusCode != http.StatusNoContent {
		t.Fatalf("password change status = %d; want 204", passwordChange.StatusCode)
	}
	for _, invalidatedRefresh := range []string{refresh, parallelRefresh} {
		oldAfterPassword, oldAfterPasswordPayload := env.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
			"refresh_token": invalidatedRefresh,
		}, "")
		if oldAfterPassword.StatusCode != http.StatusUnauthorized {
			t.Fatalf("refresh after password change status = %d; want 401", oldAfterPassword.StatusCode)
		}
		assertError(t, oldAfterPasswordPayload, "invalid_refresh_token")
	}

	newLogin := env.login(t, "student@example.test", "new-password")
	logoutRefresh := stringField(t, newLogin, "refresh_token")
	parallelLogoutRefresh := stringField(t, env.login(t, "student@example.test", "new-password"), "refresh_token")
	logout, _ := env.request(t, http.MethodPost, "/api/v1/auth/logout", map[string]string{
		"refresh_token": logoutRefresh,
	}, "")
	if logout.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d; want 204", logout.StatusCode)
	}
	for _, revokedRefresh := range []string{logoutRefresh, parallelLogoutRefresh} {
		afterLogout, afterLogoutPayload := env.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
			"refresh_token": revokedRefresh,
		}, "")
		if afterLogout.StatusCode != http.StatusUnauthorized {
			t.Fatalf("refresh after logout status = %d; want 401", afterLogout.StatusCode)
		}
		assertError(t, afterLogoutPayload, "invalid_refresh_token")
	}
}

func TestAdminAuthorizationRoleRevocationAndPagination(t *testing.T) {
	env := newTestEnvironment(t)
	studentPayload := env.register(t, "Student", "student@example.test", "password")
	student := studentPayload
	studentID := int64Field(t, student, "id")
	studentTokens := env.login(t, "student@example.test", "password")
	studentAccess := stringField(t, studentTokens, "access_token")
	studentRefresh := stringField(t, studentTokens, "refresh_token")

	adminHash, err := auth.HashPassword("admin-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.CreateUser(context.Background(), "Admin", "admin@example.test", adminHash, auth.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	adminTokens := env.login(t, "admin@example.test", "admin-password")
	adminAccess := stringField(t, adminTokens, "access_token")

	for _, request := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/admin/users", nil},
		{http.MethodPut, "/api/v1/admin/users/" + strconv.FormatInt(studentID, 10) + "/role", map[string]string{"role": auth.RoleAdmin}},
	} {
		response, payload := env.request(t, request.method, request.path, request.body, studentAccess)
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("%s %s as STUDENT returned %d; want 403", request.method, request.path, response.StatusCode)
		}
		assertError(t, payload, "forbidden")
	}
	var deniedCount int
	var deniedDetail string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT count(*), max(detail) FROM audit_logs
		WHERE user_id = $1 AND event_type = 'ROLE_CHANGE_DENIED'
	`, studentID).Scan(&deniedCount, &deniedDetail); err != nil {
		t.Fatal(err)
	}
	var deniedAudit map[string]any
	if deniedCount != 1 || json.Unmarshal([]byte(deniedDetail), &deniedAudit) != nil ||
		deniedAudit["target_user_id"] != strconv.FormatInt(studentID, 10) {
		t.Fatalf("denied role-change audit = count %d, detail %q", deniedCount, deniedDetail)
	}

	usersResponse, usersPayload := env.request(t, http.MethodGet, "/api/v1/admin/users?page=1&page_size=1", nil, adminAccess)
	if usersResponse.StatusCode != http.StatusOK {
		t.Fatalf("admin users status = %d, body = %+v", usersResponse.StatusCode, usersPayload)
	}
	if usersPayload["page"] != float64(1) || usersPayload["page_size"] != float64(1) || usersPayload["has_next"] != true {
		t.Fatalf("admin pagination metadata = %#v", usersPayload)
	}
	items, ok := usersPayload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("admin users items = %#v; want one item", usersPayload["items"])
	}
	overflowPage, overflowPagePayload := env.request(t, http.MethodGet,
		"/api/v1/admin/users?page=92233720368547759&page_size=100", nil, adminAccess)
	if overflowPage.StatusCode != http.StatusBadRequest {
		t.Fatalf("overflow pagination status = %d; want 400", overflowPage.StatusCode)
	}
	assertError(t, overflowPagePayload, "invalid_pagination")

	expired := expiredAccessToken(t)
	expiredResponse, expiredPayload := env.request(t, http.MethodGet, "/api/v1/users/me", nil, expired)
	if expiredResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired token status = %d; want 401", expiredResponse.StatusCode)
	}
	assertError(t, expiredPayload, "unauthorized")

	roleResponse, rolePayload := env.request(t, http.MethodPut,
		"/api/v1/admin/users/"+strconv.FormatInt(studentID, 10)+"/role",
		map[string]string{"role": auth.RoleLecturer}, adminAccess)
	if roleResponse.StatusCode != http.StatusOK || rolePayload["role"] != auth.RoleLecturer {
		t.Fatalf("admin role change = %d, %#v", roleResponse.StatusCode, rolePayload)
	}
	admin, _, err := env.store.UserByEmail(context.Background(), "admin@example.test")
	if err != nil {
		t.Fatal(err)
	}
	var changedDetail string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT detail FROM audit_logs
		WHERE user_id = $1 AND event_type = 'ROLE_CHANGED'
	`, admin.ID).Scan(&changedDetail); err != nil {
		t.Fatal(err)
	}
	var changedAudit map[string]any
	if err := json.Unmarshal([]byte(changedDetail), &changedAudit); err != nil {
		t.Fatal(err)
	}
	if changedAudit["target_user_id"] != float64(studentID) ||
		changedAudit["old_role"] != auth.RoleStudent || changedAudit["new_role"] != auth.RoleLecturer {
		t.Fatalf("role-change audit detail = %#v", changedAudit)
	}
	revoked, revokedPayload := env.request(t, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": studentRefresh,
	}, "")
	if revoked.StatusCode != http.StatusUnauthorized {
		t.Fatalf("refresh after admin role change status = %d; want 401", revoked.StatusCode)
	}
	assertError(t, revokedPayload, "invalid_refresh_token")
}

func TestWebSocketTicketIsBoundToOwnerAndSingleUse(t *testing.T) {
	env := newTestEnvironment(t)
	studentPayload := env.register(t, "Student", "student@example.test", "password")
	student := studentPayload
	studentID := int64Field(t, student, "id")
	studentAccess := stringField(t, env.login(t, "student@example.test", "password"), "access_token")

	lecturer, err := env.store.CreateUser(context.Background(), "Lecturer", "lecturer@example.test", "test-hash", auth.RoleLecturer)
	if err != nil {
		t.Fatal(err)
	}

	var courseID, assignmentID, submissionID int64
	err = env.pool.QueryRow(context.Background(), `
		INSERT INTO courses (code, name, lecturer_id, term)
		VALUES ('C-1', 'Course', $1, '2026-1') RETURNING id
	`, lecturer.ID).Scan(&courseID)
	if err != nil {
		t.Fatal(err)
	}
	err = env.pool.QueryRow(context.Background(), `
		INSERT INTO assignments (course_id, title, deadline_at, late_until_at)
		VALUES ($1, 'Assignment', now(), now()) RETURNING id
	`, courseID).Scan(&assignmentID)
	if err != nil {
		t.Fatal(err)
	}
	err = env.pool.QueryRow(context.Background(), `
		INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes)
		VALUES ($1, $2, 'private/main.py', 'main.py', 10) RETURNING id
	`, studentID, assignmentID).Scan(&submissionID)
	if err != nil {
		t.Fatal(err)
	}

	response, payload := env.request(t, http.MethodPost, "/api/v1/ws-tickets", map[string]string{
		"submission_id": strconv.FormatInt(submissionID, 10),
	}, studentAccess)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("ticket status = %d; body = %+v", response.StatusCode, payload)
	}
	rawTicket := stringField(t, payload, "ticket")
	consumed, err := env.tickets.Consume(rawTicket)
	if err != nil {
		t.Fatalf("consume issued ticket: %v", err)
	}
	if consumed.UserID != studentID || consumed.SubmissionID != submissionID ||
		time.Until(consumed.ExpiresAt) > auth.MaxTicketTTL {
		t.Fatalf("ticket claims = %+v; want user %d submission %d and <=60 seconds", consumed, studentID, submissionID)
	}
	if _, err := env.tickets.Consume(rawTicket); !errors.Is(err, auth.ErrInvalidTicket) {
		t.Fatalf("ticket second use error = %v; want ErrInvalidTicket", err)
	}

	_ = env.register(t, "Other Student", "other@example.test", "password")
	otherAccess := stringField(t, env.login(t, "other@example.test", "password"), "access_token")
	forbidden, forbiddenPayload := env.request(t, http.MethodPost, "/api/v1/ws-tickets", map[string]string{
		"submission_id": strconv.FormatInt(submissionID, 10),
	}, otherAccess)
	if forbidden.StatusCode != http.StatusNotFound {
		t.Fatalf("ticket for another user's submission status = %d; want 404", forbidden.StatusCode)
	}
	assertError(t, forbiddenPayload, "not_found")
}

func expiredAccessToken(t *testing.T) string {
	t.Helper()
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss":  "sandboxed-autograder",
		"sub":  "1",
		"role": auth.RoleStudent,
		"iat":  now.Add(-2 * time.Hour).Unix(),
		"nbf":  now.Add(-2 * time.Hour).Unix(),
		"exp":  now.Add(-time.Hour).Unix(),
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestInternalErrorMapsProtectedDeleteSQLStates(t *testing.T) {
	for _, state := range []string{"23503", "23001"} {
		recorder := httptest.NewRecorder()
		internalError(recorder, "delete protected row", &pgconn.PgError{Code: state})
		if recorder.Code != http.StatusConflict {
			t.Fatalf("SQLSTATE %s status = %d; want 409", state, recorder.Code)
		}
		var payload map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		assertError(t, payload, "conflict")
	}

	recorder := httptest.NewRecorder()
	internalError(recorder, "unexpected database error", errors.New("database unavailable"))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected error status = %d; want 500", recorder.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	assertError(t, payload, "internal_error")
}

func TestParseIDAcceptsIntegerAndRejectsInvalidValues(t *testing.T) {
	for _, input := range []string{`7`, `"7"`} {
		id, err := parseID(json.RawMessage(input))
		if err != nil || id != 7 {
			t.Fatalf("parseID(%s) = %d, %v; want 7, nil", input, id, err)
		}

	}
	for _, input := range []string{`null`, `0`, `-1`, `"not-an-id"`, `{}`} {
		if _, err := parseID(json.RawMessage(input)); err == nil {
			t.Fatalf("parseID(%s) succeeded; want error", input)
		}
	}
}

func TestFiftyConcurrentLoginsComplete(t *testing.T) {
	env := newTestEnvironment(t)
	_ = env.register(t, "Concurrent Login", "concurrent@example.test", "password")

	var wait sync.WaitGroup
	errorsFound := make(chan error, 50)
	for range 50 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			body, err := json.Marshal(map[string]string{
				"email": "concurrent@example.test", "password": "password",
			})
			if err != nil {
				errorsFound <- err
				return
			}
			response, err := env.server.Client().Post(
				env.server.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body),
			)
			if err != nil {
				errorsFound <- err
				return
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				payload, _ := io.ReadAll(response.Body)
				errorsFound <- fmt.Errorf("login status %d: %s", response.StatusCode, payload)
			}
		}()
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
}

func TestConcurrentRefreshRotationAndPasswordChangeRevokeAllTokens(t *testing.T) {
	env := newTestEnvironment(t)
	user, err := env.store.CreateUser(context.Background(), "Race User", "race@example.test", "unused", auth.RoleStudent)
	if err != nil {
		t.Fatal(err)
	}

	for iteration := range 40 {
		oldHash := auth.HashRefreshToken(fmt.Sprintf("old-%d", iteration))
		newHash := auth.HashRefreshToken(fmt.Sprintf("new-%d", iteration))
		if err := env.store.InsertRefreshToken(context.Background(), user.ID, oldHash, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}

		start := make(chan struct{})
		var wait sync.WaitGroup
		wait.Add(2)
		rotationResult := make(chan error, 1)
		passwordResult := make(chan error, 1)
		go func() {
			defer wait.Done()
			<-start
			_, err := env.store.RotateRefreshToken(context.Background(), oldHash, newHash, time.Now().Add(time.Hour))
			rotationResult <- err
		}()
		go func() {
			defer wait.Done()
			<-start
			passwordResult <- env.store.ChangePassword(context.Background(), user.ID, fmt.Sprintf("changed-%d", iteration))
		}()
		close(start)
		wait.Wait()
		if err := <-passwordResult; err != nil {
			t.Fatalf("iteration %d: change password: %v", iteration, err)
		}
		if err := <-rotationResult; err != nil && !errors.Is(err, auth.ErrInvalidRefresh) {
			t.Fatalf("iteration %d: rotate refresh token: %v", iteration, err)
		}

		var validTokens int
		if err := env.pool.QueryRow(context.Background(), `
				SELECT count(*) FROM refresh_tokens
				WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
			`, user.ID).Scan(&validTokens); err != nil {
			t.Fatalf("iteration %d: count valid refresh tokens: %v", iteration, err)
		}
		if validTokens != 0 {
			t.Fatalf("iteration %d: found %d valid refresh tokens after concurrent password change", iteration, validTokens)
		}
	}
}
