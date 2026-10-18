package courses

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrNotStudent             = errors.New("user is not a student")
	ErrNotLecturer            = errors.New("user is not a lecturer")
	ErrLecturerRequired       = errors.New("course lecturer is required")
	ErrCourseLecturerMismatch = errors.New("lecturer cannot assign a course to another user")
)

type Store struct {
	pool *pgxpool.Pool
}

type Course struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Term       string `json:"term"`
	LecturerID int64  `json:"lecturer_id"`
}

type CoursePage struct {
	Items    []Course `json:"items"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
	HasNext  bool     `json:"has_next"`
}

type Enrollment struct {
	ID         int64     `json:"id"`
	CourseID   int64     `json:"course_id"`
	UserID     int64     `json:"user_id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	EnrolledAt time.Time `json:"enrolled_at"`
}

type EnrollmentPage struct {
	Items    []Enrollment `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	HasNext  bool         `json:"has_next"`
}

type Assignment struct {
	ID          int64     `json:"id"`
	CourseID    int64     `json:"course_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DeadlineAt  time.Time `json:"deadline_at"`
	LateUntilAt time.Time `json:"late_until_at"`
}

type AssignmentPage struct {
	Items    []Assignment `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	HasNext  bool         `json:"has_next"`
}

type ResourceLimit struct {
	AssignmentID   int64 `json:"assignment_id"`
	CPUTimeMS      int   `json:"cpu_time_ms"`
	MemoryMB       int   `json:"memory_mb"`
	MaxProcesses   int   `json:"max_processes"`
	TimeoutS       int   `json:"timeout_s"`
	MaxOutputBytes int   `json:"max_output_bytes"`
	TmpfsSizeMB    int   `json:"tmpfs_size_mb"`
}

type TestCase struct {
	ID             int64  `json:"id"`
	AssignmentID   int64  `json:"assignment_id"`
	ExternalID     string `json:"external_id"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	Weight         int    `json:"weight"`
	IsHidden       bool   `json:"is_hidden"`
	MatchMode      string `json:"match_mode"`
}

