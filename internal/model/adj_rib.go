package model

import "encoding/json"

// AdjRibInRoute extends Route with the policy flag from Adj-RIB-In.
type AdjRibInRoute struct {
	Route
	IsPostPolicy bool `json:"is_post_policy"`
}

// AdjRibInRouteAttrs holds route attributes without prefix/path_id/is_post_policy.
type AdjRibInRouteAttrs struct {
	NextHop             *string         `json:"next_hop"`
	ASPath              []any           `json:"as_path"`
	Origin              *string         `json:"origin"`
	LocalPref           *int            `json:"local_pref"`
	MED                 *int            `json:"med"`
	OriginASN           *int            `json:"origin_asn"`
	Communities         []Community     `json:"communities"`
	ExtendedCommunities []Community     `json:"extended_communities"`
	LargeCommunities    []Community     `json:"large_communities"`
	Attrs               json.RawMessage `json:"attrs"`
	FirstSeen           string          `json:"first_seen"`
	UpdatedAt           string          `json:"updated_at"`
}

// AdjRibInPath groups pre- and post-policy attributes for a single path_id.
type AdjRibInPath struct {
	PathID     int64               `json:"path_id"`
	PrePolicy  *AdjRibInRouteAttrs `json:"pre_policy"`
	PostPolicy *AdjRibInRouteAttrs `json:"post_policy"`
}

// AdjRibInLookupMeta extends RouteLookupMeta with the policy filter.
type AdjRibInLookupMeta struct {
	MatchType    string `json:"match_type"`
	RouterStatus string `json:"router_status"`
	Policy       string `json:"policy"`
}

// AdjRibInLookupResponse is the response for an Adj-RIB-In route lookup.
type AdjRibInLookupResponse struct {
	Prefix    string             `json:"prefix"`
	Router    RouterSummary      `json:"router"`
	Paths     []AdjRibInPath     `json:"paths"`
	PlainText string             `json:"plain_text"`
	Meta      AdjRibInLookupMeta `json:"meta"`
}

// AdjRibInRouteEvent extends RouteEvent with the policy flag.
type AdjRibInRouteEvent struct {
	RouteEvent
	IsPostPolicy bool `json:"is_post_policy"`
}

// AdjRibInRouteHistoryResponse is the response for Adj-RIB-In route history.
type AdjRibInRouteHistoryResponse struct {
	RouterID string               `json:"router_id"`
	Prefix   string               `json:"prefix"`
	From     string               `json:"from"`
	To       string               `json:"to"`
	Events   []AdjRibInRouteEvent `json:"events"`
	HasMore  bool                 `json:"has_more"`
}
