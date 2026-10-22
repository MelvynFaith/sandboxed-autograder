package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/courses"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/settings"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/submissions"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxRequestBytes = 1 << 20
const maxSubmissionBytes = 256 << 10
const maxUserNameLength = 200

var (
	dummyPasswordHashMu sync.Mutex
	dummyPasswordHash   string
)

type API struct {
	store       *auth.Store
	courses     *courses.Store
	tokens      *auth.Tokens
	tickets     *auth.TicketStore
	submissions *submissions.Store
	settings    *settings.Store
	queue       queue.JobQueue
	codeDir     string
	mux         *http.ServeMux
}

func NewHandler(
	store *auth.Store,
	tokens *auth.Tokens,
	tickets *auth.TicketStore,
	courseStore *courses.Store,
	submissionStore *submissions.Store,
	settingsStore *settings.Store,
	jobs queue.JobQueue,
	codeDir string,
) *API {
	if tickets == nil {
		tickets, _ = auth.NewTicketStore(auth.MaxTicketTTL)
	}
	api := &API{
		store: store, courses: courseStore, tokens: tokens, tickets: tickets,
		submissions: submissionStore, settings: settingsStore, queue: jobs, codeDir: codeDir,
		mux: http.NewServeMux(),
	}
	api.routes()
	return api
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *API) routes() {
	a.mux.HandleFunc("POST /api/v1/auth/register", a.register)
	a.mux.HandleFunc("POST /api/v1/auth/login", a.login)
	a.mux.HandleFunc("POST /api/v1/auth/refresh", a.refresh)
	a.mux.HandleFunc("POST /api/v1/auth/logout", a.logout)
	a.mux.HandleFunc("GET /api/v1/users/me", a.requireAuth(a.getMe))
	a.mux.HandleFunc("PUT /api/v1/users/me", a.requireAuth(a.updateMe))
	a.mux.HandleFunc("PUT /api/v1/users/me/password", a.requireAuth(a.changePassword))
	a.mux.HandleFunc("POST /api/v1/ws-tickets", a.requireAuth(a.createWSTicket))
	a.mux.HandleFunc("POST /api/v1/submissions", a.requireAuth(a.createSubmission))
	a.mux.HandleFunc("POST /api/v1/submissions/{id}/regrade", a.requireAuth(a.regradeSubmission))
	a.mux.HandleFunc("GET /api/v1/submissions", a.requireAuth(a.listSubmissions))
	a.mux.HandleFunc("DELETE /api/v1/submissions/{id}", a.requireAuth(a.cancelSubmission))
	a.mux.HandleFunc("GET /api/v1/submissions/{id}", a.requireAuth(a.getSubmission))
	a.mux.HandleFunc("GET /api/v1/submissions/{id}/report", a.requireAuth(a.getSubmissionReport))
	a.mux.HandleFunc("GET /api/v1/submissions/{id}/code", a.requireAuth(a.getSubmissionCode))
	a.mux.HandleFunc("GET /api/v1/courses", a.requireAuth(a.listCourses))
	a.mux.HandleFunc("POST /api/v1/courses", a.requireAuth(a.createCourse))
	a.mux.HandleFunc("GET /api/v1/courses/{id}/enrollments", a.requireAuth(a.listEnrollments))
	a.mux.HandleFunc("POST /api/v1/courses/{id}/enrollments", a.requireAuth(a.createEnrollment))
	a.mux.HandleFunc("DELETE /api/v1/courses/{id}/enrollments/{user_id}", a.requireAuth(a.deleteEnrollment))
	a.mux.HandleFunc("GET /api/v1/courses/{id}/assignments", a.requireAuth(a.listCourseAssignments))
	a.mux.HandleFunc("POST /api/v1/courses/{id}/assignments", a.requireAuth(a.createAssignment))
	a.mux.HandleFunc("GET /api/v1/assignments", a.requireAuth(a.listAssignments))
	a.mux.HandleFunc("GET /api/v1/assignments/{id}", a.requireAuth(a.getAssignment))
	a.mux.HandleFunc("PUT /api/v1/assignments/{id}", a.requireAuth(a.updateAssignment))
	a.mux.HandleFunc("DELETE /api/v1/assignments/{id}", a.requireAuth(a.deleteAssignment))
	a.mux.HandleFunc("POST /api/v1/assignments/{id}/regrade", a.requireAuth(a.regradeAssignment))
	a.mux.HandleFunc("GET /api/v1/assignments/{id}/resource-limit", a.requireAuth(a.getResourceLimit))
	a.mux.HandleFunc("PUT /api/v1/assignments/{id}/resource-limit", a.requireAuth(a.putResourceLimit))
	a.mux.HandleFunc("GET /api/v1/assignments/{id}/results", a.requireAuth(a.listAssignmentResults))
	a.mux.HandleFunc("GET /api/v1/assignments/{id}/results/export", a.requireAuth(a.exportAssignmentResults))
	a.mux.HandleFunc("GET /api/v1/assignments/{id}/testcases", a.requireAuth(a.listTestCases))
	a.mux.HandleFunc("POST /api/v1/assignments/{id}/testcases", a.requireAuth(a.createTestCase))
	a.mux.HandleFunc("GET /api/v1/assignments/{id}/testcases/export", a.requireAuth(a.exportTestCases))
	a.mux.HandleFunc("POST /api/v1/assignments/{id}/testcases/import", a.requireAuth(a.importTestCases))
	a.mux.HandleFunc("PUT /api/v1/testcases/{id}", a.requireAuth(a.updateTestCase))
	a.mux.HandleFunc("DELETE /api/v1/testcases/{id}", a.requireAuth(a.deleteTestCase))
	a.mux.HandleFunc("GET /api/v1/admin/users", a.requireAdmin(a.listUsers))
	a.mux.HandleFunc("PUT /api/v1/admin/users/{id}/role", a.requireAuth(a.updateRole))
	a.mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Endpoint tidak ditemukan.")
	})
}

