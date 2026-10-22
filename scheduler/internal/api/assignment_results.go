package api

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/submissions"
)

var assignmentResultsCSVHeader = []string{
	"name", "email", "submission_id", "status", "total_score", "max_score",
	"is_late", "stale_grade", "submitted_at",
}

// listAssignmentResults serves the grade recap of one assignment (FR-11).
func (a *API) listAssignmentResults(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireResultsManager(w, r, user, assignmentID) {
		return
	}
	page, pageSize, ok := parsePage(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "page dan page_size harus bilangan bulat positif; page_size maksimal 100.")
		return
	}
	result, err := a.submissions.AssignmentResults(r.Context(), assignmentID, page, pageSize)
	if errors.Is(err, submissions.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "list assignment results", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// exportAssignmentResults serves the whole recap as an RFC 4180 CSV file (FR-11).
func (a *API) exportAssignmentResults(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireResultsManager(w, r, user, assignmentID) {
		return
	}
	items, err := a.submissions.AllAssignmentResults(r.Context(), assignmentID)
	if errors.Is(err, submissions.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "export assignment results", err)
		return
	}

	records := make([][]string, 0, len(items)+1)
	records = append(records, assignmentResultsCSVHeader)
	for _, item := range items {
		records = append(records, assignmentResultCSVRecord(item))
	}
	var body bytes.Buffer
	writer := csv.NewWriter(&body)
	writer.UseCRLF = true
	if err := writer.WriteAll(records); err != nil {
		internalError(w, "encode assignment results CSV", err)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="rekap-assignment-%d.csv"`, assignmentID))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body.Bytes()); err != nil {
		log.Printf("write assignment results CSV: %v", err)
	}
}

// assignmentResultCSVRecord leaves the submission cells empty for a student who has no submission,
// and the score cells empty while the latest submission is not graded.
func assignmentResultCSVRecord(item submissions.AssignmentResult) []string {
	record := make([]string, len(assignmentResultsCSVHeader))
	record[0] = item.Name
	record[1] = item.Email
	if latest := item.LatestSubmission; latest != nil {
		record[2] = strconv.FormatInt(latest.ID, 10)
		record[3] = latest.Status
		if latest.TotalScore != nil {
			record[4] = strconv.Itoa(*latest.TotalScore)
		}
		if latest.MaxScore != nil {
			record[5] = strconv.Itoa(*latest.MaxScore)
		}
		record[6] = strconv.FormatBool(latest.IsLate)
		record[7] = strconv.FormatBool(latest.StaleGrade)
		record[8] = latest.SubmittedAt.UTC().Format(time.RFC3339)
	}
	for i, cell := range record {
		record[i] = csvSafeCell(cell)
	}
	return record
}

// csvSafeCell keeps a spreadsheet from evaluating a cell as a formula (CSV injection): a name such
// as "=HYPERLINK(...)" is user-controlled. A leading apostrophe makes the spreadsheet read the cell
// as text and is not displayed.
func csvSafeCell(cell string) string {
	if cell != "" && strings.IndexByte("=+-@\t\r", cell[0]) >= 0 {
		return "'" + cell
	}
	return cell
}

// requireResultsManager admits the course's lecturer and admins. An enrolled student may see the
// assignment but not the grades of the class, so they get 403; anyone who may not see the assignment
// gets 404 (NFR-10).
func (a *API) requireResultsManager(w http.ResponseWriter, r *http.Request, user auth.User, assignmentID int64) bool {
	allowed, err := a.courses.CanManageAssignment(r.Context(), assignmentID, user.ID, user.Role)
	if err != nil {
		internalError(w, "check assignment results access", err)
		return false
	}
	if allowed {
		return true
	}
	visible, err := a.courses.CanAccessAssignment(r.Context(), assignmentID, user.ID, user.Role)
	if err != nil {
		internalError(w, "check assignment results visibility", err)
		return false
	}
	if visible {
		writeError(w, http.StatusForbidden, "forbidden", "Rekap nilai hanya untuk dosen pengampu atau administrator.")
		return false
	}
	writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
	return false
}
