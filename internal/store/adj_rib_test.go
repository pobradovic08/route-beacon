package store

import (
	"testing"

	"github.com/pobradovic08/route-beacon/internal/model"
)

func TestGroupAdjRibInPaths_Empty(t *testing.T) {
	paths := GroupAdjRibInPaths(nil)
	if len(paths) != 0 {
		t.Fatalf("expected empty slice, got %d paths", len(paths))
	}
}

func TestGroupAdjRibInPaths_PreOnly(t *testing.T) {
	nh := "192.0.2.1"
	routes := []model.AdjRibInRoute{
		{
			Route:        model.Route{Prefix: "10.0.0.0/24", PathID: 0, NextHop: &nh, ASPath: []any{64500}},
			IsPostPolicy: false,
		},
	}
	paths := GroupAdjRibInPaths(routes)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0].PrePolicy == nil {
		t.Fatal("expected PrePolicy to be set")
	}
	if paths[0].PostPolicy != nil {
		t.Fatal("expected PostPolicy to be nil")
	}
}

func TestGroupAdjRibInPaths_PostOnly(t *testing.T) {
	nh := "192.0.2.1"
	routes := []model.AdjRibInRoute{
		{
			Route:        model.Route{Prefix: "10.0.0.0/24", PathID: 0, NextHop: &nh, ASPath: []any{64500}},
			IsPostPolicy: true,
		},
	}
	paths := GroupAdjRibInPaths(routes)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0].PrePolicy != nil {
		t.Fatal("expected PrePolicy to be nil")
	}
	if paths[0].PostPolicy == nil {
		t.Fatal("expected PostPolicy to be set")
	}
}

func TestGroupAdjRibInPaths_Mixed(t *testing.T) {
	nh := "192.0.2.1"
	routes := []model.AdjRibInRoute{
		{
			Route:        model.Route{Prefix: "10.0.0.0/24", PathID: 0, NextHop: &nh, ASPath: []any{64500}},
			IsPostPolicy: false,
		},
		{
			Route:        model.Route{Prefix: "10.0.0.0/24", PathID: 0, NextHop: &nh, ASPath: []any{64500}},
			IsPostPolicy: true,
		},
	}
	paths := GroupAdjRibInPaths(routes)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0].PrePolicy == nil {
		t.Fatal("expected PrePolicy to be set")
	}
	if paths[0].PostPolicy == nil {
		t.Fatal("expected PostPolicy to be set")
	}
}

func TestGroupAdjRibInPaths_MultiplePathIDs(t *testing.T) {
	nh := "192.0.2.1"
	routes := []model.AdjRibInRoute{
		{
			Route:        model.Route{Prefix: "10.0.0.0/24", PathID: 0, NextHop: &nh, ASPath: []any{64500}},
			IsPostPolicy: false,
		},
		{
			Route:        model.Route{Prefix: "10.0.0.0/24", PathID: 0, NextHop: &nh, ASPath: []any{64500}},
			IsPostPolicy: true,
		},
		{
			Route:        model.Route{Prefix: "10.0.0.0/24", PathID: 1, NextHop: &nh, ASPath: []any{64501}},
			IsPostPolicy: false,
		},
	}
	paths := GroupAdjRibInPaths(routes)
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0].PathID != 0 {
		t.Fatalf("expected path_id 0, got %d", paths[0].PathID)
	}
	if paths[1].PathID != 1 {
		t.Fatalf("expected path_id 1, got %d", paths[1].PathID)
	}
	if paths[0].PrePolicy == nil || paths[0].PostPolicy == nil {
		t.Fatal("path 0 should have both pre and post policy")
	}
	if paths[1].PrePolicy == nil {
		t.Fatal("path 1 should have pre policy")
	}
	if paths[1].PostPolicy != nil {
		t.Fatal("path 1 should not have post policy")
	}
}

func TestGenerateAdjRibInPlainText_Empty(t *testing.T) {
	result := GenerateAdjRibInPlainText("10.0.0.0/24", "router1", nil)
	expected := "No Adj-RIB-In routes found for 10.0.0.0/24 on router1\n"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestGenerateAdjRibInPlainText_SinglePath(t *testing.T) {
	nh := "192.0.2.1"
	origin := "igp"
	lp := 100
	paths := []model.AdjRibInPath{
		{
			PathID: 0,
			PrePolicy: &model.AdjRibInRouteAttrs{
				NextHop:   &nh,
				ASPath:    []any{64500, 65000},
				Origin:    &origin,
				LocalPref: &lp,
			},
		},
	}
	result := GenerateAdjRibInPlainText("10.0.0.0/24", "router1", paths)

	if !searchString(result, "Path #1 (path_id 0)\n") {
		t.Fatalf("expected path header without (best), got:\n%s", result)
	}
	if searchString(result, "(best)") {
		t.Fatalf("should not contain (best), got:\n%s", result)
	}
	if !searchString(result, "[pre-policy]") {
		t.Fatalf("expected [pre-policy] section, got:\n%s", result)
	}
	if !searchString(result, "Next Hop: 192.0.2.1") {
		t.Fatalf("expected next hop, got:\n%s", result)
	}
	if !searchString(result, "AS Path: 64500 65000") {
		t.Fatalf("expected AS path, got:\n%s", result)
	}
}

func TestFormatASPath_Simple(t *testing.T) {
	result := formatASPath([]any{64500, 64501, 65000})
	expected := "64500 64501 65000"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatASPath_WithASSet(t *testing.T) {
	result := formatASPath([]any{64500, []any{64501, 64502}, 65000})
	expected := "64500 {64501,64502} 65000"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatASPath_Empty(t *testing.T) {
	result := formatASPath([]any{})
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
	result = formatASPath(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil, got %q", result)
	}
}
