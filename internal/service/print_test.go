package service

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func newMockService(t *testing.T) (*Printer, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	return New(database), mock, database
}

func newQueueRows(printer, birthDate any) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"Kode", "printer", "tanggal", "tgl_lahir"}).
		AddRow("B001", printer, "2026-11-27", birthDate)
}

func assertError(t *testing.T, err error, status int, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", code)
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *Error: %T (%v)", err, err)
	}
	if apiErr.Status != status || apiErr.Code != code {
		t.Fatalf("error = (%d, %s), want (%d, %s)", apiErr.Status, apiErr.Code, status, code)
	}
}

func TestRegisterSuccess(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(true))).
		WithArgs("B001").
		WillReturnRows(newQueueRows("menunggu", "1990-01-01"))
	mock.ExpectExec(regexp.QuoteMeta(updateSQL)).
		WithArgs("B001").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := svc.Register(context.Background(), "  B001  ", false)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got.Kode != "B001" || got.UnitID != "Pendaftaran" || got.Printer != "medium" || got.RowsChanged != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.DryRun {
		t.Fatal("Register must not set dry_run")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterNotFound(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(true))).
		WithArgs("B999").
		WillReturnRows(sqlmock.NewRows([]string{"Kode", "printer", "tanggal", "tgl_lahir"}))
	mock.ExpectRollback()

	_, err := svc.Register(context.Background(), "B999", false)
	assertError(t, err, 404, "QUEUE_NOT_FOUND")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterRejectsZeroBirthDate(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(true))).
		WithArgs("B001").
		WillReturnRows(newQueueRows("menunggu", "0000-00-00"))
	mock.ExpectRollback()

	_, err := svc.Register(context.Background(), "B001", false)
	assertError(t, err, 409, "NOT_ELIGIBLE")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterRejectsNullBirthDate(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(true))).
		WithArgs("B001").
		WillReturnRows(newQueueRows("menunggu", nil))
	mock.ExpectRollback()

	_, err := svc.Register(context.Background(), "B001", false)
	assertError(t, err, 409, "NOT_ELIGIBLE")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterRejectsAlreadyMedium(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(true))).
		WithArgs("B001").
		WillReturnRows(newQueueRows("medium", "1990-01-01"))
	mock.ExpectRollback()

	_, err := svc.Register(context.Background(), "B001", false)
	assertError(t, err, 409, "ALREADY_REQUESTED")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterRejectsAlreadyPrinted(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(true))).
		WithArgs("B001").
		WillReturnRows(newQueueRows("sudah", "1990-01-01"))
	mock.ExpectRollback()

	_, err := svc.Register(context.Background(), "B001", false)
	assertError(t, err, 409, "ALREADY_PRINTED")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterForceAllowsReprint(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(true))).
		WithArgs("B001").
		WillReturnRows(newQueueRows("sudah", "1990-01-01"))
	mock.ExpectExec(regexp.QuoteMeta(updateSQL)).
		WithArgs("B001").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := svc.Register(context.Background(), "B001", true)
	if err != nil {
		t.Fatalf("Register with force: %v", err)
	}
	if !got.Force || got.Printer != "medium" || got.RowsChanged != 1 {
		t.Fatalf("unexpected forced result: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterForceStillRejectsInvalidBirthDate(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(true))).
		WithArgs("B001").
		WillReturnRows(newQueueRows("sudah", "0000-00-00"))
	mock.ExpectRollback()

	_, err := svc.Register(context.Background(), "B001", true)
	assertError(t, err, 409, "NOT_ELIGIBLE")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterRejectsUnexpectedRowCount(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(true))).
		WithArgs("B001").
		WillReturnRows(newQueueRows("menunggu", "1990-01-01"))
	mock.ExpectExec(regexp.QuoteMeta(updateSQL)).
		WithArgs("B001").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err := svc.Register(context.Background(), "B001", false)
	assertError(t, err, 500, "DATABASE_ERROR")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateDryRunWithoutUpdate(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(false))).
		WithArgs("B001").
		WillReturnRows(newQueueRows("menunggu", "1990-01-01"))

	got, err := svc.Validate(context.Background(), "B001", false)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !got.DryRun || got.RowsChanged != 0 || got.Printer != "menunggu" {
		t.Fatalf("unexpected dry-run result: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStatus(t *testing.T) {
	svc, mock, database := newMockService(t)
	defer database.Close()

	mock.ExpectQuery(regexp.QuoteMeta(selectSQL(false))).
		WithArgs("B001").
		WillReturnRows(newQueueRows("medium", "1990-01-01"))

	got, err := svc.Status(context.Background(), "B001")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if got.Printer != "medium" || !got.BirthDateValid || got.Tanggal != "2026-11-27" {
		t.Fatalf("unexpected status result: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterRejectsEmptyCode(t *testing.T) {
	svc, _, database := newMockService(t)
	defer database.Close()

	_, err := svc.Register(context.Background(), "   ", false)
	assertError(t, err, 400, "INVALID_REQUEST")
}
