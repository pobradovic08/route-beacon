package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pobradovic08/route-beacon/internal/model"
	"github.com/pobradovic08/route-beacon/internal/store"
)

// HandleAdjRibInLookup handles GET /api/v1/routers/{routerId}/adj-rib-in/lookup.
func HandleAdjRibInLookup(db *store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routerID := r.PathValue("routerId")

		prefix := r.URL.Query().Get("prefix")
		matchType := r.URL.Query().Get("match_type")
		policy := r.URL.Query().Get("policy")

		if prefix == "" {
			model.WriteProblemWithParams(w, http.StatusUnprocessableEntity,
				"Request validation failed.",
				[]model.InvalidParam{{Name: "prefix", Reason: "prefix query parameter is required."}})
			return
		}

		// Default policy
		if policy == "" {
			policy = "both"
		}
		if policy != "pre" && policy != "post" && policy != "both" {
			model.WriteProblemWithParams(w, http.StatusUnprocessableEntity,
				"Request validation failed.",
				[]model.InvalidParam{{Name: "policy", Reason: "Must be 'pre', 'post', or 'both'."}})
			return
		}

		// Auto-detect match type
		if matchType == "" {
			if strings.Contains(prefix, "/") {
				matchType = "exact"
			} else {
				matchType = "longest"
			}
		}

		if matchType != "exact" && matchType != "longest" {
			model.WriteProblemWithParams(w, http.StatusUnprocessableEntity,
				"Request validation failed.",
				[]model.InvalidParam{{Name: "match_type", Reason: "Must be 'exact' or 'longest'."}})
			return
		}

		// Validate prefix format
		if matchType == "exact" {
			_, _, err := net.ParseCIDR(prefix)
			if err != nil {
				model.WriteProblemWithParams(w, http.StatusUnprocessableEntity,
					"Request validation failed.",
					[]model.InvalidParam{{Name: "prefix", Reason: "Not a valid IPv4 or IPv6 prefix."}})
				return
			}
		} else {
			ip := net.ParseIP(prefix)
			if ip == nil {
				model.WriteProblemWithParams(w, http.StatusUnprocessableEntity,
					"Request validation failed.",
					[]model.InvalidParam{{Name: "prefix", Reason: "Not a valid IPv4 or IPv6 address."}})
				return
			}
		}

		// Check router exists
		routerSummary, routerStatus, err := db.GetRouterSummary(r.Context(), routerID)
		if err != nil {
			model.WriteProblem(w, http.StatusInternalServerError, "Failed to query router.")
			return
		}
		if routerSummary == nil {
			model.WriteProblem(w, http.StatusNotFound, "Router '"+routerID+"' does not exist.")
			return
		}

		// Execute lookup
		var routes []model.AdjRibInRoute
		if matchType == "exact" {
			routes, err = db.AdjRibInExactLookup(r.Context(), routerID, prefix, policy)
		} else {
			routes, err = db.AdjRibInLPMLookup(r.Context(), routerID, prefix, policy)
		}
		if err != nil {
			model.WriteProblem(w, http.StatusInternalServerError, "Route lookup failed.")
			return
		}

		// Determine matched prefix
		responsePrefix := prefix
		if matchType == "longest" && len(routes) > 0 {
			responsePrefix = routes[0].Prefix
		}

		paths := store.GroupAdjRibInPaths(routes)

		resp := model.AdjRibInLookupResponse{
			Prefix:    responsePrefix,
			Router:    *routerSummary,
			Paths:     paths,
			PlainText: store.GenerateAdjRibInPlainText(responsePrefix, routerID, paths),
			Meta: model.AdjRibInLookupMeta{
				MatchType:    matchType,
				RouterStatus: routerStatus,
				Policy:       policy,
			},
		}

		json.NewEncoder(w).Encode(resp)
	}
}

// HandleAdjRibInHistory handles GET /api/v1/routers/{routerId}/adj-rib-in/history.
func HandleAdjRibInHistory(db *store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routerID := r.PathValue("routerId")

		prefix := r.URL.Query().Get("prefix")
		if prefix == "" {
			model.WriteProblemWithParams(w, http.StatusUnprocessableEntity,
				"Request validation failed.",
				[]model.InvalidParam{{Name: "prefix", Reason: "prefix query parameter is required."}})
			return
		}

		// Validate prefix is CIDR
		_, _, err := net.ParseCIDR(prefix)
		if err != nil {
			model.WriteProblemWithParams(w, http.StatusUnprocessableEntity,
				"Request validation failed.",
				[]model.InvalidParam{{Name: "prefix", Reason: "Not a valid IPv4 or IPv6 prefix."}})
			return
		}

		// Parse time range
		now := time.Now().UTC()
		from := now.Add(-24 * time.Hour)
		to := now

		if v := r.URL.Query().Get("from"); v != "" {
			parsed, err := time.Parse(time.RFC3339, v)
			if err != nil {
				model.WriteProblemWithParams(w, http.StatusBadRequest,
					"Request validation failed.",
					[]model.InvalidParam{{Name: "from", Reason: "Must be a valid ISO 8601 timestamp."}})
				return
			}
			from = parsed
		}

		if v := r.URL.Query().Get("to"); v != "" {
			parsed, err := time.Parse(time.RFC3339, v)
			if err != nil {
				model.WriteProblemWithParams(w, http.StatusBadRequest,
					"Request validation failed.",
					[]model.InvalidParam{{Name: "to", Reason: "Must be a valid ISO 8601 timestamp."}})
				return
			}
			to = parsed
		}

		// Validate time range
		if from.After(to) {
			model.WriteProblemWithParams(w, http.StatusBadRequest,
				"Request validation failed.",
				[]model.InvalidParam{{Name: "from", Reason: "'from' must not be after 'to'."}})
			return
		}
		if to.Sub(from) > 7*24*time.Hour {
			model.WriteProblemWithParams(w, http.StatusBadRequest,
				"Request validation failed.",
				[]model.InvalidParam{{Name: "from", Reason: "Time range must not exceed 7 days."}})
			return
		}

		// Parse limit
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			parsed, err := strconv.Atoi(v)
			if err != nil || parsed < 1 || parsed > 1000 {
				model.WriteProblemWithParams(w, http.StatusBadRequest,
					"Request validation failed.",
					[]model.InvalidParam{{Name: "limit", Reason: "Must be between 1 and 1000."}})
				return
			}
			limit = parsed
		}

		// Check router exists
		routerSummary, _, err := db.GetRouterSummary(r.Context(), routerID)
		if err != nil {
			model.WriteProblem(w, http.StatusInternalServerError, "Failed to query router.")
			return
		}
		if routerSummary == nil {
			model.WriteProblem(w, http.StatusNotFound, "Router '"+routerID+"' does not exist.")
			return
		}

		events, err := db.GetAdjRibInHistory(r.Context(), routerID, prefix, from, to, limit)
		if err != nil {
			model.WriteProblem(w, http.StatusInternalServerError, "Failed to query route history.")
			return
		}

		hasMore := len(events) > limit
		if hasMore {
			events = events[:limit]
		}

		resp := model.AdjRibInRouteHistoryResponse{
			RouterID: routerID,
			Prefix:   prefix,
			From:     model.FormatTime(from),
			To:       model.FormatTime(to),
			Events:   events,
			HasMore:  hasMore,
		}

		json.NewEncoder(w).Encode(resp)
	}
}
