package courses

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TestCaseExport is what the YAML export of an assignment (SRS 7.3) is made of.
type TestCaseExport struct {
	AssignmentTitle string
	ResourceLimit   ResourceLimit
	TestCases       []TestCase
}

// TestCaseImportResult counts what an import changed. A test case that the file repeats unchanged
// is neither created nor updated.
type TestCaseImportResult struct {
	Created int
	Updated int
}

// ExportTestCases reads the title, the resource limit and the test cases that are not deleted, in
// the order they were created. All three come from one snapshot, so a concurrent change cannot give
// a file that mixes two states. Hidden test cases are included: the caller must be a manager.
func (s *Store) ExportTestCases(ctx context.Context, assignmentID int64) (TestCaseExport, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return TestCaseExport{}, fmt.Errorf("begin export test cases: %w", err)
	}
	defer tx.Rollback(ctx)

	var export TestCaseExport
	limit := &export.ResourceLimit
	err = tx.QueryRow(ctx, `
		SELECT a.title, r.assignment_id, r.cpu_time_ms, r.memory_mb, r.max_processes, r.timeout_s,
			r.max_output_bytes, r.tmpfs_size_mb
		FROM assignments a
		JOIN resource_limits r ON r.assignment_id = a.id
		WHERE a.id = $1
	`, assignmentID).Scan(
		&export.AssignmentTitle, &limit.AssignmentID, &limit.CPUTimeMS, &limit.MemoryMB,
		&limit.MaxProcesses, &limit.TimeoutS, &limit.MaxOutputBytes, &limit.TmpfsSizeMB,
	)
	if err != nil {
		return TestCaseExport{}, fmt.Errorf("get assignment for export: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT id, assignment_id, external_id, input, expected_output, weight, is_hidden, match_mode
		FROM test_cases
		WHERE assignment_id = $1 AND NOT is_deleted
		ORDER BY id
	`, assignmentID)
	if err != nil {
		return TestCaseExport{}, fmt.Errorf("list test cases for export: %w", err)
	}
	defer rows.Close()

	export.TestCases = make([]TestCase, 0)
	for rows.Next() {
		var testCase TestCase
		if err := rows.Scan(
			&testCase.ID, &testCase.AssignmentID, &testCase.ExternalID, &testCase.Input,
			&testCase.ExpectedOutput, &testCase.Weight, &testCase.IsHidden, &testCase.MatchMode,
		); err != nil {
			return TestCaseExport{}, fmt.Errorf("scan test case for export: %w", err)
		}
		export.TestCases = append(export.TestCases, testCase)
	}
	if err := rows.Err(); err != nil {
		return TestCaseExport{}, fmt.Errorf("export test case rows: %w", err)
	}
	return export, nil
}

// ImportTestCases applies an imported document to an assignment in one transaction: the resource
// limit when limit is not nil, then every test case. A test case is matched to the one of the
// assignment with the same external_id (SRS 7.3): it is updated when its content differs, created
// when there is none. A matching test case that was soft deleted is brought back, which keeps the
// results already recorded against it. Test cases the file does not mention are left alone.
//
// Like every other test case change, a created or updated test case marks the submissions of the
// assignment stale (SRS 7.1). Nothing is written if any statement fails.
func (s *Store) ImportTestCases(ctx context.Context, assignmentID int64, limit *ResourceLimit, testCases []TestCase) (TestCaseImportResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TestCaseImportResult{}, fmt.Errorf("begin import test cases: %w", err)
	}
	defer tx.Rollback(ctx)

	// The lock serialises imports of the same assignment, so two of them cannot both decide to
	// create the same external_id.
	var lockedID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM assignments WHERE id = $1 FOR UPDATE`, assignmentID).Scan(&lockedID); err != nil {
		return TestCaseImportResult{}, fmt.Errorf("lock assignment for import: %w", err)
	}
	if limit != nil {
		limit.AssignmentID = assignmentID
		if _, err := updateResourceLimit(ctx, tx, *limit); err != nil {
			return TestCaseImportResult{}, err
		}
	}

	type storedTestCase struct {
		TestCase
		isDeleted bool
	}
	stored := make(map[string]storedTestCase)
	rows, err := tx.Query(ctx, `
		SELECT id, external_id, input, expected_output, weight, is_hidden, match_mode, is_deleted
		FROM test_cases WHERE assignment_id = $1
	`, assignmentID)
	if err != nil {
		return TestCaseImportResult{}, fmt.Errorf("list test cases for import: %w", err)
	}
	for rows.Next() {
		var testCase storedTestCase
		if err := rows.Scan(
			&testCase.ID, &testCase.ExternalID, &testCase.Input, &testCase.ExpectedOutput,
			&testCase.Weight, &testCase.IsHidden, &testCase.MatchMode, &testCase.isDeleted,
		); err != nil {
			rows.Close()
			return TestCaseImportResult{}, fmt.Errorf("scan test case for import: %w", err)
		}
		stored[testCase.ExternalID] = testCase
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return TestCaseImportResult{}, fmt.Errorf("import test case rows: %w", err)
	}

	var result TestCaseImportResult
	batch := &pgx.Batch{}
	for _, testCase := range testCases {
		current, exists := stored[testCase.ExternalID]
		switch {
		case !exists:
			batch.Queue(`
				INSERT INTO test_cases (
					assignment_id, external_id, input, expected_output, weight, is_hidden, match_mode
				) VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, assignmentID, testCase.ExternalID, testCase.Input, testCase.ExpectedOutput,
				testCase.Weight, testCase.IsHidden, testCase.MatchMode)
			result.Created++
		case current.isDeleted || current.Input != testCase.Input ||
			current.ExpectedOutput != testCase.ExpectedOutput || current.Weight != testCase.Weight ||
			current.IsHidden != testCase.IsHidden || current.MatchMode != testCase.MatchMode:
			batch.Queue(`
				UPDATE test_cases
				SET input = $2, expected_output = $3, weight = $4, is_hidden = $5, match_mode = $6,
					is_deleted = false, updated_at = now()
				WHERE id = $1
			`, current.ID, testCase.Input, testCase.ExpectedOutput, testCase.Weight,
				testCase.IsHidden, testCase.MatchMode)
			result.Updated++
		}
	}
	if batch.Len() > 0 {
		results := tx.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, err := results.Exec(); err != nil {
				results.Close()
				return TestCaseImportResult{}, fmt.Errorf("write imported test case: %w", err)
			}
		}
		if err := results.Close(); err != nil {
			return TestCaseImportResult{}, fmt.Errorf("close imported test case batch: %w", err)
		}
		if err := markAssignmentSubmissionsStale(ctx, tx, assignmentID); err != nil {
			return TestCaseImportResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return TestCaseImportResult{}, fmt.Errorf("commit import test cases: %w", err)
	}
	return result, nil
}
