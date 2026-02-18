package api

import (
	"net/http"
	"time"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// GetHealth handles GET /api/v1/health
func (h *APIHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	totalCollectors := h.Registry.TotalCollectorCount()
	connectedCollectors := h.Registry.ConnectedCollectorCount()
	totalRoutes := h.RouteStore.TotalRoutes()
	uptimeSeconds := int(time.Now().Unix() - h.StartedAt)

	// Status is "degraded" if any registered collector is disconnected
	status := Healthy
	if totalCollectors > 0 {
		for _, c := range h.Registry.ListCollectors() {
			if c.GetStatus() == model.CollectorStatusDisconnected {
				status = Degraded
				break
			}
		}
	}

	WriteJSON(w, http.StatusOK, HealthResponse{
		Status:              HealthResponseStatus(status),
		CollectorCount:      totalCollectors,
		ConnectedCollectors: connectedCollectors,
		TotalRoutes:         int(totalRoutes),
		UptimeSeconds:       uptimeSeconds,
	})
}
