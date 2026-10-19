package submissions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/ratelimit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("submission: not found")
	ErrDeadlinePassed = errors.New("submission: deadline passed")
)

type Store struct {
	pool *pgxpool.Pool
}

type Submission struct {
	ID               int64      `json:"id"`
	UserID           int64      `json:"user_id"`
	AssignmentID     int64      `json:"assignment_id"`
	OriginalFilename string     `json:"original_filename"`
	SizeBytes        int        `json:"size_bytes"`
	Status           string     `json:"status"`
	IsLate           bool       `json:"is_late"`
	StaleGrade       bool       `json:"stale_grade"`
	TotalScore       *int       `json:"total_score,omitempty"`
	MaxScore         *int       `json:"max_score,omitempty"`
	SubmittedAt      time.Time  `json:"submitted_at"`
	GradedAt         *time.Time `json:"graded_at,omitempty"`
}

type Page struct {
	Items    []Submission `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	HasNext  bool         `json:"has_next"`
}

type Report struct {
	SubmissionID int64         `json:"submission_id"`
	TotalScore   *int          `json:"total_score,omitempty"`
	MaxScore     *int          `json:"max_score,omitempty"`
	Results      []ReportEntry `json:"results"`
}

type ReportEntry struct {
	TestCaseID   *int64  `json:"testcase_id,omitempty"`
	ExternalID   *string `json:"external_id,omitempty"`
	Status       string  `json:"status"`
	Score        int     `json:"score"`
	ActualOutput *string `json:"actual_output,omitempty"`
	ErrorMessage *string `json:"error_message,omitempty"`
	IsTruncated  *bool   `json:"is_truncated,omitempty"`
	ExecTimeMS   int     `json:"exec_time_ms"`
	MemoryKB     int     `json:"memory_kb"`
	MemorySource *string `json:"memory_source,omitempty"`
	Input        *string `json:"input,omitempty"`
	Expected     *string `json:"expected_output,omitempty"`
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Create(
	ctx context.Context,
	userID, assignmentID int64,
	filename, codePath string,
	sizeBytes int,
	limits ratelimit.Limits,
) (Submission, error) {
	return s.create(ctx, userID, assignmentID, filename, codePath, sizeBytes, time.Time{}, limits)
}

func (s *Store) CreateAt(
	ctx context.Context,
	userID, assignmentID int64,
	filename, codePath string,
	sizeBytes int,
	submittedAt time.Time,
	limits ratelimit.Limits,
) (Submission, error) {
	return s.create(ctx, userID, assignmentID, filename, codePath, sizeBytes, submittedAt, limits)
}

func (s *Store) create(
	ctx context.Context,
	userID, assignmentID int64,
	filename, codePath string,
	sizeBytes int,
	submittedAt time.Time,
	limits ratelimit.Limits,
) (Submission, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Submission{}, fmt.Errorf("submission: begin create: %w", err)
	}
	defer tx.Rollback(ctx)

	var deadlineAt, lateUntilAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT a.deadline_at, a.late_until_at
		FROM assignments a
		WHERE a.id = $1 AND EXISTS (
			SELECT 1 FROM enrollments e WHERE e.course_id = a.course_id AND e.user_id = $2
		)
	`, assignmentID, userID).Scan(&deadlineAt, &lateUntilAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrNotFound
	}
	if err != nil {
		return Submission{}, fmt.Errorf("submission: check assignment access: %w", err)
	}
	useProvidedTime := !submittedAt.IsZero()
	if !useProvidedTime {
		submittedAt = time.Now().UTC()
	}
	if submittedAt.After(lateUntilAt) {
		return Submission{}, ErrDeadlinePassed
	}
	if err := limits.Check(ctx, tx, userID, assignmentID); err != nil {
		return Submission{}, err
	}
	if !useProvidedTime {
		submittedAt = time.Now().UTC()
	}
	if submittedAt.After(lateUntilAt) {
		return Submission{}, ErrDeadlinePassed
	}
	isLate := submittedAt.After(deadlineAt)

	var submission Submission
	err = tx.QueryRow(ctx, `
		INSERT INTO submissions (
			user_id, assignment_id, code_path, original_filename, size_bytes, status, is_late, submitted_at
		) VALUES ($1, $2, $3, $4, $5, 'QUEUED', $6, $7)
		RETURNING id, user_id, assignment_id, original_filename, size_bytes, status, is_late,
			stale_grade, total_score, max_score, submitted_at, graded_at
	`, userID, assignmentID, codePath, filename, sizeBytes, isLate, submittedAt).Scan(
		&submission.ID, &submission.UserID, &submission.AssignmentID, &submission.OriginalFilename,
		&submission.SizeBytes, &submission.Status, &submission.IsLate, &submission.StaleGrade,
		&submission.TotalScore, &submission.MaxScore, &submission.SubmittedAt, &submission.GradedAt,
	)
	if err != nil {
		return Submission{}, fmt.Errorf("submission: insert: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Submission{}, fmt.Errorf("submission: commit create: %w", err)
	}
	return submission, nil
}

