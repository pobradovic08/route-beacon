package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/pobradovic08/route-beacon/internal/central/registry"
	"github.com/pobradovic08/route-beacon/internal/central/routestore"
	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// setupTestHandler creates an APIHandler with a real registry and route store,
// registers a collector with a single router session, and returns the handler
// along with the LG target ID string used to address the session.
func setupTestHandler(t *testing.T) (*APIHandler, model.LGTargetID, string) {
	t.Helper()

	reg := registry.New()
	store := routestore.New()

	sess := &model.RouterSession{
		ID:          "sess-1",
		DisplayName: "Test Router",
		ASN:         65000,
	}
	err := reg.RegisterCollector("col-1", "Test Location", []*model.RouterSession{sess})
	if err != nil {
		t.Fatalf("RegisterCollector: %v", err)
	}

	lgTargetID := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
	store.CreateTrie(lgTargetID)

	targetIDStr := lgTargetID.String() // "col-1:sess-1"

	h := &APIHandler{
		Registry:   reg,
		RouteStore: store,
	}
	return h, lgTargetID, targetIDStr
}

func TestLookupRoutes_ExactMatch(t *testing.T) {
	h, lgTargetID, targetIDStr := setupTestHandler(t)

	// Insert a route into the store.
	prefix := netip.MustParsePrefix("8.8.8.0/24")
	bgpPath := model.BGPPath{
		NextHop: netip.MustParseAddr("10.0.0.1"),
		Origin:  "igp",
		IsBest:  true,
		PathID:  1,
		ASPath: []model.ASPathSegment{
			{Type: "AS_SEQUENCE", ASNs: []uint32{65000, 15169}},
		},
		LocalPref:        100,
		LocalPrefPresent: true,
	}
	h.RouteStore.UpsertRoute(lgTargetID, prefix, bgpPath)

	// Build request with exact prefix including prefix length.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/targets/"+targetIDStr+"/routes/lookup?prefix=8.8.8.0/24", nil)

	params := LookupRoutesParams{
		Prefix: "8.8.8.0/24",
	}
	h.LookupRoutes(rec, req, targetIDStr, params)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp RouteLookupResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify prefix.
	if resp.Prefix != "8.8.8.0/24" {
		t.Errorf("resp.Prefix = %q, want %q", resp.Prefix, "8.8.8.0/24")
	}

	// Verify target summary.
	if resp.Target.Id != targetIDStr {
		t.Errorf("resp.Target.Id = %q, want %q", resp.Target.Id, targetIDStr)
	}
	if resp.Target.DisplayName != "Test Router" {
		t.Errorf("resp.Target.DisplayName = %q, want %q", resp.Target.DisplayName, "Test Router")
	}
	if resp.Target.Asn != 65000 {
		t.Errorf("resp.Target.Asn = %d, want %d", resp.Target.Asn, 65000)
	}

	// Verify paths.
	if len(resp.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(resp.Paths))
	}
	p := resp.Paths[0]
	if !p.Best {
		t.Error("expected path.Best = true")
	}
	if p.NextHop != "10.0.0.1" {
		t.Errorf("path.NextHop = %q, want %q", p.NextHop, "10.0.0.1")
	}
	if p.Origin != Igp {
		t.Errorf("path.Origin = %q, want %q", p.Origin, Igp)
	}
	if len(p.AsPath) != 2 || p.AsPath[0] != 65000 || p.AsPath[1] != 15169 {
		t.Errorf("path.AsPath = %v, want [65000 15169]", p.AsPath)
	}
	if p.LocalPref == nil || *p.LocalPref != 100 {
		t.Errorf("path.LocalPref = %v, want 100", p.LocalPref)
	}

	// Verify meta.
	if resp.Meta.MatchType != RouteLookupMetaMatchTypeExact {
		t.Errorf("meta.MatchType = %q, want %q", resp.Meta.MatchType, RouteLookupMetaMatchTypeExact)
	}
	if resp.Meta.CollectorStatus != RouteLookupMetaCollectorStatusOnline {
		t.Errorf("meta.CollectorStatus = %q, want %q", resp.Meta.CollectorStatus, RouteLookupMetaCollectorStatusOnline)
	}
}

