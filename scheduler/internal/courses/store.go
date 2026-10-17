package courses

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrNotStudent   = errors.New("user is not a student")
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

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateCourse(ctx context.Context, name, code, term string, lecturerID int64) (Course, error) {
	var course Course
	err := s.pool.QueryRow(ctx, `
		INSERT INTO courses (name, code, lecturer_id, term)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, code, term, lecturer_id
	`, name, code, lecturerID, term).Scan(
		&course.ID, &course.Name, &course.Code, &course.Term, &course.LecturerID,
	)
	if err != nil {
		return Course{}, fmt.Errorf("create course: %w", err)
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
			SELECT id, name, code, term, lecturer_id
			FROM courses
			WHERE lecturer_id = $1
			ORDER BY id
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
			SELECT 1 FROM courses
			WHERE id = $1 AND ($2 = 'ADMIN' OR lecturer_id = $3)
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
