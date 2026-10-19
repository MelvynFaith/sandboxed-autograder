// Package retention deletes the stored files of old submissions (NFR-13, SRS 13.4).
//
// A submission's file is deleted once its assignment has been closed for the retention period,
// closed meaning past late_until_at, the last moment a submission is accepted. The row stays: its
// grade, results and audit entries are kept for the records, and submissions.code_deleted_at says
// the file is gone. A submission that is QUEUED or RUNNING keeps its file, so a regrade in progress
// is not broken.
package retention

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// batchSize is how many submissions one transaction handles, so a first run after a long time does
// not hold thousands of row locks at once.
var batchSize = 500

// Report counts what Expire did with each submission it found.
type Report struct {
	Deleted int // file deleted
	Missing int // file already gone; the row is marked all the same
	Failed  int // file not deleted, and the row left unmarked for the next run
}

// due selects the submissions whose file is to go, past a submission id so that each batch moves on
// even when some files fail.
const due = `
	FROM submissions s JOIN assignments a ON a.id = s.assignment_id
	WHERE a.late_until_at < $1 AND s.code_deleted_at IS NULL AND s.status NOT IN ('QUEUED', 'RUNNING')`

// Pending counts the submissions whose file Expire would delete, without changing anything.
func Pending(ctx context.Context, pool *pgxpool.Pool, closedBefore time.Time) (int, error) {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*)`+due, closedBefore).Scan(&n); err != nil {
		return 0, fmt.Errorf("retention: count due submissions: %w", err)
	}
	return n, nil
}

// Expire deletes the file of every submission whose assignment closed before closedBefore, under
// codeDir, and sets code_deleted_at on its row. A file that is already gone is treated as deleted
// and counted as Missing. A file that cannot be deleted, or whose stored path would leave codeDir,
// keeps its row unmarked; Expire goes on with the others and returns those errors joined at the end.
//
// Each row is locked while its file is deleted and the row marked, so a regrade cannot queue the
// submission in between. A row locked by someone else is skipped until the next run.
func Expire(ctx context.Context, pool *pgxpool.Pool, codeDir string,
	closedBefore time.Time) (Report, error) {
	var r Report
	var failures []error
	var after int64
	for {
		n, err := expireBatch(ctx, pool, codeDir, closedBefore, &after, &r, &failures)
		if err != nil {
			return r, err
		}
		if n < batchSize {
			return r, errors.Join(failures...)
		}
	}
}

func expireBatch(ctx context.Context, pool *pgxpool.Pool, codeDir string, closedBefore time.Time,
	after *int64, r *Report, failures *[]error) (int, error) {
	var n int
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT s.id, s.code_path`+due+` AND s.id > $2
			ORDER BY s.id LIMIT $3 FOR UPDATE OF s SKIP LOCKED`, closedBefore, *after, batchSize)
		if err != nil {
			return err
		}
		type stored struct {
			ID   int64
			Path string
		}
		batch, err := pgx.CollectRows(rows, pgx.RowToStructByPos[stored])
		if err != nil {
			return err
		}
		n = len(batch)

		gone := make([]int64, 0, len(batch))
		for _, s := range batch {
			*after = s.ID
			switch err := remove(codeDir, s.Path); {
			case err == nil:
				r.Deleted++
			case errors.Is(err, fs.ErrNotExist):
				r.Missing++
			default:
				r.Failed++
				*failures = append(*failures, fmt.Errorf("retention: submission %d: %w", s.ID, err))
				continue
			}
			gone = append(gone, s.ID)
		}
		_, err = tx.Exec(ctx, `UPDATE submissions SET code_deleted_at = now() WHERE id = ANY($1)`, gone)
		return err
	})
	if err != nil {
		return n, fmt.Errorf("retention: expire submission files: %w", err)
	}
	return n, nil
}

// remove deletes a stored file. code_path is server-generated and relative to codeDir; one that is
// not is never followed.
func remove(codeDir, codePath string) error {
	if !filepath.IsLocal(codePath) {
		return fmt.Errorf("stored code path %q leaves the code directory", codePath)
	}
	return os.Remove(filepath.Join(codeDir, codePath))
}
