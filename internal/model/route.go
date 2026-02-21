package model

import "encoding/json"

// Community represents a BGP community.
type Community struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Route represents a BGP route entry from the Loc-RIB.
type Route struct {
	Prefix              string            `json:"prefix"`
	PathID              int64             `json:"path_id"`
	NextHop             *string           `json:"next_hop"`
	ASPath              []any             `json:"as_path"`
	Origin              *string           `json:"origin"`
	LocalPref           *int              `json:"local_pref"`
	MED                 *int              `json:"med"`
	OriginASN           *int              `json:"origin_asn"`
	Communities         []Community       `json:"communities"`
	ExtendedCommunities []Community       `json:"extended_communities"`
	LargeCommunities    []Community       `json:"large_communities"`
	Attrs               json.RawMessage   `json:"attrs"`
	FirstSeen           string            `json:"first_seen"`
	UpdatedAt           string            `json:"updated_at"`
}

// RouterSummary is the router info embedded in a route lookup response.
type RouterSummary struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	ASNumber    *int64 `json:"as_number"`
}

// RouteLookupMeta contains metadata about the lookup.
type RouteLookupMeta struct {
	MatchType    string `json:"match_type"`
	RouterStatus string `json:"router_status"`
}

// RouteLookupResponse is the response for a route lookup.
type RouteLookupResponse struct {
	Prefix    string          `json:"prefix"`
	Router    RouterSummary   `json:"router"`
	Routes    []Route         `json:"routes"`
	PlainText string          `json:"plain_text"`
	Meta      RouteLookupMeta `json:"meta"`
}

// RouteEvent represents a historical route change.
type RouteEvent struct {
	Timestamp           string      `json:"timestamp"`
	Action              string      `json:"action"`
	Prefix              string      `json:"prefix"`
	PathID              *int64      `json:"path_id"`
	NextHop             *string     `json:"next_hop"`
	ASPath              []any       `json:"as_path"`
	Origin              *string     `json:"origin"`
	LocalPref           *int        `json:"local_pref"`
	MED                 *int        `json:"med"`
	OriginASN           *int        `json:"origin_asn"`
	Communities         []Community `json:"communities"`
	ExtendedCommunities []Community `json:"extended_communities"`
	LargeCommunities    []Community `json:"large_communities"`
}

// RouteHistoryResponse is the response for route history queries.
type RouteHistoryResponse struct {
	RouterID string       `json:"router_id"`
	Prefix   string       `json:"prefix"`
	From     string       `json:"from"`
	To       string       `json:"to"`
	Events   []RouteEvent `json:"events"`
	HasMore  bool         `json:"has_more"`
}
