package api

import (
	"net/http"

	"github.com/pobradovic08/route-beacon/internal/central/grpcserver"
	"github.com/pobradovic08/route-beacon/internal/central/registry"
	"github.com/pobradovic08/route-beacon/internal/central/routestore"
	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// APIHandler implements the ServerInterface from the generated OpenAPI code.
type APIHandler struct {
	Registry   *registry.Registry
	RouteStore *routestore.Store
	StartedAt  int64 // unix timestamp for uptime calculation
	Dispatcher *grpcserver.CommandDispatcher
}

// ListCollectors handles GET /api/v1/collectors
func (h *APIHandler) ListCollectors(w http.ResponseWriter, r *http.Request) {
	collectors := h.Registry.ListCollectors()

	apiCollectors := make([]Collector, 0, len(collectors))
	for _, c := range collectors {
		status := CollectorStatusOnline
		if c.GetStatus() == model.CollectorStatusDisconnected {
			status = CollectorStatusOffline
		}
		apiCollectors = append(apiCollectors, Collector{
			Id:          c.ID,
			Location:    c.Location,
			Status:      status,
			RouterCount: len(c.ListRouterSessions()),
		})
	}

	WriteJSON(w, http.StatusOK, struct {
		Data []Collector `json:"data"`
	}{Data: apiCollectors})
}
