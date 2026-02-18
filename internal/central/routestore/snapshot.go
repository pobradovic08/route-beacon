package routestore

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// snapshotVersion is bumped when the serialisation format changes.
// Readers reject versions they do not understand.
const snapshotVersion uint32 = 1

// snapshotHeader is written first and allows forward-compatible detection of
// incompatible snapshot files.
type snapshotHeader struct {
	Version   uint32
	Timestamp time.Time
}

// snapshotData is the top-level envelope serialised to disk.
type snapshotData struct {
	Header  snapshotHeader
	Targets []snapshotTarget
}

// snapshotTarget holds all routes for one LG target (collector + session pair).
type snapshotTarget struct {
	CollectorID string
	SessionID   string
	Entries     []snapshotEntry
}

// snapshotEntry is a gob-friendly representation of a single prefix and its
// BGP paths. The stdlib net/netip types do not implement encoding/gob natively,
// so we convert to/from raw byte representations.
type snapshotEntry struct {
	// PrefixBytes is the marshalled form of netip.Prefix.
	PrefixBytes []byte
	Paths       []snapshotPath
}

// snapshotPath mirrors model.BGPPath with gob-friendly field types.
type snapshotPath struct {
	ASPath              []snapshotASPathSegment
	NextHopBytes        []byte // marshalled netip.Addr
	Origin              string
	MED                 uint32
	MEDPresent          bool
	LocalPref           uint32
	LocalPrefPresent    bool
	Communities         []uint32
	ExtendedCommunities []snapshotExtCommunity
	LargeCommunities    []snapshotLargeCommunity
	Aggregator          *snapshotAggregator
	AtomicAggregate     bool
	IsBest              bool
	Age                 time.Duration
	ReceivedAt          time.Time
	PathID              uint32
}

type snapshotASPathSegment struct {
	Type string
	ASNs []uint32
}

type snapshotExtCommunity struct {
	Raw   [8]byte
	Type  string
	Value string
}

type snapshotLargeCommunity struct {
	GlobalAdmin uint32
	LocalData1  uint32
	LocalData2  uint32
}

type snapshotAggregator struct {
	ASN          uint32
	AddressBytes []byte // marshalled netip.Addr
}

// SaveSnapshot serialises all route data from the store's tries into a binary
// file at the given path. It uses atomic writes: a temporary file is written
// first, then renamed to the final path so that a crash mid-write never
// corrupts the snapshot.
func (s *Store) SaveSnapshot(path string) error {
	data, err := s.buildSnapshotData()
	if err != nil {
		return fmt.Errorf("build snapshot data: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up the temp file on any error path.
	success := false
	defer func() {
		if !success {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	enc := gob.NewEncoder(tmp)
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Atomic rename.
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename snapshot file: %w", err)
	}

	success = true
	return nil
}

// LoadSnapshot deserialises a binary snapshot file and loads all routes back
// into the store's tries. If the file does not exist the call returns nil
// (not an error) so that first-time startups work without special handling.
func (s *Store) LoadSnapshot(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open snapshot file: %w", err)
	}
	defer f.Close()

	var data snapshotData
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&data); err != nil {
		return fmt.Errorf("decode snapshot: %w", err)
	}

	if data.Header.Version != snapshotVersion {
		return fmt.Errorf("unsupported snapshot version %d (expected %d)", data.Header.Version, snapshotVersion)
	}

	for _, target := range data.Targets {
		id := model.LGTargetID{
			CollectorID: target.CollectorID,
			SessionID:   target.SessionID,
		}

		entries := make(map[netip.Prefix]*model.RouteEntry, len(target.Entries))
		for _, se := range target.Entries {
			prefix, err := unmarshalPrefix(se.PrefixBytes)
			if err != nil {
				return fmt.Errorf("unmarshal prefix for target %s: %w", id.String(), err)
			}

			paths := make([]model.BGPPath, 0, len(se.Paths))
			for _, sp := range se.Paths {
				p, err := snapshotPathToModel(sp)
				if err != nil {
					return fmt.Errorf("convert path for prefix %s on target %s: %w", prefix.String(), id.String(), err)
				}
				paths = append(paths, p)
			}

			entries[prefix] = &model.RouteEntry{
				Prefix: prefix,
				Paths:  paths,
			}
		}

		s.ReplaceAll(id, entries)
	}

	return nil
}

// StartPeriodicSnapshot launches a goroutine that saves a snapshot at the
// given interval. The goroutine exits when the context is cancelled.
func (s *Store) StartPeriodicSnapshot(ctx context.Context, path string, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Perform one final snapshot before exiting.
				if err := s.SaveSnapshot(path); err != nil {
					slog.Error("final snapshot save failed", "error", err)
				} else {
					slog.Info("final snapshot saved", "path", path)
				}
				return
			case <-ticker.C:
				start := time.Now()
				if err := s.SaveSnapshot(path); err != nil {
					slog.Error("periodic snapshot save failed", "error", err, "path", path)
				} else {
					slog.Info("periodic snapshot saved",
						"path", path,
						"duration", time.Since(start),
						"total_routes", s.TotalRoutes(),
					)
				}
			}
		}
	}()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildSnapshotData creates a snapshot of all data in the store while
