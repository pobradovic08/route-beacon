package store

import (
	"testing"

	"github.com/pobradovic08/route-beacon/internal/model"
)

func TestParseASPath_Empty(t *testing.T) {
	result := parseASPath(nil)
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %v", result)
	}

	empty := ""
	result = parseASPath(&empty)
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %v", result)
	}
}

func TestParseASPath_Simple(t *testing.T) {
	s := "64500 64501 65000"
	result := parseASPath(&s)
	if len(result) != 3 {
		t.Fatalf("expected 3 elements, got %d: %v", len(result), result)
	}
	for i, want := range []int{64500, 64501, 65000} {
		got, ok := result[i].(int)
		if !ok {
			t.Fatalf("element %d: expected int, got %T", i, result[i])
		}
		if got != want {
			t.Fatalf("element %d: expected %d, got %d", i, want, got)
		}
	}
}

func TestParseASPath_WithASSet(t *testing.T) {
	s := "64500 {64501,64502} 65000"
	result := parseASPath(&s)
	if len(result) != 3 {
		t.Fatalf("expected 3 elements, got %d: %v", len(result), result)
	}

	// First element: plain ASN
	if v, ok := result[0].(int); !ok || v != 64500 {
		t.Fatalf("element 0: expected 64500, got %v", result[0])
	}

	// Second element: AS_SET
	set, ok := result[1].([]any)
	if !ok {
		t.Fatalf("element 1: expected []any (AS_SET), got %T", result[1])
	}
	if len(set) != 2 {
		t.Fatalf("AS_SET: expected 2 elements, got %d", len(set))
	}
	if v, ok := set[0].(int); !ok || v != 64501 {
		t.Fatalf("AS_SET[0]: expected 64501, got %v", set[0])
	}
	if v, ok := set[1].(int); !ok || v != 64502 {
		t.Fatalf("AS_SET[1]: expected 64502, got %v", set[1])
	}

	// Third element: plain ASN
	if v, ok := result[2].(int); !ok || v != 65000 {
		t.Fatalf("element 2: expected 65000, got %v", result[2])
	}
}

func TestParseASPath_SingleASN(t *testing.T) {
	s := "64500"
	result := parseASPath(&s)
	if len(result) != 1 {
		t.Fatalf("expected 1 element, got %d: %v", len(result), result)
	}
	if v, ok := result[0].(int); !ok || v != 64500 {
		t.Fatalf("expected 64500, got %v", result[0])
	}
}

func TestParseASPath_SetOnly(t *testing.T) {
	s := "{64501,64502}"
	result := parseASPath(&s)
	if len(result) != 1 {
		t.Fatalf("expected 1 element, got %d: %v", len(result), result)
	}
	set, ok := result[0].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result[0])
	}
	if len(set) != 2 {
		t.Fatalf("expected 2 elements in set, got %d", len(set))
	}
}

func TestParseCommunities_Empty(t *testing.T) {
	result := parseCommunities(nil, "standard")
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %v", result)
	}
}

func TestParseCommunities_Values(t *testing.T) {
	result := parseCommunities([]string{"65000:100", "65000:200"}, "standard")
	if len(result) != 2 {
		t.Fatalf("expected 2 communities, got %d", len(result))
	}
	if result[0].Type != "standard" || result[0].Value != "65000:100" {
		t.Fatalf("unexpected community 0: %+v", result[0])
	}
	if result[1].Type != "standard" || result[1].Value != "65000:200" {
		t.Fatalf("unexpected community 1: %+v", result[1])
	}
}

func TestGeneratePlainText_NoRoutes(t *testing.T) {
	result := GeneratePlainText("10.0.0.0/24", "router1", nil)
	expected := "No routes found for 10.0.0.0/24 on router1\n"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestGeneratePlainText_MultiplePaths(t *testing.T) {
	nh := "192.0.2.1"
	origin := "igp"
	routes := []model.Route{
		{
			Prefix:  "10.0.0.0/24",
			PathID:  0,
			NextHop: &nh,
			ASPath:  []any{64500, 65000},
			Origin:  &origin,
		},
		{
			Prefix:  "10.0.0.0/24",
			PathID:  1,
			NextHop: &nh,
			ASPath:  []any{64501, 65000},
			Origin:  &origin,
		},
	}
	result := GeneratePlainText("10.0.0.0/24", "router1", routes)

	// Path #1 should NOT have a "(best)" label
	if contains(result, "(best)") {
		t.Fatalf("expected no (best) label, got:\n%s", result)
	}
	if !contains(result, "Path #1\n") {
		t.Fatalf("expected 'Path #1\\n', got:\n%s", result)
	}
	if !contains(result, "Path #2\n") {
		t.Fatalf("expected 'Path #2\\n', got:\n%s", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
