package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookupRejectsMissingPrefix(t *testing.T) {
	handler := HandleLookupRoutes(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/routes/lookup",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestLookupRejectsInvalidMatchType(t *testing.T) {
	handler := HandleLookupRoutes(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/routes/lookup?prefix=10.0.0.0/24&match_type=invalid",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestLookupRejectsInvalidCIDR(t *testing.T) {
	handler := HandleLookupRoutes(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/routes/lookup?prefix=not-a-cidr/24&match_type=exact",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}

	var prob problemResponse
	if err := json.NewDecoder(w.Body).Decode(&prob); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(prob.InvalidParams) == 0 {
		t.Fatal("expected invalid_params in response")
	}
	if prob.InvalidParams[0].Name != "prefix" {
		t.Fatalf("expected param name 'prefix', got %q", prob.InvalidParams[0].Name)
	}
}

func TestLookupRejectsInvalidIPForLongest(t *testing.T) {
	handler := HandleLookupRoutes(nil)

	req := httptest.NewRequest("GET",
		"/api/v1/routers/r1/routes/lookup?prefix=not-an-ip&match_type=longest",
		nil)
	req.SetPathValue("routerId", "r1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