func (s *Store) DeleteQueued(ctx context.Context, submissionID int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM submissions WHERE id = $1 AND status = 'QUEUED'`, submissionID)
	if err != nil {
		return false, fmt.Errorf("submission: delete queued %d: %w", submissionID, err)
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) ListByUser(ctx context.Context, userID int64, page, pageSize int) (Page, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, assignment_id, original_filename, size_bytes, status, is_late,
			stale_grade, total_score, max_score, submitted_at, graded_at
		FROM submissions WHERE user_id = $1
		ORDER BY submitted_at DESC, id DESC LIMIT $2 OFFSET $3
	`, userID, pageSize+1, (page-1)*pageSize)
	if err != nil {
		return Page{}, fmt.Errorf("submission: list: %w", err)
	}
	defer rows.Close()

	items := make([]Submission, 0, pageSize+1)
	for rows.Next() {
		var submission Submission
		if err := rows.Scan(
			&submission.ID, &submission.UserID, &submission.AssignmentID, &submission.OriginalFilename,
			&submission.SizeBytes, &submission.Status, &submission.IsLate, &submission.StaleGrade,
			&submission.TotalScore, &submission.MaxScore, &submission.SubmittedAt, &submission.GradedAt,
		); err != nil {
			return Page{}, fmt.Errorf("submission: scan list row: %w", err)
		}
		items = append(items, submission)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("submission: list rows: %w", err)
	}
	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}
	return Page{Items: items, Page: page, PageSize: pageSize, HasNext: hasNext}, nil
}

func (s *Store) GetVisible(ctx context.Context, submissionID, userID int64, role string) (Submission, error) {
	var submission Submission
	err := s.pool.QueryRow(ctx, `
		SELECT s.id, s.user_id, s.assignment_id, s.original_filename, s.size_bytes, s.status, s.is_late,
			s.stale_grade, s.total_score, s.max_score, s.submitted_at, s.graded_at
		FROM submissions s
		JOIN assignments a ON a.id = s.assignment_id
		JOIN courses c ON c.id = a.course_id
		WHERE s.id = $1 AND (
			s.user_id = $2 OR $3 = 'ADMIN' OR ($3 = 'LECTURER' AND c.lecturer_id = $2)
		)
	`, submissionID, userID, role).Scan(
		&submission.ID, &submission.UserID, &submission.AssignmentID, &submission.OriginalFilename,
		&submission.SizeBytes, &submission.Status, &submission.IsLate, &submission.StaleGrade,
		&submission.TotalScore, &submission.MaxScore, &submission.SubmittedAt, &submission.GradedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrNotFound
	}
	if err != nil {
		return Submission{}, fmt.Errorf("submission: get visible %d: %w", submissionID, err)
	}
	return submission, nil
}

func (s *Store) Report(ctx context.Context, submission Submission, role string) (Report, error) {
	report := Report{
		SubmissionID: submission.ID,
		TotalScore:   submission.TotalScore,
		MaxScore:     submission.MaxScore,
		Results:      make([]ReportEntry, 0),
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.testcase_id, tc.external_id, tc.is_hidden, r.status, r.score, r.actual_output,
			r.error_message, r.is_truncated, r.exec_time_ms, r.memory_kb, r.memory_source,
			tc.input, tc.expected_output
		FROM results r
		JOIN test_cases tc ON tc.id = r.testcase_id
		WHERE r.submission_id = $1
		ORDER BY tc.id
	`, submission.ID)
	if err != nil {
		return Report{}, fmt.Errorf("submission: read report %d: %w", submission.ID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var testcaseID int64
		var externalID string
		var hidden bool
		var entry ReportEntry
		var actualOutput, errorMessage *string
		var input, expected string
		var isTruncated bool
		var memorySource string
		if err := rows.Scan(
			&testcaseID, &externalID, &hidden, &entry.Status, &entry.Score, &actualOutput,
			&errorMessage, &isTruncated, &entry.ExecTimeMS, &entry.MemoryKB, &memorySource,
			&input, &expected,
		); err != nil {
			return Report{}, fmt.Errorf("submission: scan report row: %w", err)
		}
		if role == "STUDENT" && hidden {
			report.Results = append(report.Results, entry)
			continue
		}
		entry.TestCaseID = &testcaseID
		entry.ExternalID = &externalID
		entry.ActualOutput = actualOutput
		entry.ErrorMessage = errorMessage
		entry.IsTruncated = &isTruncated
		entry.MemorySource = &memorySource
		entry.Input = &input
		entry.Expected = &expected
		report.Results = append(report.Results, entry)
	}
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("submission: report rows: %w", err)
	}
	return report, nil
}
