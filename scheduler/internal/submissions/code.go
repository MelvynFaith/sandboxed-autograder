package submissions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
)

// ErrCodeDeleted says the stored file was deleted by the retention job (NFR-13); the row is kept.
var ErrCodeDeleted = errors.New("submission: code deleted")

// maxCodeBytes is the upload limit (SRS 6.3): a stored file larger than this was not put there by
// the API, so it is not read into memory.
const maxCodeBytes = 256 << 10

// Code is the source of one submission (FR-20).
type Code struct {
	SubmissionID     int64  `json:"submission_id"`
	OriginalFilename string `json:"original_filename"`
	Code             string `json:"code"`
}

// Code returns the source of a submission to its owner, to the lecturer of its course and to an
// admin (FR-20, NFR-13); for anyone else it returns ErrNotFound, like GetVisible. It returns
// ErrCodeDeleted once the retention job has deleted the file. Any other error, including a stored
// path that leaves codeDir or a file that is gone although code_deleted_at is not set, is a server
// fault: its text names the path, so the caller logs it and never sends it to the client.
func (s *Store) Code(
	ctx context.Context, codeDir string, submissionID, userID int64, role string,
) (Code, error) {
	code := Code{SubmissionID: submissionID}
	var codePath string
	var deleted bool
	err := s.pool.QueryRow(ctx, `
		SELECT s.original_filename, s.code_path, s.code_deleted_at IS NOT NULL
		FROM submissions s
		JOIN assignments a ON a.id = s.assignment_id
		JOIN courses c ON c.id = a.course_id
		WHERE s.id = $1 AND (
			s.user_id = $2 OR $3 = 'ADMIN' OR ($3 = 'LECTURER' AND c.lecturer_id = $2)
		)
	`, submissionID, userID, role).Scan(&code.OriginalFilename, &codePath, &deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return Code{}, ErrNotFound
	}
	if err != nil {
		return Code{}, fmt.Errorf("submission: get code access %d: %w", submissionID, err)
	}
	if deleted {
		return Code{}, ErrCodeDeleted
	}

	// code_path is server-generated and relative to codeDir; one that is not is never followed,
	// the same rule the retention job applies before it deletes the file.
	if !filepath.IsLocal(codePath) {
		return Code{}, fmt.Errorf("submission: stored code path %q of %d leaves the code directory",
			codePath, submissionID)
	}
	file, err := os.Open(filepath.Join(codeDir, codePath))
	if err != nil {
		return Code{}, fmt.Errorf("submission: open code of %d: %w", submissionID, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCodeBytes+1))
	if err != nil {
		return Code{}, fmt.Errorf("submission: read code of %d: %w", submissionID, err)
	}
	if len(data) > maxCodeBytes {
		return Code{}, fmt.Errorf("submission: stored code of %d is larger than %d bytes",
			submissionID, maxCodeBytes)
	}
	code.Code = string(data)
	return code, nil
}
