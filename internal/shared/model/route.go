package model

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"time"
)

type RouteEntry struct {
	Prefix netip.Prefix `json:"prefix"`
	Paths  []BGPPath    `json:"paths"`
}

type BGPPath struct {
	ASPath              []ASPathSegment     `json:"as_path"`
	NextHop             netip.Addr          `json:"next_hop"`
	Origin              string              `json:"origin"`
	MED                 uint32              `json:"med"`
	MEDPresent          bool                `json:"med_present"`
	LocalPref           uint32              `json:"local_pref"`
	LocalPrefPresent    bool                `json:"local_pref_present"`
	Communities         []Community         `json:"communities,omitempty"`
	ExtendedCommunities []ExtendedCommunity `json:"extended_communities,omitempty"`
	LargeCommunities    []LargeCommunity    `json:"large_communities,omitempty"`
	Aggregator          *Aggregator         `json:"aggregator,omitempty"`
	AtomicAggregate     bool                `json:"atomic_aggregate,omitempty"`
	IsBest              bool                `json:"is_best"`
	IsFiltered          bool                `json:"is_filtered"`
	IsStale             bool                `json:"is_stale"`
	Age                 time.Duration       `json:"age"`
	ReceivedAt          time.Time           `json:"received_at"`
	PathID              uint32              `json:"path_id,omitempty"`
}

type ASPathSegment struct {
	Type string   `json:"type"`
	ASNs []uint32 `json:"asns"`
}

type Community struct {
	Value uint32 `json:"-"`
	High  uint16 `json:"high"`
	Low   uint16 `json:"low"`
}

func NewCommunity(value uint32) Community {
	return Community{
		Value: value,
		High:  uint16(value >> 16),
		Low:   uint16(value & 0xFFFF),
	}
}

func (c Community) String() string {
	return fmt.Sprintf("%d:%d", c.High, c.Low)
}

func (c Community) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

func (c *Community) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	var high, low uint16
	if _, err := fmt.Sscanf(s, "%d:%d", &high, &low); err != nil {
		return fmt.Errorf("invalid community format %q: %w", s, err)
	}
	c.High = high
	c.Low = low
	c.Value = uint32(high)<<16 | uint32(low)
	return nil
}

type ExtendedCommunity struct {
	Raw   [8]byte `json:"-"`
	Type  string  `json:"type"`
	Value string  `json:"value"`
}

type LargeCommunity struct {
	GlobalAdmin uint32 `json:"global_admin"`
	LocalData1  uint32 `json:"local_data1"`
	LocalData2  uint32 `json:"local_data2"`
}

func (lc LargeCommunity) String() string {
	return fmt.Sprintf("%d:%d:%d", lc.GlobalAdmin, lc.LocalData1, lc.LocalData2)
}

type Aggregator struct {
	ASN     uint32     `json:"asn"`
	Address netip.Addr `json:"address"`
}
