package bgp

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	apipb "github.com/osrg/gobgp/v3/api"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// RouteEvent represents a route change from a BGP peer.
type RouteEvent struct {
	SessionID string
	IsAdd     bool
	Prefix    netip.Prefix
	Path      model.BGPPath
	PathID    uint32
}

// ConvertPath converts a GoBGP Path to the internal BGPPath model.
func ConvertPath(p *apipb.Path) (model.BGPPath, error) {
	bgpPath := model.BGPPath{
		IsBest:     p.Best,
		IsFiltered: p.Filtered,
		IsStale:    p.Stale,
		PathID:     p.Identifier,
		ReceivedAt: time.Now(),
	}

	for _, a := range p.Pattrs {
		msg, err := anypb.UnmarshalNew(a, proto.UnmarshalOptions{})
		if err != nil {
			slog.Warn("failed to unmarshal path attribute", "error", err)
			continue
		}

		switch attr := msg.(type) {
		case *apipb.AsPathAttribute:
			bgpPath.ASPath = convertASPath(attr)
		case *apipb.NextHopAttribute:
			bgpPath.NextHop, _ = netip.ParseAddr(attr.NextHop)
		case *apipb.OriginAttribute:
			bgpPath.Origin = originToString(attr.Origin)
		case *apipb.MultiExitDiscAttribute:
			bgpPath.MED = attr.Med
			bgpPath.MEDPresent = true
		case *apipb.LocalPrefAttribute:
			bgpPath.LocalPref = attr.LocalPref
			bgpPath.LocalPrefPresent = true
		case *apipb.CommunitiesAttribute:
			for _, c := range attr.Communities {
				bgpPath.Communities = append(bgpPath.Communities, model.NewCommunity(c))
			}
		case *apipb.ExtendedCommunitiesAttribute:
			for _, ec := range attr.Communities {
				bgpPath.ExtendedCommunities = append(bgpPath.ExtendedCommunities, convertExtCommunity(ec))
			}
		case *apipb.LargeCommunitiesAttribute:
			for _, lc := range attr.Communities {
				bgpPath.LargeCommunities = append(bgpPath.LargeCommunities, model.LargeCommunity{
					GlobalAdmin: lc.GlobalAdmin,
					LocalData1:  lc.LocalData1,
					LocalData2:  lc.LocalData2,
				})
			}
		case *apipb.AggregatorAttribute:
			bgpPath.Aggregator = &model.Aggregator{
				ASN: attr.Asn,
			}
			if addr, err := netip.ParseAddr(attr.Address); err == nil {
				bgpPath.Aggregator.Address = addr
			}
		case *apipb.AtomicAggregateAttribute:
			bgpPath.AtomicAggregate = true
		case *apipb.MpReachNLRIAttribute:
			// Extract next hop from MP_REACH for IPv6
			if len(attr.NextHops) > 0 {
				bgpPath.NextHop, _ = netip.ParseAddr(attr.NextHops[0])
			}
		}
	}

	if p.Age != nil {
		bgpPath.Age = time.Since(p.Age.AsTime())
	}

	return bgpPath, nil
}

// ParsePrefixFromPath extracts the prefix from a GoBGP Path's NLRI.
func ParsePrefixFromPath(p *apipb.Path) (netip.Prefix, error) {
	if p.Nlri == nil {
		return netip.Prefix{}, fmt.Errorf("path has no NLRI")
	}

	msg, err := anypb.UnmarshalNew(p.Nlri, proto.UnmarshalOptions{})
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("unmarshal NLRI: %w", err)
	}

	switch nlri := msg.(type) {
	case *apipb.IPAddressPrefix:
		addr, err := netip.ParseAddr(nlri.Prefix)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("parse prefix addr: %w", err)
		}
		return netip.PrefixFrom(addr, int(nlri.PrefixLen)), nil
	default:
		return netip.Prefix{}, fmt.Errorf("unsupported NLRI type: %T", nlri)
	}
}

func convertASPath(attr *apipb.AsPathAttribute) []model.ASPathSegment {
	segments := make([]model.ASPathSegment, 0, len(attr.Segments))
	for _, seg := range attr.Segments {
		segType := "sequence"
		if seg.Type == apipb.AsSegment_AS_SET {
			segType = "set"
		}
		segments = append(segments, model.ASPathSegment{
			Type: segType,
			ASNs: seg.Numbers,
		})
	}
	return segments
}

func originToString(origin uint32) string {
	switch origin {
	case 0:
		return "igp"
	case 1:
		return "egp"
	default:
		return "incomplete"
	}
}

func convertExtCommunity(a *anypb.Any) model.ExtendedCommunity {
	ec := model.ExtendedCommunity{
		Type:  "unknown",
		Value: fmt.Sprintf("%x", a.Value),
	}

	msg, err := anypb.UnmarshalNew(a, proto.UnmarshalOptions{})
	if err != nil {
		return ec
	}

	switch v := msg.(type) {
	case *apipb.TwoOctetAsSpecificExtended:
		if v.IsTransitive {
			ec.Type = "route-target"
		} else {
			ec.Type = "route-origin"
		}
		ec.Value = fmt.Sprintf("%d:%d", v.Asn, v.LocalAdmin)
		// Wire format: [Type][SubType][2-byte ASN][4-byte LocalAdmin]
		if v.IsTransitive {
			ec.Raw[0] = 0x00
		} else {
			ec.Raw[0] = 0x40
		}
		ec.Raw[1] = byte(v.SubType)
		binary.BigEndian.PutUint16(ec.Raw[2:4], uint16(v.Asn))
		binary.BigEndian.PutUint32(ec.Raw[4:8], v.LocalAdmin)
	case *apipb.IPv4AddressSpecificExtended:
		ec.Type = "ipv4-specific"
		ec.Value = fmt.Sprintf("%s:%d", v.Address, v.LocalAdmin)
		// Wire format: [Type][SubType][4-byte IPv4][2-byte LocalAdmin]
		if v.IsTransitive {
			ec.Raw[0] = 0x01
		} else {
			ec.Raw[0] = 0x41
		}
		ec.Raw[1] = byte(v.SubType)
		addr, err := netip.ParseAddr(v.Address)
		if err == nil {
			ip4 := addr.As4()
			copy(ec.Raw[2:6], ip4[:])
		}
		binary.BigEndian.PutUint16(ec.Raw[6:8], uint16(v.LocalAdmin))
	case *apipb.FourOctetAsSpecificExtended:
		ec.Type = "four-octet-as"
		ec.Value = fmt.Sprintf("%d:%d", v.Asn, v.LocalAdmin)
		// Wire format: [Type][SubType][4-byte ASN][2-byte LocalAdmin]
		if v.IsTransitive {
			ec.Raw[0] = 0x02
		} else {
			ec.Raw[0] = 0x42
		}
		ec.Raw[1] = byte(v.SubType)
		binary.BigEndian.PutUint32(ec.Raw[2:6], v.Asn)
		binary.BigEndian.PutUint16(ec.Raw[6:8], uint16(v.LocalAdmin))
	}

	return ec
}