type TestCasePage struct {
	Items    []TestCase `json:"items"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	HasNext  bool       `json:"has_next"`
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateCourse(ctx context.Context, name, code, term, role string, actorID int64, lecturerEmail string) (Course, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Course{}, fmt.Errorf("begin create course: %w", err)
	}
	defer tx.Rollback(ctx)

	var lecturerID int64
	switch role {
	case "LECTURER":
		var actorEmail string
		err = tx.QueryRow(ctx, `
			SELECT id, email FROM users WHERE id = $1 AND role = 'LECTURER' FOR SHARE
		`, actorID).Scan(&lecturerID, &actorEmail)
		if errors.Is(err, pgx.ErrNoRows) {
			return Course{}, ErrNotLecturer
		}
		if err != nil {
			return Course{}, fmt.Errorf("verify course lecturer: %w", err)
		}
		if lecturerEmail != "" && !strings.EqualFold(lecturerEmail, actorEmail) {
			return Course{}, ErrCourseLecturerMismatch
		}
	case "ADMIN":
		if lecturerEmail == "" {
			return Course{}, ErrLecturerRequired
		}
		var lecturerRole string
		err = tx.QueryRow(ctx, `
			SELECT id, role FROM users WHERE email = $1 FOR SHARE
		`, lecturerEmail).Scan(&lecturerID, &lecturerRole)
		if errors.Is(err, pgx.ErrNoRows) {
			return Course{}, ErrUserNotFound
		}
		if err != nil {
			return Course{}, fmt.Errorf("find course lecturer: %w", err)
		}
		if lecturerRole != "LECTURER" {
			return Course{}, ErrNotLecturer
		}
	default:
		return Course{}, fmt.Errorf("create course: unsupported creator role %q", role)
	}

	var course Course
	err = tx.QueryRow(ctx, `
		INSERT INTO courses (name, code, lecturer_id, term)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, code, term, lecturer_id
	`, name, code, lecturerID, term).Scan(
		&course.ID, &course.Name, &course.Code, &course.Term, &course.LecturerID,
	)
	if err != nil {
		return Course{}, fmt.Errorf("create course: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Course{}, fmt.Errorf("commit create course: %w", err)
	}
	return course, nil
}

func (s *Store) ListCourses(ctx context.Context, role string, userID int64, page, pageSize int) (CoursePage, error) {
	var query string
	var args []any
	switch role {
	case "STUDENT":
		query = `
			SELECT c.id, c.name, c.code, c.term, c.lecturer_id
			FROM courses c
			WHERE EXISTS (
				SELECT 1 FROM enrollments e WHERE e.course_id = c.id AND e.user_id = $1
			)
			ORDER BY c.id
			LIMIT $2 OFFSET $3
		`
		args = []any{userID, pageSize + 1, (page - 1) * pageSize}
	case "LECTURER":
		query = `
			SELECT c.id, c.name, c.code, c.term, c.lecturer_id
			FROM courses c
			JOIN users u ON u.id = c.lecturer_id AND u.role = 'LECTURER'
			WHERE c.lecturer_id = $1
			ORDER BY c.id
			LIMIT $2 OFFSET $3
		`
		args = []any{userID, pageSize + 1, (page - 1) * pageSize}
	case "ADMIN":
		query = `
			SELECT id, name, code, term, lecturer_id
			FROM courses
			ORDER BY id
			LIMIT $1 OFFSET $2
		`
		args = []any{pageSize + 1, (page - 1) * pageSize}
	default:
		return CoursePage{}, fmt.Errorf("list courses: unsupported role %q", role)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return CoursePage{}, fmt.Errorf("list courses: %w", err)
	}
	defer rows.Close()

	items := make([]Course, 0, pageSize+1)
	for rows.Next() {
		var course Course
		if err := rows.Scan(&course.ID, &course.Name, &course.Code, &course.Term, &course.LecturerID); err != nil {
			return CoursePage{}, fmt.Errorf("scan course: %w", err)
		}
		items = append(items, course)
	}
	if err := rows.Err(); err != nil {
		return CoursePage{}, fmt.Errorf("list courses rows: %w", err)
	}
	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}
	return CoursePage{Items: items, Page: page, PageSize: pageSize, HasNext: hasNext}, nil
}

func (s *Store) CanManageCourse(ctx context.Context, courseID, userID int64, role string) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM courses c
			WHERE c.id = $1 AND ($2 = 'ADMIN' OR ($2 = 'LECTURER' AND c.lecturer_id = $3))
		)
	`, courseID, role, userID).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check course management access: %w", err)
	}
	return allowed, nil
}

func (s *Store) CanViewAssignments(ctx context.Context, courseID, userID int64, role string) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM courses c
			WHERE c.id = $1 AND (
				$2 = 'ADMIN'
				OR ($2 = 'LECTURER' AND c.lecturer_id = $3)
				OR ($2 = 'STUDENT' AND EXISTS (
					SELECT 1 FROM enrollments e WHERE e.course_id = c.id AND e.user_id = $3
				))
			)
		)
	`, courseID, role, userID).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check course assignment access: %w", err)
	}
	return allowed, nil
}

func (s *Store) CreateEnrollmentByEmail(ctx context.Context, courseID int64, email string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin course enrollment: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID int64
	var role string
	err = tx.QueryRow(ctx, `
		SELECT id, role FROM users WHERE email = $1 FOR SHARE
	`, email).Scan(&userID, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("get enrollment user by email: %w", err)
	}
	if role != "STUDENT" {
		return ErrNotStudent
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO enrollments (course_id, user_id) VALUES ($1, $2)
	`, courseID, userID); err != nil {
		return fmt.Errorf("create enrollment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit course enrollment: %w", err)
	}
	return nil
}

