package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/pobradovic08/route-beacon/internal/model"
	"github.com/pobradovic08/route-beacon/internal/store"
)

// HandleLookupRoutes handles GET /api/v1/routers/{routerId}/routes/lookup.
func HandleLookupRoutes(db *store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routerID := r.PathValue("routerId")
		prefix := r.URL.Query().Get("prefix")
		matchType := r.URL.Query().Get("match_type")

		if prefix == "" {
			model.WriteProblemWithParams(w, http.StatusUnprocessableEntity,
				"Request validation failed.",
				[]model.InvalidParam{{Name: "prefix", Reason: "prefix query parameter is required."}})
			return
		}

		// Auto-detect match type if not specified
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
		var routes []model.Route
		if matchType == "exact" {
			routes, err = db.ExactLookup(r.Context(), routerID, prefix)
		} else {
			routes, err = db.LPMLookup(r.Context(), routerID, prefix)
		}
		if err != nil {
			model.WriteProblem(w, http.StatusInternalServerError, "Route lookup failed.")
			return
		}

		// Determine the matched prefix for the response
		responsePrefix := prefix
		if matchType == "longest" && len(routes) > 0 {
			responsePrefix = routes[0].Prefix
		}

		resp := model.RouteLookupResponse{
			Prefix:    responsePrefix,
			Router:    *routerSummary,
			Routes:    routes,
			PlainText: store.GeneratePlainText(responsePrefix, routerID, routes),
			Meta: model.RouteLookupMeta{
				MatchType:    matchType,
				RouterStatus: routerStatus,
			},
		}

		json.NewEncoder(w).Encode(resp)
	}
}
