package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SonyPradana/simpus-antrian-terminal/internal/service"

	"github.com/DATA-DOG/go-sqlmock"
)

type fakePrinter struct {
	registerResult   *service.PrintResult
	registerErr      error
	validateResult   *service.PrintResult
	validateErr      error
	statusResult     *service.StatusResult
	statusErr        error
	lastRegisterKode string
	lastValidateKode string
	lastStatusKode   string
	lastForce        bool
}

func (f *fakePrinter) Register(_ context.Context, kode string, force bool) (*service.PrintResult, error) {
	f.lastRegisterKode = kode
	f.lastForce = force
	return f.registerResult, f.registerErr
}

func (f *fakePrinter) Validate(_ context.Context, kode string, force bool) (*service.PrintResult, error) {
	f.lastValidateKode = kode
	f.lastForce = force
	return f.validateResult, f.validateErr
}

func (f *fakePrinter) Status(_ context.Context, kode string) (*service.StatusResult, error) {
	f.lastStatusKode = kode
	return f.statusResult, f.statusErr
}

func newTestServer(t *testing.T, fake *fakePrinter, dryRun bool) (*Server, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	database, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	return New(fake, database, "test-token", dryRun), mock, database
}

func request(t *testing.T, srv *Server, method, target, body, apiKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if apiKey != "" {
		req.Header.Set(headerAPIKey, apiKey)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, status, rec.Body.String())
	}
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error.Code != code {
		t.Fatalf("code = %q, want %q", body.Error.Code, code)
	}
}

func TestPostPrintSuccess(t *testing.T) {
	fake := &fakePrinter{registerResult: &service.PrintResult{
		Kode: "B001", UnitID: "Pendaftaran", Tanggal: "2026-11-27", Printer: "medium", RowsChanged: 1,
	}}
	srv, _, database := newTestServer(t, fake, false)
	defer database.Close()

	rec := request(t, srv, http.MethodPost, "/api/print", `{"kode":"B001"}`, "test-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if fake.lastRegisterKode != "B001" {
		t.Fatalf("service received code %q", fake.lastRegisterKode)
	}
	var got service.PrintResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Printer != "medium" || got.RowsChanged != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestPostPrintWithoutAPIKey(t *testing.T) {
	srv, _, database := newTestServer(t, &fakePrinter{}, false)
	defer database.Close()

	rec := request(t, srv, http.MethodPost, "/api/print", `{"kode":"B001"}`, "")
	assertErrorResponse(t, rec, http.StatusUnauthorized, "UNAUTHORIZED")
}

func TestPostPrintWithWrongAPIKey(t *testing.T) {
	srv, _, database := newTestServer(t, &fakePrinter{}, false)
	defer database.Close()

	rec := request(t, srv, http.MethodPost, "/api/print", `{"kode":"B001"}`, "salah")
	assertErrorResponse(t, rec, http.StatusUnauthorized, "UNAUTHORIZED")
}

func TestPostPrintInvalidJSON(t *testing.T) {
	srv, _, database := newTestServer(t, &fakePrinter{}, false)
	defer database.Close()

	rec := request(t, srv, http.MethodPost, "/api/print", `{`, "test-token")
	assertErrorResponse(t, rec, http.StatusBadRequest, "INVALID_REQUEST")
}

func TestPostPrintRejectsUnknownField(t *testing.T) {
	srv, _, database := newTestServer(t, &fakePrinter{}, false)
	defer database.Close()

	rec := request(t, srv, http.MethodPost, "/api/print", `{"kode":"B001","unit_id":"Pendaftaran"}`, "test-token")
	assertErrorResponse(t, rec, http.StatusBadRequest, "INVALID_REQUEST")
}

func TestPostPrintForwardsServiceError(t *testing.T) {
	fake := &fakePrinter{registerErr: &service.Error{
		Status: http.StatusConflict, Code: "ALREADY_REQUESTED", Message: "already requested",
	}}
	srv, _, database := newTestServer(t, fake, false)
	defer database.Close()

	rec := request(t, srv, http.MethodPost, "/api/print", `{"kode":"B001"}`, "test-token")
	assertErrorResponse(t, rec, http.StatusConflict, "ALREADY_REQUESTED")
}

func TestPostPrintDryRunCallsValidate(t *testing.T) {
	fake := &fakePrinter{validateResult: &service.PrintResult{
		Kode: "B001", Printer: "menunggu", DryRun: true,
	}}
	srv, _, database := newTestServer(t, fake, true)
	defer database.Close()

	rec := request(t, srv, http.MethodPost, "/api/print", `{"kode":"B001"}`, "test-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if fake.lastRegisterKode != "" || fake.lastValidateKode != "B001" {
		t.Fatalf("dry-run called wrong method: register=%q validate=%q", fake.lastRegisterKode, fake.lastValidateKode)
	}
	var got service.PrintResult
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if !got.DryRun {
		t.Fatal("response did not set dry_run")
	}
}

func TestPostPrintAcceptsForce(t *testing.T) {
	fake := &fakePrinter{registerResult: &service.PrintResult{
		Kode: "B001", Printer: "medium", RowsChanged: 1, Force: true,
	}}
	srv, _, database := newTestServer(t, fake, false)
	defer database.Close()

	rec := request(t, srv, http.MethodPost, "/api/print", `{"kode":"B001","force":true}`, "test-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if !fake.lastForce {
		t.Fatal("force flag was not forwarded to the service")
	}
	var got service.PrintResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !got.Force {
		t.Fatal("response did not set force")
	}
}

func TestGetStatus(t *testing.T) {
	fake := &fakePrinter{statusResult: &service.StatusResult{
		Kode: "B001", UnitID: "Pendaftaran", Printer: "medium", BirthDateValid: true,
	}}
	srv, _, database := newTestServer(t, fake, false)
	defer database.Close()

	rec := request(t, srv, http.MethodGet, "/api/print/B001", "", "test-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if fake.lastStatusKode != "B001" {
		t.Fatalf("service received code %q", fake.lastStatusKode)
	}
}

func TestHealthOK(t *testing.T) {
	srv, mock, database := newTestServer(t, &fakePrinter{}, false)
	defer database.Close()
	mock.ExpectPing()

	rec := request(t, srv, http.MethodGet, "/healthz", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"db":"ok"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHealthDatabaseDown(t *testing.T) {
	srv, mock, database := newTestServer(t, &fakePrinter{}, false)
	defer database.Close()
	mock.ExpectPing().WillReturnError(errors.New("db down"))

	rec := request(t, srv, http.MethodGet, "/healthz", "", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"degraded"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