// holding read locks. Each trie is iterated under its own lock to avoid
// holding the top-level lock for the entire duration.
func (s *Store) buildSnapshotData() (*snapshotData, error) {
	s.mu.RLock()
	// Capture the set of target IDs and trie pointers under the top-level lock.
	type idTrie struct {
		id   model.LGTargetID
		trie *lgTargetTrie
	}
	items := make([]idTrie, 0, len(s.tries))
	for id, t := range s.tries {
		items = append(items, idTrie{id: id, trie: t})
	}
	s.mu.RUnlock()

	targets := make([]snapshotTarget, 0, len(items))
	for _, it := range items {
		st, err := buildTargetSnapshot(it.id, it.trie)
		if err != nil {
			return nil, err
		}
		targets = append(targets, st)
	}

	return &snapshotData{
		Header: snapshotHeader{
			Version:   snapshotVersion,
			Timestamp: time.Now(),
		},
		Targets: targets,
	}, nil
}

func buildTargetSnapshot(id model.LGTargetID, t *lgTargetTrie) (snapshotTarget, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	st := snapshotTarget{
		CollectorID: id.CollectorID,
		SessionID:   id.SessionID,
		Entries:     make([]snapshotEntry, 0, int(t.count)),
	}

	for prefix, entry := range t.trie.All() {
		se, err := modelEntryToSnapshot(prefix, entry)
		if err != nil {
			return st, fmt.Errorf("convert entry for prefix %s: %w", prefix.String(), err)
		}
		st.Entries = append(st.Entries, se)
	}

	return st, nil
}

func modelEntryToSnapshot(prefix netip.Prefix, entry *model.RouteEntry) (snapshotEntry, error) {
	prefixBytes, err := marshalPrefix(prefix)
	if err != nil {
		return snapshotEntry{}, err
	}

	paths := make([]snapshotPath, 0, len(entry.Paths))
	for _, p := range entry.Paths {
		sp, err := modelPathToSnapshot(p)
		if err != nil {
			return snapshotEntry{}, err
		}
		paths = append(paths, sp)
	}

	return snapshotEntry{
		PrefixBytes: prefixBytes,
		Paths:       paths,
	}, nil
}

