package api

import (
	"net/http"

	"github.com/pobradovic08/route-beacon/internal/central/registry"
	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// ListTargets handles GET /api/v1/targets
func (h *APIHandler) ListTargets(w http.ResponseWriter, r *http.Request, params ListTargetsParams) {
	var sessions []*model.RouterSession
	if params.CollectorId != nil && *params.CollectorId != "" {
		sessions = h.Registry.ListTargetsByCollector(*params.CollectorId)
	} else {
		sessions = h.Registry.ListTargets()
	}

	targets := make([]LGTarget, 0, len(sessions))
	for _, sess := range sessions {
		targets = append(targets, sessionToLGTarget(sess, h.Registry))
	}

	WriteJSON(w, http.StatusOK, struct {
		Data []LGTarget `json:"data"`
	}{Data: targets})
}

// GetTarget handles GET /api/v1/targets/{targetId}
func (h *APIHandler) GetTarget(w http.ResponseWriter, r *http.Request, targetId string) {
	sess := h.Registry.GetTargetByString(targetId)
	if sess == nil {
		WriteProblem(w, http.StatusNotFound, "Not Found",
			"LG target '"+targetId+"' does not exist.")
		return
	}

	target := sessionToLGTarget(sess, h.Registry)
	WriteJSON(w, http.StatusOK, struct {
		Data LGTarget `json:"data"`
	}{Data: target})
}

func sessionToLGTarget(sess *model.RouterSession, reg *registry.Registry) LGTarget {
	collector := reg.GetCollector(sess.CollectorID)

	collectorSummary := CollectorSummary{
		Id:       sess.CollectorID,
		Location: "",
	}
	if collector != nil {
		collectorSummary.Location = collector.Location
	}

	status := lgTargetStatusFromModel(sess.GetStatus())

	return LGTarget{
		Id:          sess.LGTargetID().String(),
		Collector:   collectorSummary,
		DisplayName: sess.DisplayName,
		Asn:         int(sess.ASN),
		Status:      status,
		LastUpdate:  sess.GetLastUpdate(),
	}
}

func lgTargetStatusFromModel(status model.RouterSessionStatus) LGTargetStatus {
	switch status {
	case model.RouterSessionStatusActive, model.RouterSessionStatusEstablished:
		return Up
	case model.RouterSessionStatusDown:
		return Down
	default:
		return Unknown
	}
}
