package handler

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/pobradovic08/route-beacon/internal/model"
	"github.com/pobradovic08/route-beacon/internal/store"
)

// HandleGetRouteHistory handles GET /api/v1/routers/{routerId}/routes/history.
func HandleGetRouteHistory(db *store.DB) http.HandlerFunc {
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

		events, err := db.GetRouteHistory(r.Context(), routerID, prefix, from, to, limit)
		if err != nil {
			model.WriteProblem(w, http.StatusInternalServerError, "Failed to query route history.")
			return
		}

		hasMore := len(events) > limit
		if hasMore {
			events = events[:limit]
		}

		resp := model.RouteHistoryResponse{
			RouterID: routerID,
			Prefix:   prefix,
			From:     model.FormatTime(from),
			To:       model.FormatTime(to),
			Events:   events,
			HasMore:  hasMore,
		}

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("route history: failed to encode response: %v", err)
		}
	}
}
