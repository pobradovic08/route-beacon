package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
	"github.com/pobradovic08/route-beacon/internal/shared/validate"
)

// LookupRoutes handles GET /api/v1/targets/{targetId}/routes/lookup
func (h *APIHandler) LookupRoutes(w http.ResponseWriter, r *http.Request, targetId TargetId, params LookupRoutesParams) {
	// Find the target
	sess := h.Registry.GetTargetByString(targetId)
	if sess == nil {
		WriteProblem(w, http.StatusNotFound, "Not Found",
			fmt.Sprintf("LG target '%s' does not exist.", targetId))
		return
	}

	// Parse and validate the prefix
	prefix, isBareIP, err := validate.ParsePrefix(params.Prefix)
	if err != nil {
		WriteProblem(w, http.StatusBadRequest, "Bad Request",
			fmt.Sprintf("Invalid prefix: %s", err))
		return
	}

	// Determine match type
	matchType := "exact"
	if isBareIP {
		matchType = "longest"
	}
	if params.MatchType != nil {
		if *params.MatchType == LookupRoutesParamsMatchTypeExact {
			matchType = "exact"
		} else if *params.MatchType == LookupRoutesParamsMatchTypeLongest {
			matchType = "longest"
		}
	}

	// Perform lookup
	lgTargetID := sess.LGTargetID()
	var entry *model.RouteEntry

	if matchType == "exact" {
		entry = h.RouteStore.LookupExact(lgTargetID, prefix)
		// Fall back to LPM if no exact match and no explicit match_type
		if entry == nil && params.MatchType == nil {
			entry = h.RouteStore.LookupLPM(lgTargetID, prefix.Addr())
			if entry != nil {
				matchType = "longest"
			}
		}
	} else {
		entry = h.RouteStore.LookupLPM(lgTargetID, prefix.Addr())
	}

	if entry == nil {
		// Return empty result, not an error
		collector := h.Registry.GetCollector(sess.CollectorID)
		collectorStatus := RouteLookupMetaCollectorStatusOnline
		if collector != nil && collector.GetStatus() == model.CollectorStatusDisconnected {
			collectorStatus = RouteLookupMetaCollectorStatusOffline
		}

		mt := RouteLookupMetaMatchType(matchType)
		dataUpdated := sess.GetLastUpdate()
		stale := sess.GetStatus() == model.RouterSessionStatusDown

		WriteJSON(w, http.StatusOK, RouteLookupResponse{
			Prefix:    prefix.String(),
			Target:    TargetSummary{Id: targetId, DisplayName: sess.DisplayName, Asn: int(sess.ASN)},
			Paths:     []BGPPath{},
			PlainText: fmt.Sprintf("No routes found for %s on %s (AS%d)", prefix.String(), sess.DisplayName, sess.ASN),
			Meta:      RouteLookupMeta{MatchType: mt, DataUpdatedAt: dataUpdated, Stale: stale, CollectorStatus: collectorStatus},
		})
		return
	}

	// Convert paths to API format
	apiPaths := make([]BGPPath, 0, len(entry.Paths))
	for _, p := range entry.Paths {
		apiPaths = append(apiPaths, modelPathToAPI(p))
	}

	// Build plain text
	plainText := buildPlainText(entry, sess)

	// Build response
	collector := h.Registry.GetCollector(sess.CollectorID)
	collectorStatus := RouteLookupMetaCollectorStatusOnline
	if collector != nil && collector.GetStatus() == model.CollectorStatusDisconnected {
		collectorStatus = RouteLookupMetaCollectorStatusOffline
	}

	mt := RouteLookupMetaMatchType(matchType)
	dataUpdated := sess.GetLastUpdate()
	stale := sess.GetStatus() == model.RouterSessionStatusDown

	WriteJSON(w, http.StatusOK, RouteLookupResponse{
		Prefix:    entry.Prefix.String(),
		Target:    TargetSummary{Id: targetId, DisplayName: sess.DisplayName, Asn: int(sess.ASN)},
		Paths:     apiPaths,
		PlainText: plainText,
		Meta:      RouteLookupMeta{MatchType: mt, DataUpdatedAt: dataUpdated, Stale: stale, CollectorStatus: collectorStatus},
	})
}

