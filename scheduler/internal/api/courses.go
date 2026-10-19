package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/courses"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	maxCourseNameLength = 200
	maxCourseCodeLength = 32
	maxCourseTermLength = 20
)

func (a *API) listCourses(w http.ResponseWriter, r *http.Request, user auth.User) {
	page, pageSize, ok := parsePage(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "page dan page_size harus bilangan bulat positif; page_size maksimal 100.")
		return
	}
	result, err := a.courses.ListCourses(r.Context(), user.Role, user.ID, page, pageSize)
	if err != nil {
		internalError(w, "list courses", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) createCourse(w http.ResponseWriter, r *http.Request, user auth.User) {
	if user.Role != auth.RoleLecturer && user.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "Hanya dosen atau administrator yang dapat membuat Course.")
		return
	}
	var request struct {
		Name          string `json:"name"`
		Code          string `json:"code"`
		Term          string `json:"term"`
		LecturerEmail string `json:"lecturer_email"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Code = strings.TrimSpace(request.Code)
	request.Term = strings.TrimSpace(request.Term)
	request.LecturerEmail = strings.ToLower(strings.TrimSpace(request.LecturerEmail))
	if utf8.RuneCountInString(request.Name) == 0 ||
		utf8.RuneCountInString(request.Name) > maxCourseNameLength ||
		strings.ContainsRune(request.Name, '\x00') ||
		utf8.RuneCountInString(request.Code) == 0 ||
		utf8.RuneCountInString(request.Code) > maxCourseCodeLength ||
		strings.ContainsRune(request.Code, '\x00') ||
		utf8.RuneCountInString(request.Term) == 0 ||
		utf8.RuneCountInString(request.Term) > maxCourseTermLength ||
		strings.ContainsRune(request.Term, '\x00') {
		writeError(w, http.StatusBadRequest, "invalid_request", "Nama wajib diisi (maksimal 200 karakter), kode wajib diisi (maksimal 32 karakter), dan term wajib diisi (maksimal 20 karakter).")
		return
	}
	if request.LecturerEmail != "" && !validEmail(request.LecturerEmail) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Alamat email dosen tidak valid.")
		return
	}
	if user.Role == auth.RoleAdmin && request.LecturerEmail == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Admin wajib menentukan email dosen pengampu.")
		return
	}
	course, err := a.courses.CreateCourse(
		r.Context(), request.Name, request.Code, request.Term, user.Role, user.ID, request.LecturerEmail,
	)
	if err != nil {
		if errors.Is(err, courses.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user_not_found", "Dosen dengan email tersebut tidak ditemukan.")
			return
		}
		if errors.Is(err, courses.ErrLecturerRequired) {
			writeError(w, http.StatusBadRequest, "invalid_request", "Admin wajib menentukan email dosen pengampu.")
			return
		}
		if errors.Is(err, courses.ErrNotLecturer) {
			writeError(w, http.StatusBadRequest, "invalid_request", "Pengguna yang dipilih bukan dosen dengan peran LECTURER.")
			return
		}
		if errors.Is(err, courses.ErrCourseLecturerMismatch) {
			writeError(w, http.StatusBadRequest, "invalid_request", "Dosen hanya dapat membuat Course untuk dirinya sendiri.")
			return
		}
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "course_code_exists", "Kode Course sudah digunakan.")
			return
		}
		internalError(w, "create course", err)
		return
	}
	writeJSON(w, http.StatusCreated, course)
}

func (a *API) listEnrollments(w http.ResponseWriter, r *http.Request, user auth.User) {
	courseID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Course tidak ditemukan.")
		return
	}
	if !a.requireCourseManager(w, r, user, courseID) {
		return
	}
	page, pageSize, ok := parsePage(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "page dan page_size harus bilangan bulat positif; page_size maksimal 100.")
		return
	}
	result, err := a.courses.ListEnrollments(r.Context(), courseID, page, pageSize)
	if err != nil {
		internalError(w, "list course enrollments", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) createEnrollment(w http.ResponseWriter, r *http.Request, user auth.User) {
	courseID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Course tidak ditemukan.")
		return
	}
	if !a.requireCourseManager(w, r, user, courseID) {
		return
	}
	var request struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if !validEmail(request.Email) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Alamat email tidak valid.")
		return
	}
	err := a.courses.CreateEnrollmentByEmail(r.Context(), courseID, request.Email)
	if errors.Is(err, courses.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, "user_not_found", "Pengguna dengan email tersebut tidak ditemukan.")
		return
	}
	if errors.Is(err, courses.ErrNotStudent) {
		writeError(w, http.StatusBadRequest, "invalid_enrollment_user", "Hanya akun mahasiswa yang dapat didaftarkan.")
		return
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "enrollment_exists", "Mahasiswa sudah terdaftar pada Course ini.")
			return
		}
		internalError(w, "create course enrollment", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) deleteEnrollment(w http.ResponseWriter, r *http.Request, user auth.User) {
	courseID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Course tidak ditemukan.")
		return
	}
	userID, ok := coursePathID(r, "user_id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Pendaftaran mahasiswa tidak ditemukan.")
		return
	}
	if !a.requireCourseManager(w, r, user, courseID) {
		return
	}
	deleted, err := a.courses.DeleteEnrollment(r.Context(), courseID, userID)
	if err != nil {
		internalError(w, "delete course enrollment", err)
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "not_found", "Pendaftaran mahasiswa tidak ditemukan.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listCourseAssignments(w http.ResponseWriter, r *http.Request, user auth.User) {
	courseID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Course tidak ditemukan.")
		return
	}
	allowed, err := a.courses.CanViewAssignments(r.Context(), courseID, user.ID, user.Role)
	if err != nil {
		internalError(w, "check course assignment access", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "Course tidak ditemukan.")
		return
	}
	page, pageSize, ok := parsePage(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "page dan page_size harus bilangan bulat positif; page_size maksimal 100.")
		return
	}
	result, err := a.courses.ListAssignments(r.Context(), courseID, page, pageSize)
	if err != nil {
		internalError(w, "list course assignments", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) requireCourseManager(w http.ResponseWriter, r *http.Request, user auth.User, courseID int64) bool {
	allowed, err := a.courses.CanManageCourse(r.Context(), courseID, user.ID, user.Role)
	if err != nil {
		internalError(w, "check course management access", err)
		return false
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "not_found", "Course tidak ditemukan.")
		return false
	}
	return true
}

func coursePathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
