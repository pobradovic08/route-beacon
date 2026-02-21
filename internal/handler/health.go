package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pobradovic08/route-beacon/internal/model"
	"github.com/pobradovic08/route-beacon/internal/store"
)

// HandleGetHealth returns the system health status.
func HandleGetHealth(db *store.DB, startTime time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary, err := db.GetHealthSummary(r.Context())
		if err != nil {
			model.WriteProblem(w, http.StatusServiceUnavailable, "Database unreachable.")
			return
		}

		status := "unhealthy"
		if summary.OnlineRouters > 0 {
			if summary.AllEOR != nil && *summary.AllEOR {
				status = "healthy"
			} else {
				status = "degraded"
			}
		}

		resp := model.HealthResponse{
			Status:        status,
			RouterCount:   summary.RouterCount,
			OnlineRouters: summary.OnlineRouters,
			TotalRoutes:   summary.TotalRoutes,
			UptimeSeconds: int64(time.Since(startTime).Seconds()),
		}

		if status == "unhealthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(resp)
	}
}