func (s *Store) ListEnrollments(ctx context.Context, courseID int64, page, pageSize int) (EnrollmentPage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.course_id, e.user_id, u.name, u.email, e.enrolled_at
		FROM enrollments e
		JOIN users u ON u.id = e.user_id
		WHERE e.course_id = $1
		ORDER BY e.user_id
		LIMIT $2 OFFSET $3
	`, courseID, pageSize+1, (page-1)*pageSize)
	if err != nil {
		return EnrollmentPage{}, fmt.Errorf("list enrollments: %w", err)
	}
	defer rows.Close()

	items := make([]Enrollment, 0, pageSize+1)
	for rows.Next() {
		var enrollment Enrollment
		if err := rows.Scan(
			&enrollment.ID, &enrollment.CourseID, &enrollment.UserID,
			&enrollment.Name, &enrollment.Email, &enrollment.EnrolledAt,
		); err != nil {
			return EnrollmentPage{}, fmt.Errorf("scan enrollment: %w", err)
		}
		items = append(items, enrollment)
	}
	if err := rows.Err(); err != nil {
		return EnrollmentPage{}, fmt.Errorf("list enrollment rows: %w", err)
	}
	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}
	return EnrollmentPage{Items: items, Page: page, PageSize: pageSize, HasNext: hasNext}, nil
}

func (s *Store) DeleteEnrollment(ctx context.Context, courseID, userID int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM enrollments WHERE course_id = $1 AND user_id = $2
	`, courseID, userID)
	if err != nil {
		return false, fmt.Errorf("delete enrollment: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) ListAssignments(ctx context.Context, courseID int64, page, pageSize int) (AssignmentPage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, course_id, title, description, deadline_at, late_until_at
		FROM assignments
		WHERE course_id = $1
		ORDER BY id
		LIMIT $2 OFFSET $3
	`, courseID, pageSize+1, (page-1)*pageSize)
	if err != nil {
		return AssignmentPage{}, fmt.Errorf("list course assignments: %w", err)
	}
	defer rows.Close()

	items := make([]Assignment, 0, pageSize+1)
	for rows.Next() {
		var assignment Assignment
		if err := rows.Scan(
			&assignment.ID, &assignment.CourseID, &assignment.Title, &assignment.Description,
			&assignment.DeadlineAt, &assignment.LateUntilAt,
		); err != nil {
			return AssignmentPage{}, fmt.Errorf("scan course assignment: %w", err)
		}
		items = append(items, assignment)
	}
	if err := rows.Err(); err != nil {
		return AssignmentPage{}, fmt.Errorf("list course assignment rows: %w", err)
	}
	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}
	return AssignmentPage{Items: items, Page: page, PageSize: pageSize, HasNext: hasNext}, nil
}

func (s *Store) CreateAssignment(ctx context.Context, courseID int64, title, description string, deadlineAt, lateUntilAt time.Time) (Assignment, error) {
	var assignment Assignment
	err := s.pool.QueryRow(ctx, `
		INSERT INTO assignments (course_id, title, description, deadline_at, late_until_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, course_id, title, description, deadline_at, late_until_at
	`, courseID, title, description, deadlineAt, lateUntilAt).Scan(
		&assignment.ID, &assignment.CourseID, &assignment.Title, &assignment.Description,
		&assignment.DeadlineAt, &assignment.LateUntilAt,
	)
	if err != nil {
		return Assignment{}, fmt.Errorf("create assignment: %w", err)
	}
	return assignment, nil
}

func (s *Store) AssignmentByID(ctx context.Context, assignmentID int64) (Assignment, error) {
	var assignment Assignment
	err := s.pool.QueryRow(ctx, `
		SELECT id, course_id, title, description, deadline_at, late_until_at
		FROM assignments WHERE id = $1
	`, assignmentID).Scan(
		&assignment.ID, &assignment.CourseID, &assignment.Title, &assignment.Description,
		&assignment.DeadlineAt, &assignment.LateUntilAt,
	)
	if err != nil {
		return Assignment{}, fmt.Errorf("get assignment: %w", err)
	}
	return assignment, nil
}

func (s *Store) ListVisibleAssignments(ctx context.Context, role string, userID int64, page, pageSize int) (AssignmentPage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.course_id, a.title, a.description, a.deadline_at, a.late_until_at
		FROM assignments a
		JOIN courses c ON c.id = a.course_id
		WHERE $1 = 'ADMIN'
			OR ($1 = 'LECTURER' AND c.lecturer_id = $2)
			OR ($1 = 'STUDENT' AND EXISTS (
				SELECT 1 FROM enrollments e WHERE e.course_id = c.id AND e.user_id = $2
			))
		ORDER BY a.id
		LIMIT $3 OFFSET $4
	`, role, userID, pageSize+1, (page-1)*pageSize)
	if err != nil {
		return AssignmentPage{}, fmt.Errorf("list visible assignments: %w", err)
	}
	defer rows.Close()

	items := make([]Assignment, 0, pageSize+1)
	for rows.Next() {
		var assignment Assignment
		if err := rows.Scan(
			&assignment.ID, &assignment.CourseID, &assignment.Title, &assignment.Description,
			&assignment.DeadlineAt, &assignment.LateUntilAt,
		); err != nil {
			return AssignmentPage{}, fmt.Errorf("scan visible assignment: %w", err)
		}
		items = append(items, assignment)
	}
	if err := rows.Err(); err != nil {
		return AssignmentPage{}, fmt.Errorf("list visible assignment rows: %w", err)
	}
	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}
	return AssignmentPage{Items: items, Page: page, PageSize: pageSize, HasNext: hasNext}, nil
}