func TestLookupRoutes_LPMFallback(t *testing.T) {
	h, lgTargetID, targetIDStr := setupTestHandler(t)

	// Insert a /24 route.
	prefix := netip.MustParsePrefix("8.8.8.0/24")
	bgpPath := model.BGPPath{
		NextHop: netip.MustParseAddr("10.0.0.1"),
		Origin:  "igp",
		IsBest:  true,
		PathID:  1,
		ASPath: []model.ASPathSegment{
			{Type: "AS_SEQUENCE", ASNs: []uint32{65000, 15169}},
		},
	}
	h.RouteStore.UpsertRoute(lgTargetID, prefix, bgpPath)

	// Send a bare IP (no prefix length) with no explicit match_type.
	// The handler should detect a bare IP, default to LPM, and find the /24.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/targets/"+targetIDStr+"/routes/lookup?prefix=8.8.8.8", nil)

	params := LookupRoutesParams{
		Prefix: "8.8.8.8",
	}
	h.LookupRoutes(rec, req, targetIDStr, params)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp RouteLookupResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Bare IP triggers LPM; the matched prefix should be 8.8.8.0/24.
	if resp.Prefix != "8.8.8.0/24" {
		t.Errorf("resp.Prefix = %q, want %q", resp.Prefix, "8.8.8.0/24")
	}

	if len(resp.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(resp.Paths))
	}

	// Meta should say "longest" since the bare IP forced LPM mode.
	if resp.Meta.MatchType != RouteLookupMetaMatchTypeLongest {
		t.Errorf("meta.MatchType = %q, want %q", resp.Meta.MatchType, RouteLookupMetaMatchTypeLongest)
	}
}

func TestLookupRoutes_EmptyResult(t *testing.T) {
	h, _, targetIDStr := setupTestHandler(t)

	// Look up a prefix with no routes inserted.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/targets/"+targetIDStr+"/routes/lookup?prefix=192.0.2.0/24", nil)

	params := LookupRoutesParams{
		Prefix: "192.0.2.0/24",
	}
	h.LookupRoutes(rec, req, targetIDStr, params)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp RouteLookupResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Paths should be an empty array.
	if resp.Paths == nil {
		t.Fatal("expected non-nil (empty) paths array")
	}
	if len(resp.Paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(resp.Paths))
	}

	// Prefix in the response should still be the requested one.
	if resp.Prefix != "192.0.2.0/24" {
		t.Errorf("resp.Prefix = %q, want %q", resp.Prefix, "192.0.2.0/24")
	}

	// Target should be populated.
	if resp.Target.Id != targetIDStr {
		t.Errorf("resp.Target.Id = %q, want %q", resp.Target.Id, targetIDStr)
	}

	// Meta should still be present and reflect collector status.
	if resp.Meta.CollectorStatus != RouteLookupMetaCollectorStatusOnline {
		t.Errorf("meta.CollectorStatus = %q, want %q", resp.Meta.CollectorStatus, RouteLookupMetaCollectorStatusOnline)
	}
}

func TestLookupRoutes_InvalidPrefix(t *testing.T) {
	h, _, targetIDStr := setupTestHandler(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/targets/"+targetIDStr+"/routes/lookup?prefix=not-a-prefix", nil)

	params := LookupRoutesParams{
		Prefix: "not-a-prefix",
	}
	h.LookupRoutes(rec, req, targetIDStr, params)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it returns a ProblemDetail response.
	var problem ProblemDetail
	if err := json.NewDecoder(rec.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem response: %v", err)
	}
	if problem.Status != http.StatusBadRequest {
		t.Errorf("problem.Status = %d, want %d", problem.Status, http.StatusBadRequest)
	}
	if problem.Title != "Bad Request" {
		t.Errorf("problem.Title = %q, want %q", problem.Title, "Bad Request")
	}
}

func TestLookupRoutes_UnknownTarget(t *testing.T) {
	h, _, _ := setupTestHandler(t)

	unknownTargetID := "nonexistent-collector:nonexistent-session"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/targets/"+unknownTargetID+"/routes/lookup?prefix=8.8.8.0/24", nil)

	params := LookupRoutesParams{
		Prefix: "8.8.8.0/24",
	}
	h.LookupRoutes(rec, req, unknownTargetID, params)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it returns a ProblemDetail response.
	var problem ProblemDetail
	if err := json.NewDecoder(rec.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem response: %v", err)
	}
	if problem.Status != http.StatusNotFound {
		t.Errorf("problem.Status = %d, want %d", problem.Status, http.StatusNotFound)
	}
	if problem.Title != "Not Found" {
		t.Errorf("problem.Title = %q, want %q", problem.Title, "Not Found")
	}
}
