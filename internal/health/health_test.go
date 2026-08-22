package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockHandler(t *testing.T) (*Handler, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectPing()
	database, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm with sqlmock: %v", err)
	}

	return NewHandler(database), mock
}

func serve(handler *Handler, method, path string) *httptest.ResponseRecorder {
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) response {
	t.Helper()
	var body response
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return body
}

func TestLivenessDoesNotCheckPostgres(t *testing.T) {
	handler, mock := newMockHandler(t)

	for _, path := range []string{"/livez", "/health/live"} {
		rr := serve(handler, http.MethodGet, path)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, rr.Code)
		}
		body := decodeBody(t, rr)
		if body.Status != "up" {
			t.Errorf("%s: expected status up, got %s", path, body.Status)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("liveness should not ping postgres: %v", err)
	}
}

func TestDependencyProbesSucceedWhenPostgresIsHealthy(t *testing.T) {
	handler, mock := newMockHandler(t)

	paths := []string{"/health", "/healthz", "/readyz", "/startupz", "/health/ready", "/health/startup"}
	for range paths {
		mock.ExpectPing()
	}

	for _, path := range paths {
		rr := serve(handler, http.MethodGet, path)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d body=%s", path, rr.Code, rr.Body.String())
		}
		body := decodeBody(t, rr)
		if body.Status != "up" {
			t.Errorf("%s: expected status up, got %s", path, body.Status)
		}
		if body.Checks["postgres"] != "up" {
			t.Errorf("%s: expected postgres up, got %s", path, body.Checks["postgres"])
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expected postgres pings: %v", err)
	}
}

func TestDependencyProbesFailWhenPostgresIsDown(t *testing.T) {
	handler, mock := newMockHandler(t)

	paths := []string{"/health", "/healthz", "/readyz", "/startupz", "/health/ready", "/health/startup"}
	for range paths {
		mock.ExpectPing().WillReturnError(gorm.ErrInvalidDB)
	}

	for _, path := range paths {
		rr := serve(handler, http.MethodGet, path)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: expected 503, got %d body=%s", path, rr.Code, rr.Body.String())
		}
		body := decodeBody(t, rr)
		if body.Status != "down" {
			t.Errorf("%s: expected status down, got %s", path, body.Status)
		}
		if body.Checks["postgres"] != "down" {
			t.Errorf("%s: expected postgres down, got %s", path, body.Checks["postgres"])
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expected failed postgres pings: %v", err)
	}
}

func TestHealthFailsWhenDatabaseIsMissing(t *testing.T) {
	rr := serve(NewHandler(nil), http.MethodGet, "/health")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	body := decodeBody(t, rr)
	if body.Status != "down" || body.Checks["postgres"] != "down" {
		t.Errorf("expected down postgres check, got %+v", body)
	}
}