func modelPathToAPI(p model.BGPPath) BGPPath {
	asPath := make([]int, 0)
	for _, seg := range p.ASPath {
		for _, asn := range seg.ASNs {
			asPath = append(asPath, int(asn))
		}
	}

	var med *int
	if p.MEDPresent {
		m := int(p.MED)
		med = &m
	}
	var localPref *int
	if p.LocalPrefPresent {
		lp := int(p.LocalPref)
		localPref = &lp
	}

	// Communities
	communities := make([]Community, 0)
	for _, c := range p.Communities {
		communities = append(communities, Community{Type: Standard, Value: c.String()})
	}
	extCommunities := make([]Community, 0)
	for _, ec := range p.ExtendedCommunities {
		extCommunities = append(extCommunities, Community{Type: Extended, Value: ec.Value})
	}
	largeCommunities := make([]Community, 0)
	for _, lc := range p.LargeCommunities {
		largeCommunities = append(largeCommunities, Community{Type: Large, Value: lc.String()})
	}

	origin := BGPPathOrigin(p.Origin)

	var agg *struct {
		Address *string `json:"address,omitempty"`
		Asn     *int    `json:"asn,omitempty"`
	}
	if p.Aggregator != nil {
		addr := p.Aggregator.Address.String()
		asn := int(p.Aggregator.ASN)
		agg = &struct {
			Address *string `json:"address,omitempty"`
			Asn     *int    `json:"asn,omitempty"`
		}{
			Address: &addr,
			Asn:     &asn,
		}
	}

	return BGPPath{
		Best:                p.IsBest,
		AsPath:              asPath,
		NextHop:             p.NextHop.String(),
		Origin:              origin,
		Med:                 med,
		LocalPref:           localPref,
		Communities:         communities,
		ExtendedCommunities: extCommunities,
		LargeCommunities:    largeCommunities,
		Aggregator:          agg,
		AtomicAggregate:     p.AtomicAggregate,
	}
}

func buildPlainText(entry *model.RouteEntry, sess *model.RouterSession) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "BGP routing table entry for %s\n", entry.Prefix)
	fmt.Fprintf(&sb, "Target: %s (AS%d)\n\n", sess.DisplayName, sess.ASN)

	for i, p := range entry.Paths {
		bestMarker := ""
		if p.IsBest {
			bestMarker = " (best)"
		}
		fmt.Fprintf(&sb, "Path #%d%s\n", i+1, bestMarker)

		// AS Path
		asns := make([]string, 0)
		for _, seg := range p.ASPath {
			for _, asn := range seg.ASNs {
				asns = append(asns, fmt.Sprintf("%d", asn))
			}
		}
		fmt.Fprintf(&sb, "  AS Path: %s\n", strings.Join(asns, " "))
		fmt.Fprintf(&sb, "  Next Hop: %s\n", p.NextHop)
		fmt.Fprintf(&sb, "  Origin: %s\n", strings.ToUpper(p.Origin))

		if p.MEDPresent {
			fmt.Fprintf(&sb, "  MED: %d\n", p.MED)
		}
		if p.LocalPrefPresent {
			fmt.Fprintf(&sb, "  Local Pref: %d\n", p.LocalPref)
		}
		if len(p.Communities) > 0 {
			comms := make([]string, 0, len(p.Communities))
			for _, c := range p.Communities {
				comms = append(comms, c.String())
			}
			fmt.Fprintf(&sb, "  Communities: %s\n", strings.Join(comms, " "))
		}
		if len(p.LargeCommunities) > 0 {
			comms := make([]string, 0, len(p.LargeCommunities))
			for _, lc := range p.LargeCommunities {
				comms = append(comms, lc.String())
			}
			fmt.Fprintf(&sb, "  Large Communities: %s\n", strings.Join(comms, " "))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
