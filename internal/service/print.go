// Package service contains the logic that writes the printer flag to finger_unit.
// It does not print and does not mark rows as 'Sudah'; the legacy print server does that.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const unitID = "Pendaftaran"

const selectBase = `SELECT fu.Kode, fu.printer, DATE_FORMAT(fu.TanggalFingger, '%Y-%m-%d') AS tanggal, p.tgl_lahir
FROM finger_unit fu
LEFT JOIN patients p ON p.id = fu.NoPasien
WHERE fu.Kode = ?
  AND fu.UnitID = 'Pendaftaran'
  AND DATE(fu.TanggalFingger) = CURDATE()`

const updateSQL = `UPDATE finger_unit
SET printer = 'medium'
WHERE Kode = ?
  AND UnitID = 'Pendaftaran'
  AND DATE(TanggalFingger) = CURDATE()
LIMIT 1`

// Error is a domain error that can be mapped to an HTTP response.
// Cause is never sent to the client; it is only used for server-side logging.
type Error struct {
	Status  int
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string { return e.Message }

func apiError(status int, code, format string, args ...any) *Error {
	return &Error{
		Status:  status,
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

func dbError(action string, err error) *Error {
	e := apiError(httpStatusInternal, codeDatabase, "database error while %s", action)
	e.Cause = err
	return e
}

const (
	httpStatusBadRequest = 400
	httpStatusNotFound   = 404
	httpStatusConflict   = 409
	httpStatusInternal   = 500
	codeInvalidRequest   = "INVALID_REQUEST"
	codeQueueNotFound    = "QUEUE_NOT_FOUND"
	codeNotEligible      = "NOT_ELIGIBLE"
	codeAlreadyRequested = "ALREADY_REQUESTED"
	codeAlreadyPrinted   = "ALREADY_PRINTED"
	codeDatabase         = "DATABASE_ERROR"
)

// PrintResult describes the result of registering a print request.
type PrintResult struct {
	Kode        string `json:"kode"`
	UnitID      string `json:"unit_id"`
	Tanggal     string `json:"tanggal"`
	Printer     string `json:"printer"`
	RowsChanged int64  `json:"rows_terubah"`
	DryRun      bool   `json:"dry_run,omitempty"`
	Force       bool   `json:"force,omitempty"`
}

// StatusResult summarizes the current flag for one queue code.
type StatusResult struct {
	Kode           string `json:"kode"`
	UnitID         string `json:"unit_id"`
	Tanggal        string `json:"tanggal"`
	Printer        string `json:"printer"`
	BirthDateValid bool   `json:"tgl_lahir_valid"`
}

type queueRow struct {
	code      string
	printer   string
	date      string
	birthDate sql.NullString
}

func (r queueRow) birthDateValid() bool {
	if !r.birthDate.Valid {
		return false
	}
	value := strings.TrimSpace(r.birthDate.String)
	return value != "" && value != "0000-00-00" && value != "0000-00-00 00:00:00"
}

// Printer holds the database dependency.
type Printer struct {
	db *sql.DB
}

// New creates a service with an already opened connection.
func New(db *sql.DB) *Printer {
	return &Printer{db: db}
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Register validates the row and writes printer='medium' in one transaction.
// SELECT ... FOR UPDATE prevents two concurrent requests from printing twice.
// Force=true skips only the already-requested/already-printed guards; it is
// intended for controlled reprints during testing.
func (p *Printer) Register(ctx context.Context, rawCode string, force bool) (*PrintResult, error) {
	code, e := normalizeCode(rawCode)
	if e != nil {
		return nil, e
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, dbError("begin transaction", err)
	}
	defer func() { _ = tx.Rollback() }()

	row, err := fetchRow(ctx, tx, code, true)
	if err != nil {
		return nil, err
	}
	if e := validateRow(code, row, force); e != nil {
		return nil, e
	}

	result, err := tx.ExecContext(ctx, updateSQL, code)
	if err != nil {
		return nil, dbError("UPDATE printer", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, dbError("read RowsAffected", err)
	}
	if affected != 1 {
		return nil, apiError(httpStatusInternal, codeDatabase,
			"UPDATE affected %d rows, expected exactly 1", affected)
	}

	if err := tx.Commit(); err != nil {
		return nil, dbError("commit transaction", err)
	}

	return &PrintResult{
		Kode:        code,
		UnitID:      unitID,
		Tanggal:     row.date,
		Printer:     "medium",
		RowsChanged: affected,
		Force:       force,
	}, nil
}

// Validate performs the same checks as Register without running UPDATE.
// It is used by --dry-run mode.
func (p *Printer) Validate(ctx context.Context, rawCode string, force bool) (*PrintResult, error) {
	code, e := normalizeCode(rawCode)
	if e != nil {
		return nil, e
	}

	row, err := fetchRow(ctx, p.db, code, false)
	if err != nil {
		return nil, err
	}
	if e := validateRow(code, row, force); e != nil {
		return nil, e
	}

	return &PrintResult{
		Kode:        code,
		UnitID:      unitID,
		Tanggal:     row.date,
		Printer:     row.printer,
		RowsChanged: 0,
		DryRun:      true,
		Force:       force,
	}, nil
}

// Status returns the current flag without changing data.
func (p *Printer) Status(ctx context.Context, rawCode string) (*StatusResult, error) {
	code, e := normalizeCode(rawCode)
	if e != nil {
		return nil, e
	}

	row, err := fetchRow(ctx, p.db, code, false)
	if err != nil {
		return nil, err
	}

	return &StatusResult{
		Kode:           code,
		UnitID:         unitID,
		Tanggal:        row.date,
		Printer:        row.printer,
		BirthDateValid: row.birthDateValid(),
	}, nil
}

func normalizeCode(rawCode string) (string, *Error) {
	code := strings.TrimSpace(rawCode)
	if code == "" {
		return "", apiError(httpStatusBadRequest, codeInvalidRequest, "kode is required")
	}
	if len(code) > 50 {
		return "", apiError(httpStatusBadRequest, codeInvalidRequest, "kode is too long (max 50 characters)")
	}
	return code, nil
}

func selectSQL(forUpdate bool) string {
	query := selectBase + "\nLIMIT 1"
	if forUpdate {
		query += " FOR UPDATE"
	}
	return query
}

func fetchRow(ctx context.Context, q queryRower, code string, forUpdate bool) (queueRow, error) {
	var row queueRow
	err := q.QueryRowContext(ctx, selectSQL(forUpdate), code).
		Scan(&row.code, &row.printer, &row.date, &row.birthDate)
	if errors.Is(err, sql.ErrNoRows) {
		return row, apiError(httpStatusNotFound, codeQueueNotFound,
			"queue %s for %s today was not found", code, unitID)
	}
	if err != nil {
		return row, dbError("SELECT queue row", err)
	}
	return row, nil
}

func validateRow(code string, row queueRow, force bool) *Error {
	if !row.birthDateValid() {
		return apiError(httpStatusConflict, codeNotEligible,
			"tgl_lahir is invalid; the print server will skip queue %s", code)
	}
	if force {
		return nil
	}
	if strings.EqualFold(row.printer, "medium") {
		return apiError(httpStatusConflict, codeAlreadyRequested,
			"queue %s already has a pending print request", code)
	}
	if strings.EqualFold(row.printer, "sudah") {
		return apiError(httpStatusConflict, codeAlreadyPrinted,
			"queue %s has already been printed", code)
	}
	return nil
}
