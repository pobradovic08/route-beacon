package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pobradovic08/route-beacon/internal/model"
	"github.com/pobradovic08/route-beacon/internal/store"
)

// HandleListRouters returns all monitored routers.
func HandleListRouters(db *store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routers, err := db.ListRouters(r.Context())
		if err != nil {
			model.WriteProblem(w, http.StatusInternalServerError, "Failed to query routers.")
			return
		}
		json.NewEncoder(w).Encode(model.RouterListResponse{Data: routers})
	}
}

// HandleGetRouter returns a single router by ID with routing statistics.
func HandleGetRouter(db *store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routerID := r.PathValue("routerId")
		router, err := db.GetRouterDetail(r.Context(), routerID)
		if err != nil {
			model.WriteProblem(w, http.StatusInternalServerError, "Failed to query router.")
			return
		}
		if router == nil {
			model.WriteProblem(w, http.StatusNotFound, "Router '"+routerID+"' does not exist.")
			return
		}
		json.NewEncoder(w).Encode(struct {
			Data *model.RouterDetail `json:"data"`
		}{Data: router})
	}
}
