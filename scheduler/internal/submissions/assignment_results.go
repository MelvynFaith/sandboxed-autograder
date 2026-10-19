package submissions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// AssignmentResult is one row of an assignment's grade recap (FR-11): an enrolled user, how many
// submissions they made, and the one that counts.
type AssignmentResult struct {
	UserID           int64             `json:"user_id"`
	Name             string            `json:"name"`
	Email            string            `json:"email"`
	SubmissionCount  int               `json:"submission_count"`
	LatestSubmission *LatestSubmission `json:"latest_submission"`
}

// LatestSubmission is the newest submission of a user by submitted_at. Cancelled submissions are
// never graded, so they are neither the latest one nor counted in SubmissionCount.
type LatestSubmission struct {
	ID          int64     `json:"id"`
	Status      string    `json:"status"`
	TotalScore  *int      `json:"total_score,omitempty"`
	MaxScore    *int      `json:"max_score,omitempty"`
	IsLate      bool      `json:"is_late"`
	StaleGrade  bool      `json:"stale_grade"`
	SubmittedAt time.Time `json:"submitted_at"`
}

type AssignmentResultPage struct {
	AssignmentID    int64              `json:"assignment_id"`
	AssignmentTitle string             `json:"assignment_title"`
	Items           []AssignmentResult `json:"items"`
	Page            int                `json:"page"`
	PageSize        int                `json:"page_size"`
	HasNext         bool               `json:"has_next"`
}

// AssignmentResults returns one page of the recap of an assignment, one item per enrollment of its
// course ordered by name and then user id, including users who never submitted.
func (s *Store) AssignmentResults(ctx context.Context, assignmentID int64, page, pageSize int) (AssignmentResultPage, error) {
	title, err := s.assignmentTitle(ctx, assignmentID)
	if err != nil {
		return AssignmentResultPage{}, err
	}
	limit := pageSize + 1
	items, err := s.assignmentResults(ctx, assignmentID, &limit, (page-1)*pageSize)
	if err != nil {
		return AssignmentResultPage{}, err
	}
	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}
	return AssignmentResultPage{
		AssignmentID: assignmentID, AssignmentTitle: title,
		Items: items, Page: page, PageSize: pageSize, HasNext: hasNext,
	}, nil
}

// AllAssignmentResults returns every row of the recap, in the same order as AssignmentResults, for
// the CSV export. A class has up to 100 students (SRS 2.5), so the rows fit in memory, and a database
// error can still be answered with a proper status instead of a truncated file.
func (s *Store) AllAssignmentResults(ctx context.Context, assignmentID int64) ([]AssignmentResult, error) {
	if _, err := s.assignmentTitle(ctx, assignmentID); err != nil {
		return nil, err
	}
	return s.assignmentResults(ctx, assignmentID, nil, 0)
}

func (s *Store) assignmentTitle(ctx context.Context, assignmentID int64) (string, error) {
	var title string
	err := s.pool.QueryRow(ctx, `SELECT title FROM assignments WHERE id = $1`, assignmentID).Scan(&title)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("submission: get assignment %d title: %w", assignmentID, err)
	}
	return title, nil
}

// assignmentResults reads the recap rows. A nil limit reads all of them (LIMIT NULL is LIMIT ALL).
// The lateral subqueries use submissions_user_assignment_idx (user_id, assignment_id, submitted_at).
func (s *Store) assignmentResults(ctx context.Context, assignmentID int64, limit *int, offset int) ([]AssignmentResult, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.name, u.email, counted.n,
			latest.id, latest.status, latest.total_score, latest.max_score,
			latest.is_late, latest.stale_grade, latest.submitted_at
		FROM assignments a
		JOIN enrollments e ON e.course_id = a.course_id
		JOIN users u ON u.id = e.user_id
		CROSS JOIN LATERAL (
			SELECT count(*) AS n FROM submissions s
			WHERE s.user_id = u.id AND s.assignment_id = a.id AND s.status <> 'CANCELLED'
		) counted
		LEFT JOIN LATERAL (
			SELECT s.id, s.status, s.total_score, s.max_score, s.is_late, s.stale_grade, s.submitted_at
			FROM submissions s
			WHERE s.user_id = u.id AND s.assignment_id = a.id AND s.status <> 'CANCELLED'
			ORDER BY s.submitted_at DESC, s.id DESC
			LIMIT 1
		) latest ON true
		WHERE a.id = $1
		ORDER BY u.name, u.id
		LIMIT $2 OFFSET $3
	`, assignmentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("submission: list assignment %d results: %w", assignmentID, err)
	}
	defer rows.Close()

	items := make([]AssignmentResult, 0)
	for rows.Next() {
		var item AssignmentResult
		var latestID *int64
		var status *string
		var totalScore, maxScore *int
		var isLate, staleGrade *bool
		var submittedAt *time.Time
		if err := rows.Scan(
			&item.UserID, &item.Name, &item.Email, &item.SubmissionCount,
			&latestID, &status, &totalScore, &maxScore, &isLate, &staleGrade, &submittedAt,
		); err != nil {
			return nil, fmt.Errorf("submission: scan assignment result row: %w", err)
		}
		if latestID != nil {
			item.LatestSubmission = &LatestSubmission{
				ID: *latestID, Status: *status, TotalScore: totalScore, MaxScore: maxScore,
				IsLate: *isLate, StaleGrade: *staleGrade, SubmittedAt: submittedAt.UTC(),
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("submission: assignment result rows: %w", err)
	}
	return items, nil
}
