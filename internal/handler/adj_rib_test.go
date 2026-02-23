package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdjRibInLookupRejectsMissingPrefix(t *testing.T) {
	handler := HandleAdjRibInLookup(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/lookup",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestAdjRibInLookupRejectsInvalidPolicy(t *testing.T) {
	handler := HandleAdjRibInLookup(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/lookup?prefix=10.0.0.0/24&policy=invalid",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestAdjRibInLookupRejectsInvalidMatchType(t *testing.T) {
	handler := HandleAdjRibInLookup(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/lookup?prefix=10.0.0.0/24&match_type=invalid",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestAdjRibInLookupRejectsInvalidCIDR(t *testing.T) {
	handler := HandleAdjRibInLookup(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/lookup?prefix=not-a-cidr/24&match_type=exact",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestAdjRibInLookupRejectsInvalidIP(t *testing.T) {
	handler := HandleAdjRibInLookup(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/lookup?prefix=not-an-ip&match_type=longest",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestAdjRibInHistoryRejectsMissingPrefix(t *testing.T) {
	handler := HandleAdjRibInHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/history",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestAdjRibInHistoryRejectsInvalidPrefix(t *testing.T) {
	handler := HandleAdjRibInHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/history?prefix=notaprefix",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestAdjRibInHistoryRejectsInvalidPolicy(t *testing.T) {
	handler := HandleAdjRibInHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/history?prefix=10.0.0.0/24&policy=invalid",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestAdjRibInHistoryRejectsReversedTimeRange(t *testing.T) {
	handler := HandleAdjRibInHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/history?prefix=10.0.0.0/24&from=2025-01-02T00:00:00Z&to=2025-01-01T00:00:00Z",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAdjRibInHistoryRejectsExceeded7DayRange(t *testing.T) {
	handler := HandleAdjRibInHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/history?prefix=10.0.0.0/24&from=2025-01-01T00:00:00Z&to=2025-01-10T00:00:00Z",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAdjRibInHistoryRejectsInvalidLimit(t *testing.T) {
	handler := HandleAdjRibInHistory(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/adj-rib-in/history?prefix=10.0.0.0/24&limit=0",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