func (s *Store) UpdateAssignment(ctx context.Context, assignmentID int64, title, description string, deadlineAt, lateUntilAt time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE assignments
		SET title = $2, description = $3, deadline_at = $4, late_until_at = $5, updated_at = now()
		WHERE id = $1
	`, assignmentID, title, description, deadlineAt, lateUntilAt)
	if err != nil {
		return false, fmt.Errorf("update assignment: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) DeleteAssignment(ctx context.Context, assignmentID int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM assignments WHERE id = $1`, assignmentID)
	if err != nil {
		return false, fmt.Errorf("delete assignment: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) CanAccessAssignment(ctx context.Context, assignmentID, userID int64, role string) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM assignments a
			JOIN courses c ON c.id = a.course_id
			WHERE a.id = $1 AND (
				$2 = 'ADMIN'
				OR ($2 = 'LECTURER' AND c.lecturer_id = $3)
				OR ($2 = 'STUDENT' AND EXISTS (
					SELECT 1 FROM enrollments e WHERE e.course_id = c.id AND e.user_id = $3
				))
			)
		)
	`, assignmentID, role, userID).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check assignment access: %w", err)
	}
	return allowed, nil
}

func (s *Store) CanManageAssignment(ctx context.Context, assignmentID, userID int64, role string) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM assignments a
			JOIN courses c ON c.id = a.course_id
			WHERE a.id = $1 AND ($2 = 'ADMIN' OR ($2 = 'LECTURER' AND c.lecturer_id = $3))
		)
	`, assignmentID, role, userID).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check assignment management access: %w", err)
	}
	return allowed, nil
}

func (s *Store) GetResourceLimit(ctx context.Context, assignmentID int64) (ResourceLimit, error) {
	var limit ResourceLimit
	err := s.pool.QueryRow(ctx, `
		SELECT assignment_id, cpu_time_ms, memory_mb, max_processes, timeout_s,
			max_output_bytes, tmpfs_size_mb
		FROM resource_limits WHERE assignment_id = $1
	`, assignmentID).Scan(
		&limit.AssignmentID, &limit.CPUTimeMS, &limit.MemoryMB, &limit.MaxProcesses,
		&limit.TimeoutS, &limit.MaxOutputBytes, &limit.TmpfsSizeMB,
	)
	if err != nil {
		return ResourceLimit{}, fmt.Errorf("get resource limit: %w", err)
	}
	return limit, nil
}