type authenticatedHandler func(http.ResponseWriter, *http.Request, auth.User)

func (a *API) requireAuth(next authenticatedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fields := strings.Fields(r.Header.Get("Authorization"))
		if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Autentikasi diperlukan.")
			return
		}
		userID, err := a.tokens.ParseAccessToken(fields[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Access token tidak valid atau kedaluwarsa.")
			return
		}
		user, err := a.store.UserByID(r.Context(), userID)
		if errors.Is(err, auth.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Access token tidak valid.")
			return
		}
		if err != nil {
			internalError(w, "load authenticated user", err)
			return
		}
		next(w, r, user)
	}
}

func (a *API) requireAdmin(next authenticatedHandler) http.HandlerFunc {
	return a.requireAuth(func(w http.ResponseWriter, r *http.Request, user auth.User) {
		if user.Role != auth.RoleAdmin {
			writeError(w, http.StatusForbidden, "forbidden", "Akses hanya untuk administrator.")
			return
		}
		next(w, r, user)
	})
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if request.Name == "" || utf8.RuneCountInString(request.Name) > maxUserNameLength ||
		!validEmail(request.Email) || request.Password == "" || len(request.Password) > 1024 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Nama wajib diisi dan maksimal 200 karakter; email dan kata sandi harus valid.")
		return
	}
	exists, err := a.store.EmailExists(r.Context(), request.Email)
	if err != nil {
		internalError(w, "check registration email", err)
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "email_already_exists", "Email sudah terdaftar.")
		return
	}
	passwordHash, err := auth.HashPasswordContext(r.Context(), request.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Kata sandi tidak valid.")
		return
	}
	user, err := a.store.CreateUser(r.Context(), request.Name, request.Email, passwordHash, auth.RoleStudent)
	if errors.Is(err, auth.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "email_already_exists", "Email sudah terdaftar.")
		return
	}
	if err != nil {
		internalError(w, "register user", err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Password == "" || len(request.Password) > 1024 {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Email atau kata sandi tidak valid.")
		return
	}
	dummyHash, err := getDummyPasswordHash(r.Context())
	if err != nil {
		internalError(w, "prepare login password verification", err)
		return
	}
	user, passwordHash, err := a.store.UserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(request.Email)))
	if err != nil && !errors.Is(err, auth.ErrInvalidLogin) {
		internalError(w, "load login user", err)
		return
	}
	if errors.Is(err, auth.ErrInvalidLogin) {
		passwordHash = dummyHash
	}
	valid, hashErr := auth.VerifyPasswordContext(r.Context(), passwordHash, request.Password)
	if err != nil || hashErr != nil || !valid {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Email atau kata sandi tidak valid.")
		return
	}
	accessToken, refreshToken, err := a.issueSession(r, user)
	if err != nil {
		internalError(w, "issue login session", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          user,
	})
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.RefreshToken == "" {
		writeError(w, http.StatusUnauthorized, "invalid_refresh_token", "Refresh token tidak valid.")
		return
	}
	nextRefresh, err := auth.NewRefreshToken()
	if err != nil {
		internalError(w, "generate refresh token", err)
		return
	}
	user, err := a.store.RotateRefreshToken(
		r.Context(),
		auth.HashRefreshToken(request.RefreshToken),
		auth.HashRefreshToken(nextRefresh),
		time.Now().Add(auth.RefreshTokenTTL),
	)
	if errors.Is(err, auth.ErrInvalidRefresh) {
		writeError(w, http.StatusUnauthorized, "invalid_refresh_token", "Refresh token tidak valid atau kedaluwarsa.")
		return
	}
	if err != nil {
		internalError(w, "rotate refresh token", err)
		return
	}
	accessToken, err := a.tokens.IssueAccessToken(user)
	if err != nil {
		internalError(w, "issue refreshed access token", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  accessToken,
		"refresh_token": nextRefresh,
	})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.RefreshToken != "" {
		if err := a.store.RevokeRefreshToken(r.Context(), auth.HashRefreshToken(request.RefreshToken)); err != nil {
			internalError(w, "logout user", err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) getMe(w http.ResponseWriter, _ *http.Request, user auth.User) {
	writeJSON(w, http.StatusOK, user)
}

func (a *API) updateMe(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || utf8.RuneCountInString(request.Name) > maxUserNameLength {
		writeError(w, http.StatusBadRequest, "invalid_request", "Nama wajib diisi dan maksimal 200 karakter.")
		return
	}
	updated, err := a.store.UpdateName(r.Context(), user.ID, request.Name)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "update profile name", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) changePassword(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.CurrentPassword == "" || request.NewPassword == "" || len(request.NewPassword) > 1024 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Kata sandi lama dan baru wajib diisi.")
		return
	}
	_, passwordHash, err := a.store.UserByEmail(r.Context(), user.Email)
	if err != nil {
		internalError(w, "load password for verification", err)
		return
	}
	valid, err := auth.VerifyPasswordContext(r.Context(), passwordHash, request.CurrentPassword)
	if err != nil {
		internalError(w, "verify current password", err)
		return
	}
	if !valid {
		writeError(w, http.StatusBadRequest, "invalid_password", "Kata sandi lama tidak sesuai.")
		return
	}
	newHash, err := auth.HashPasswordContext(r.Context(), request.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Kata sandi baru tidak valid.")
		return
	}
	if err := a.store.ChangePassword(r.Context(), user.ID, newHash); err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan.")
			return
		}
		internalError(w, "change password", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) createWSTicket(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		SubmissionID json.RawMessage `json:"submission_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	submissionID, err := parseID(request.SubmissionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "submission_id harus berupa ID positif.")
		return
	}
	ownerID, err := a.store.SubmissionOwner(r.Context(), submissionID)
	if errors.Is(err, auth.ErrNotFound) || (err == nil && ownerID != user.ID) {
		writeError(w, http.StatusNotFound, "not_found", "Submission tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "check WebSocket ticket submission owner", err)
		return
	}
	ticket, _, err := a.tickets.Issue(user.ID, submissionID)
	if err != nil {
		internalError(w, "issue WebSocket ticket", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"ticket": ticket})
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request, _ auth.User) {
	page, pageSize, ok := parsePage(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "page dan page_size harus bilangan bulat positif; page_size maksimal 100.")
		return
	}
	result, err := a.store.ListUsers(r.Context(), page, pageSize)
	if err != nil {
		internalError(w, "list admin users", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) updateRole(w http.ResponseWriter, r *http.Request, actor auth.User) {
	if actor.Role != auth.RoleAdmin {
		if err := a.store.RecordDeniedRoleChange(r.Context(), actor.ID, r.PathValue("id")); err != nil {
			internalError(w, "record denied role change", err)
			return
		}
		writeError(w, http.StatusForbidden, "forbidden", "Akses hanya untuk administrator.")
		return
	}
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "ID pengguna tidak valid.")
		return
	}
	var request struct {
		Role string `json:"role"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Role != auth.RoleStudent && request.Role != auth.RoleLecturer && request.Role != auth.RoleAdmin {
		writeError(w, http.StatusBadRequest, "invalid_role", "Peran pengguna tidak valid.")
		return
	}
	user, err := a.store.SetRole(r.Context(), actor.ID, userID, request.Role)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan.")
		return
	}

	if err != nil {
		internalError(w, "update user role", err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func getDummyPasswordHash(ctx context.Context) (string, error) {
	dummyPasswordHashMu.Lock()
	if dummyPasswordHash != "" {
		hash := dummyPasswordHash
		dummyPasswordHashMu.Unlock()
		return hash, nil
	}
	dummyPasswordHashMu.Unlock()

	hash, err := auth.HashPasswordContext(ctx, "fixed dummy password for unknown login")
	if err != nil {
		return "", err
	}
	dummyPasswordHashMu.Lock()
	if dummyPasswordHash == "" {
		dummyPasswordHash = hash
	}
	hash = dummyPasswordHash
	dummyPasswordHashMu.Unlock()
	return hash, nil
}

func (a *API) issueSession(r *http.Request, user auth.User) (string, string, error) {
	accessToken, err := a.tokens.IssueAccessToken(user)
	if err != nil {
		return "", "", err
	}
	refreshToken, err := auth.NewRefreshToken()
	if err != nil {
		return "", "", err
	}
	err = a.store.InsertRefreshToken(
		r.Context(), user.ID, auth.HashRefreshToken(refreshToken), time.Now().Add(auth.RefreshTokenTTL),
	)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Body JSON tidak valid.")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Body JSON harus berisi satu objek.")
		return false
	}
	return true
}

func validEmail(email string) bool {
	if len(email) > 254 || strings.Count(email, "@") != 1 {
		return false
	}
	parts := strings.SplitN(email, "@", 2)
	return parts[0] != "" && strings.Contains(parts[1], ".") &&
		!strings.HasPrefix(parts[1], ".") && !strings.HasSuffix(parts[1], ".")
}

func parsePage(r *http.Request) (int, int, bool) {
	page := 1
	pageSize := 50
	var err error
	if value := r.URL.Query().Get("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			return 0, 0, false
		}
	}
	if value := r.URL.Query().Get("page_size"); value != "" {
		pageSize, err = strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > 100 {
			return 0, 0, false
		}
	}
	maxInt := int(^uint(0) >> 1)
	if page > maxInt/pageSize {
		return 0, 0, false
	}
	return page, pageSize, true
}

func parseID(raw json.RawMessage) (int64, error) {
	var stringID string
	if err := json.Unmarshal(raw, &stringID); err == nil {
		id, err := strconv.ParseInt(stringID, 10, 64)
		if err != nil || id <= 0 {
			return 0, errors.New("invalid ID")
		}
		return id, nil
	}
	var numericID int64
	if err := json.Unmarshal(raw, &numericID); err != nil || numericID <= 0 {
		return 0, errors.New("invalid ID")
	}
	return numericID, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func internalError(w http.ResponseWriter, operation string, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
		writeError(w, http.StatusConflict, "conflict", "Operasi ditolak karena data terkait masih digunakan.")
		return
	}
	log.Printf("%s: %v", operation, err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan pada server.")
}