func modelPathToSnapshot(p model.BGPPath) (snapshotPath, error) {
	nhBytes, err := p.NextHop.MarshalBinary()
	if err != nil {
		return snapshotPath{}, fmt.Errorf("marshal next_hop: %w", err)
	}

	segments := make([]snapshotASPathSegment, len(p.ASPath))
	for i, seg := range p.ASPath {
		segments[i] = snapshotASPathSegment{
			Type: seg.Type,
			ASNs: seg.ASNs,
		}
	}

	communities := make([]uint32, len(p.Communities))
	for i, c := range p.Communities {
		communities[i] = c.Value
	}

	extComms := make([]snapshotExtCommunity, len(p.ExtendedCommunities))
	for i, ec := range p.ExtendedCommunities {
		extComms[i] = snapshotExtCommunity{
			Raw:   ec.Raw,
			Type:  ec.Type,
			Value: ec.Value,
		}
	}

	largeComms := make([]snapshotLargeCommunity, len(p.LargeCommunities))
	for i, lc := range p.LargeCommunities {
		largeComms[i] = snapshotLargeCommunity{
			GlobalAdmin: lc.GlobalAdmin,
			LocalData1:  lc.LocalData1,
			LocalData2:  lc.LocalData2,
		}
	}

	sp := snapshotPath{
		ASPath:              segments,
		NextHopBytes:        nhBytes,
		Origin:              p.Origin,
		MED:                 p.MED,
		MEDPresent:          p.MEDPresent,
		LocalPref:           p.LocalPref,
		LocalPrefPresent:    p.LocalPrefPresent,
		Communities:         communities,
		ExtendedCommunities: extComms,
		LargeCommunities:    largeComms,
		AtomicAggregate:     p.AtomicAggregate,
		IsBest:              p.IsBest,
		Age:                 p.Age,
		ReceivedAt:          p.ReceivedAt,
		PathID:              p.PathID,
	}

	if p.Aggregator != nil {
		addrBytes, err := p.Aggregator.Address.MarshalBinary()
		if err != nil {
			return snapshotPath{}, fmt.Errorf("marshal aggregator address: %w", err)
		}
		sp.Aggregator = &snapshotAggregator{
			ASN:          p.Aggregator.ASN,
			AddressBytes: addrBytes,
		}
	}

	return sp, nil
}

func snapshotPathToModel(sp snapshotPath) (model.BGPPath, error) {
	var nextHop netip.Addr
	if err := nextHop.UnmarshalBinary(sp.NextHopBytes); err != nil {
		return model.BGPPath{}, fmt.Errorf("unmarshal next_hop: %w", err)
	}

	segments := make([]model.ASPathSegment, len(sp.ASPath))
	for i, seg := range sp.ASPath {
		segments[i] = model.ASPathSegment{
			Type: seg.Type,
			ASNs: seg.ASNs,
		}
	}

	communities := make([]model.Community, len(sp.Communities))
	for i, v := range sp.Communities {
		communities[i] = model.NewCommunity(v)
	}

	extComms := make([]model.ExtendedCommunity, len(sp.ExtendedCommunities))
	for i, ec := range sp.ExtendedCommunities {
		extComms[i] = model.ExtendedCommunity{
			Raw:   ec.Raw,
			Type:  ec.Type,
			Value: ec.Value,
		}
	}

	largeComms := make([]model.LargeCommunity, len(sp.LargeCommunities))
	for i, lc := range sp.LargeCommunities {
		largeComms[i] = model.LargeCommunity{
			GlobalAdmin: lc.GlobalAdmin,
			LocalData1:  lc.LocalData1,
			LocalData2:  lc.LocalData2,
		}
	}

	p := model.BGPPath{
		ASPath:              segments,
		NextHop:             nextHop,
		Origin:              sp.Origin,
		MED:                 sp.MED,
		MEDPresent:          sp.MEDPresent,
		LocalPref:           sp.LocalPref,
		LocalPrefPresent:    sp.LocalPrefPresent,
		Communities:         communities,
		ExtendedCommunities: extComms,
		LargeCommunities:    largeComms,
		AtomicAggregate:     sp.AtomicAggregate,
		IsBest:              sp.IsBest,
		Age:                 sp.Age,
		ReceivedAt:          sp.ReceivedAt,
		PathID:              sp.PathID,
	}

	if sp.Aggregator != nil {
		var addr netip.Addr
		if err := addr.UnmarshalBinary(sp.Aggregator.AddressBytes); err != nil {
			return model.BGPPath{}, fmt.Errorf("unmarshal aggregator address: %w", err)
		}
		p.Aggregator = &model.Aggregator{
			ASN:     sp.Aggregator.ASN,
			Address: addr,
		}
	}

	return p, nil
}

// marshalPrefix encodes a netip.Prefix to bytes via its MarshalBinary method.
func marshalPrefix(p netip.Prefix) ([]byte, error) {
	return p.MarshalBinary()
}

// unmarshalPrefix decodes a netip.Prefix from bytes.
func unmarshalPrefix(b []byte) (netip.Prefix, error) {
	var p netip.Prefix
	if err := p.UnmarshalBinary(b); err != nil {
		return p, err
	}
	return p, nil
}