func (s *Store) UpsertResourceLimit(ctx context.Context, limit ResourceLimit) (ResourceLimit, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO resource_limits (
			assignment_id, cpu_time_ms, memory_mb, max_processes, timeout_s, max_output_bytes, tmpfs_size_mb
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (assignment_id) DO UPDATE SET
			cpu_time_ms = EXCLUDED.cpu_time_ms,
			memory_mb = EXCLUDED.memory_mb,
			max_processes = EXCLUDED.max_processes,
			timeout_s = EXCLUDED.timeout_s,
			max_output_bytes = EXCLUDED.max_output_bytes,
			tmpfs_size_mb = EXCLUDED.tmpfs_size_mb,
			updated_at = now()
		RETURNING assignment_id, cpu_time_ms, memory_mb, max_processes, timeout_s,
			max_output_bytes, tmpfs_size_mb
	`, limit.AssignmentID, limit.CPUTimeMS, limit.MemoryMB, limit.MaxProcesses, limit.TimeoutS,
		limit.MaxOutputBytes, limit.TmpfsSizeMB).Scan(
		&limit.AssignmentID, &limit.CPUTimeMS, &limit.MemoryMB, &limit.MaxProcesses,
		&limit.TimeoutS, &limit.MaxOutputBytes, &limit.TmpfsSizeMB,
	)
	if err != nil {
		return ResourceLimit{}, fmt.Errorf("save resource limit: %w", err)
	}
	return limit, nil
}

func (s *Store) ListTestCases(ctx context.Context, assignmentID int64, includeHidden bool, page, pageSize int) (TestCasePage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, assignment_id, external_id, input, expected_output, weight, is_hidden, match_mode
		FROM test_cases
		WHERE assignment_id = $1 AND NOT is_deleted AND ($2 OR NOT is_hidden)
		ORDER BY id
		LIMIT $3 OFFSET $4
	`, assignmentID, includeHidden, pageSize+1, (page-1)*pageSize)
	if err != nil {
		return TestCasePage{}, fmt.Errorf("list test cases: %w", err)
	}
	defer rows.Close()

	items := make([]TestCase, 0, pageSize+1)
	for rows.Next() {
		var testCase TestCase
		if err := rows.Scan(
			&testCase.ID, &testCase.AssignmentID, &testCase.ExternalID, &testCase.Input,
			&testCase.ExpectedOutput, &testCase.Weight, &testCase.IsHidden, &testCase.MatchMode,
		); err != nil {
			return TestCasePage{}, fmt.Errorf("scan test case: %w", err)
		}
		items = append(items, testCase)
	}
	if err := rows.Err(); err != nil {
		return TestCasePage{}, fmt.Errorf("list test case rows: %w", err)
	}
	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}
	return TestCasePage{Items: items, Page: page, PageSize: pageSize, HasNext: hasNext}, nil
}

