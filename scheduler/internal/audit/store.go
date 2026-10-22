// Package audit reads the audit log of FR-12 for the administrator (US-18).
//
// Rows are written where the events happen, each in the same transaction as the change it records:
// the grading pipeline (internal/store) logs sandbox violations, with the denied syscall number when
// the sandbox reported one, and exceeded resource limits; internal/auth logs role changes and denied
// role changes; internal/settings logs changed rate limits; internal/seed logs the first admin. The
// table is append-only, so this package only reads it.
package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry is one audit log row. SubmissionID and SyscallNr are null, not omitted, when the event has
// none: a role change belongs to no submission, and only a denied syscall has a number.
type Entry struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	SubmissionID *int64    `json:"submission_id"`
	EventType    string    `json:"event_type"`
	SyscallNr    *int      `json:"syscall_nr"`
	Detail       string    `json:"detail"`
	CreatedAt    time.Time `json:"created_at"`
}

// Page is one page of the log, in the list format of SRS 6.3.
type Page struct {
	Items    []Entry `json:"items"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
	HasNext  bool    `json:"has_next"`
}

// Store reads the audit log. It is safe for concurrent use.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store that uses pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// List returns page (from 1) of the log, pageSize rows at most, newest first. Rows written in one
// transaction share created_at, so the id breaks the tie and keeps the order stable from one page to
// the next. The ORDER BY matches audit_logs_created_at_idx, which serves the query.
func (s *Store) List(ctx context.Context, page, pageSize int) (Page, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, submission_id, event_type, syscall_nr, detail, created_at
		FROM audit_logs
		ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2`, pageSize+1, (page-1)*pageSize)
	if err != nil {
		return Page{}, fmt.Errorf("audit: list: %w", err)
	}
	defer rows.Close()

	// One row more than the page shows whether another page follows.
	items := make([]Entry, 0, pageSize+1)
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.UserID, &e.SubmissionID, &e.EventType, &e.SyscallNr,
			&e.Detail, &e.CreatedAt); err != nil {
			return Page{}, fmt.Errorf("audit: scan: %w", err)
		}
		e.CreatedAt = e.CreatedAt.UTC()
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("audit: list: %w", err)
	}
	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}
	return Page{Items: items, Page: page, PageSize: pageSize, HasNext: hasNext}, nil
}
