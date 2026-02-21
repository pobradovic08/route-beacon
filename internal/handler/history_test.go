package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// problemResponse is a minimal structure for RFC 7807 error responses.
type problemResponse struct {
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Detail        string `json:"detail"`
	InvalidParams []struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	} `json:"invalid_params"`
}

func TestHistoryRejectsReversedTimeRange(t *testing.T) {
	// Use a nil DB since validation should reject before any DB call.
	handler := HandleGetRouteHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/routes/history?prefix=10.0.0.0/24&from=2025-01-02T00:00:00Z&to=2025-01-01T00:00:00Z",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var prob problemResponse
	if err := json.NewDecoder(w.Body).Decode(&prob); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(prob.InvalidParams) == 0 {
		t.Fatal("expected invalid_params in response")
	}
	if prob.InvalidParams[0].Name != "from" {
		t.Fatalf("expected param name 'from', got %q", prob.InvalidParams[0].Name)
	}
}

func TestHistoryRejectsMissingPrefix(t *testing.T) {
	handler := HandleGetRouteHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/routes/history",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestHistoryRejectsInvalidPrefix(t *testing.T) {
	handler := HandleGetRouteHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/routes/history?prefix=notaprefix",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestHistoryRejectsExceeded7DayRange(t *testing.T) {
	handler := HandleGetRouteHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/routes/history?prefix=10.0.0.0/24&from=2025-01-01T00:00:00Z&to=2025-01-10T00:00:00Z",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHistoryRejectsInvalidLimit(t *testing.T) {
	handler := HandleGetRouteHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/routes/history?prefix=10.0.0.0/24&limit=0",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