func (s *Store) CreateTestCase(ctx context.Context, testCase TestCase) (TestCase, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TestCase{}, fmt.Errorf("begin create test case: %w", err)
	}
	defer tx.Rollback(ctx)

	if testCase.ExternalID == "" {
		var assignmentID int64
		if err := tx.QueryRow(ctx, `SELECT id FROM assignments WHERE id = $1 FOR UPDATE`, testCase.AssignmentID).Scan(&assignmentID); err != nil {
			return TestCase{}, fmt.Errorf("lock assignment for test case: %w", err)
		}
		for nextID := int64(1); ; nextID++ {
			candidate := fmt.Sprintf("tc_%02d", nextID)
			var exists bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM test_cases WHERE assignment_id = $1 AND external_id = $2
				)
			`, testCase.AssignmentID, candidate).Scan(&exists); err != nil {
				return TestCase{}, fmt.Errorf("generate test case external identifier: %w", err)
			}
			if !exists {
				testCase.ExternalID = candidate
				break
			}
			if nextID == int64(1<<63-1) {
				return TestCase{}, fmt.Errorf("generate test case external identifier: identifier space exhausted")
			}
		}
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO test_cases (
			assignment_id, external_id, input, expected_output, weight, is_hidden, match_mode
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, testCase.AssignmentID, testCase.ExternalID, testCase.Input, testCase.ExpectedOutput,
		testCase.Weight, testCase.IsHidden, testCase.MatchMode).Scan(&testCase.ID)
	if err != nil {
		return TestCase{}, fmt.Errorf("create test case: %w", err)
	}
	if err := markAssignmentSubmissionsStale(ctx, tx, testCase.AssignmentID); err != nil {
		return TestCase{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TestCase{}, fmt.Errorf("commit create test case: %w", err)
	}
	return testCase, nil
}

func (s *Store) UpdateTestCase(ctx context.Context, testCase TestCase) (TestCase, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TestCase{}, false, fmt.Errorf("begin update test case: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		SELECT assignment_id FROM test_cases WHERE id = $1 AND NOT is_deleted FOR UPDATE
	`, testCase.ID).Scan(&testCase.AssignmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TestCase{}, false, nil
	}
	if err != nil {
		return TestCase{}, false, fmt.Errorf("lock test case for update: %w", err)
	}
	err = tx.QueryRow(ctx, `
		UPDATE test_cases
		SET external_id = COALESCE(NULLIF($2, ''), external_id),
			input = $3, expected_output = $4, weight = $5, is_hidden = $6,
			match_mode = $7, updated_at = now()
		WHERE id = $1 AND NOT is_deleted
		RETURNING id, assignment_id, external_id, input, expected_output, weight, is_hidden, match_mode
	`, testCase.ID, testCase.ExternalID, testCase.Input, testCase.ExpectedOutput, testCase.Weight,
		testCase.IsHidden, testCase.MatchMode).Scan(
		&testCase.ID, &testCase.AssignmentID, &testCase.ExternalID, &testCase.Input,
		&testCase.ExpectedOutput, &testCase.Weight, &testCase.IsHidden, &testCase.MatchMode,
	)
	if err != nil {
		return TestCase{}, false, fmt.Errorf("update test case: %w", err)
	}
	if err := markAssignmentSubmissionsStale(ctx, tx, testCase.AssignmentID); err != nil {
		return TestCase{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TestCase{}, false, fmt.Errorf("commit update test case: %w", err)
	}
	return testCase, true, nil
}

func (s *Store) DeleteTestCase(ctx context.Context, testCaseID int64) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin delete test case: %w", err)
	}
	defer tx.Rollback(ctx)

	var assignmentID int64
	err = tx.QueryRow(ctx, `
		SELECT assignment_id FROM test_cases WHERE id = $1 AND NOT is_deleted FOR UPDATE
	`, testCaseID).Scan(&assignmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock test case for deletion: %w", err)
	}
	var hasResults bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM results WHERE testcase_id = $1)`, testCaseID).Scan(&hasResults); err != nil {
		return false, fmt.Errorf("check test case results: %w", err)
	}
	if hasResults {
		_, err = tx.Exec(ctx, `
			UPDATE test_cases SET is_deleted = true, updated_at = now()
			WHERE id = $1
		`, testCaseID)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM test_cases WHERE id = $1`, testCaseID)
	}
	if err != nil {
		return false, fmt.Errorf("delete test case: %w", err)
	}
	if err := markAssignmentSubmissionsStale(ctx, tx, assignmentID); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit delete test case: %w", err)
	}
	return true, nil
}

func markAssignmentSubmissionsStale(ctx context.Context, tx pgx.Tx, assignmentID int64) error {
	if _, err := tx.Exec(ctx, `
		UPDATE submissions SET stale_grade = true, updated_at = now()
		WHERE assignment_id = $1
	`, assignmentID); err != nil {
		return fmt.Errorf("mark assignment submissions stale: %w", err)
	}
	return nil
}

func (s *Store) CanManageTestCase(ctx context.Context, testCaseID, userID int64, role string) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM test_cases t
			JOIN assignments a ON a.id = t.assignment_id
			JOIN courses c ON c.id = a.course_id
			WHERE t.id = $1 AND NOT t.is_deleted
				AND ($2 = 'ADMIN' OR ($2 = 'LECTURER' AND c.lecturer_id = $3))
		)
	`, testCaseID, role, userID).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check test case management access: %w", err)
	}
	return allowed, nil
}
