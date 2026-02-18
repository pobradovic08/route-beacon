package routestore

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// testRouteEntry builds a RouteEntry with representative BGP attributes for
// snapshot round-trip testing.
func testRouteEntry(prefix netip.Prefix) *model.RouteEntry {
	return &model.RouteEntry{
		Prefix: prefix,
		Paths: []model.BGPPath{
			{
				ASPath: []model.ASPathSegment{
					{Type: "AS_SEQUENCE", ASNs: []uint32{65000, 65001, 65002}},
				},
				NextHop:          netip.MustParseAddr("10.0.0.1"),
				Origin:           "igp",
				MED:              100,
				MEDPresent:       true,
				LocalPref:        200,
				LocalPrefPresent: true,
				Communities: []model.Community{
					model.NewCommunity(65000<<16 | 100),
					model.NewCommunity(65001<<16 | 200),
				},
				ExtendedCommunities: []model.ExtendedCommunity{
					{Raw: [8]byte{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x64}, Type: "route-target", Value: "65000:100"},
				},
				LargeCommunities: []model.LargeCommunity{
					{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2},
				},
				Aggregator: &model.Aggregator{
					ASN:     65000,
					Address: netip.MustParseAddr("192.168.1.1"),
				},
				AtomicAggregate: true,
				IsBest:          true,
				Age:             5 * time.Minute,
				ReceivedAt:      time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
				PathID:          1,
			},
			{
				ASPath: []model.ASPathSegment{
					{Type: "AS_SEQUENCE", ASNs: []uint32{65003, 65004}},
					{Type: "AS_SET", ASNs: []uint32{65005, 65006}},
				},
				NextHop:          netip.MustParseAddr("10.0.0.2"),
				Origin:           "incomplete",
				MED:              0,
				MEDPresent:       false,
				LocalPref:        100,
				LocalPrefPresent: true,
				IsBest:           false,
				Age:              10 * time.Minute,
				ReceivedAt:       time.Date(2026, 1, 15, 11, 55, 0, 0, time.UTC),
				PathID:           2,
			},
		},
	}
}

func TestSaveAndLoadSnapshot(t *testing.T) {
	// Populate a store with routes across multiple LG targets.
	store := New()

	target1 := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
	target2 := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-2"}
	target3 := model.LGTargetID{CollectorID: "col-2", SessionID: "sess-1"}

	store.CreateTrie(target1)
	store.CreateTrie(target2)
	store.CreateTrie(target3)

	// IPv4 routes.
	pfx1 := netip.MustParsePrefix("10.0.0.0/24")
	pfx2 := netip.MustParsePrefix("192.168.0.0/16")

	// IPv6 route.
	pfx3 := netip.MustParsePrefix("2001:db8::/32")

	e1 := testRouteEntry(pfx1)
	for _, p := range e1.Paths {
		store.UpsertRoute(target1, pfx1, p)
	}

	e2 := testRouteEntry(pfx2)
	for _, p := range e2.Paths {
		store.UpsertRoute(target2, pfx2, p)
	}

	e3 := testRouteEntry(pfx3)
	// Adjust the next-hop to IPv6 for the third entry.
	e3.Paths[0].NextHop = netip.MustParseAddr("2001:db8::1")
	e3.Paths[1].NextHop = netip.MustParseAddr("2001:db8::2")
	for _, p := range e3.Paths {
		store.UpsertRoute(target3, pfx3, p)
	}

	// Save snapshot.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.snapshot")

	if err := store.SaveSnapshot(path); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	// Verify the file exists.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("snapshot file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("snapshot file is empty")
	}

	// Load into a fresh store.
	store2 := New()
	if err := store2.LoadSnapshot(path); err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}

	// Verify target1 routes.
	got := store2.LookupExact(target1, pfx1)
	if got == nil {
		t.Fatal("target1: expected route entry for", pfx1)
	}
	if len(got.Paths) != 2 {
		t.Fatalf("target1: expected 2 paths, got %d", len(got.Paths))
	}
	assertPathEqual(t, "target1/path0", e1.Paths[0], got.Paths[0])
	assertPathEqual(t, "target1/path1", e1.Paths[1], got.Paths[1])

	// Verify target2 routes.
	got2 := store2.LookupExact(target2, pfx2)
	if got2 == nil {
		t.Fatal("target2: expected route entry for", pfx2)
	}
	if len(got2.Paths) != 2 {
		t.Fatalf("target2: expected 2 paths, got %d", len(got2.Paths))
	}

	// Verify target3 routes (IPv6).
	got3 := store2.LookupExact(target3, pfx3)
	if got3 == nil {
		t.Fatal("target3: expected route entry for", pfx3)
	}
	if len(got3.Paths) != 2 {
		t.Fatalf("target3: expected 2 paths, got %d", len(got3.Paths))
	}
	if got3.Paths[0].NextHop != netip.MustParseAddr("2001:db8::1") {
		t.Errorf("target3: expected IPv6 next_hop 2001:db8::1, got %s", got3.Paths[0].NextHop)
	}

	// Verify route counts.
	if c := store2.Count(target1); c != 1 {
		t.Errorf("target1 count: expected 1, got %d", c)
	}
	if c := store2.Count(target2); c != 1 {
		t.Errorf("target2 count: expected 1, got %d", c)
	}
	if c := store2.Count(target3); c != 1 {
		t.Errorf("target3 count: expected 1, got %d", c)
	}
	if total := store2.TotalRoutes(); total != 3 {
		t.Errorf("total routes: expected 3, got %d", total)
	}
}

func TestLoadSnapshot_FileNotExists(t *testing.T) {
	store := New()
	err := store.LoadSnapshot("/nonexistent/path/snapshot.bin")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
}

func TestLoadSnapshot_EmptyStore(t *testing.T) {
	// Save an empty store's snapshot and reload it.
	store := New()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.snapshot")

	if err := store.SaveSnapshot(path); err != nil {
		t.Fatalf("SaveSnapshot (empty): %v", err)
	}

	store2 := New()
	if err := store2.LoadSnapshot(path); err != nil {
		t.Fatalf("LoadSnapshot (empty): %v", err)
	}

	if total := store2.TotalRoutes(); total != 0 {
		t.Errorf("expected 0 routes, got %d", total)
	}
}

func TestSaveSnapshot_AtomicWrite(t *testing.T) {
	store := New()
	target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
	store.CreateTrie(target)
	store.UpsertRoute(target, netip.MustParsePrefix("10.0.0.0/8"), model.BGPPath{
		NextHop: netip.MustParseAddr("10.0.0.1"),
		Origin:  "igp",
		IsBest:  true,
		PathID:  1,
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.snapshot")

	// Save twice -- the second should atomically replace the first.
	if err := store.SaveSnapshot(path); err != nil {
		t.Fatalf("first SaveSnapshot: %v", err)
	}
	if err := store.SaveSnapshot(path); err != nil {
		t.Fatalf("second SaveSnapshot: %v", err)
	}

	// No leftover temp files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "atomic.snapshot" {
			t.Errorf("unexpected file in dir: %s", e.Name())
		}
	}
}

func TestStartPeriodicSnapshot(t *testing.T) {
	store := New()
	target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
	store.CreateTrie(target)
	store.UpsertRoute(target, netip.MustParsePrefix("10.0.0.0/8"), model.BGPPath{
		NextHop: netip.MustParseAddr("10.0.0.1"),
		Origin:  "igp",
		IsBest:  true,
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "periodic.snapshot")

	ctx, cancel := context.WithCancel(context.Background())
	store.StartPeriodicSnapshot(ctx, path, 50*time.Millisecond)

	// Wait for at least one tick.
	time.Sleep(150 * time.Millisecond)
	cancel()
	// Give the goroutine time to write the final snapshot.
	time.Sleep(100 * time.Millisecond)

	// The snapshot file should exist.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("periodic snapshot file not found: %v", err)
	}

	// Verify it loads correctly.
	store2 := New()
	if err := store2.LoadSnapshot(path); err != nil {
		t.Fatalf("LoadSnapshot after periodic: %v", err)
	}
	if total := store2.TotalRoutes(); total != 1 {
		t.Errorf("expected 1 route after periodic reload, got %d", total)
	}
}

func TestSaveSnapshot_LPMLookupAfterLoad(t *testing.T) {
	// Verify that LPM lookups work correctly on the reloaded trie.
	store := New()
	target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
	store.CreateTrie(target)
	store.UpsertRoute(target, netip.MustParsePrefix("10.0.0.0/8"), model.BGPPath{
		NextHop: netip.MustParseAddr("10.0.0.1"),
		Origin:  "igp",
		IsBest:  true,
		PathID:  1,
	})
	store.UpsertRoute(target, netip.MustParsePrefix("10.1.0.0/16"), model.BGPPath{
		NextHop: netip.MustParseAddr("10.0.0.2"),
		Origin:  "igp",
		IsBest:  true,
		PathID:  2,
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "lpm.snapshot")

	if err := store.SaveSnapshot(path); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	store2 := New()
	if err := store2.LoadSnapshot(path); err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}

	// LPM for 10.1.2.3 should match 10.1.0.0/16 (more specific).
	entry := store2.LookupLPM(target, netip.MustParseAddr("10.1.2.3"))
	if entry == nil {
		t.Fatal("expected LPM match for 10.1.2.3")
	}
	if entry.Prefix != netip.MustParsePrefix("10.1.0.0/16") {
		t.Errorf("expected 10.1.0.0/16, got %s", entry.Prefix)
	}

	// LPM for 10.2.0.1 should match 10.0.0.0/8.
	entry2 := store2.LookupLPM(target, netip.MustParseAddr("10.2.0.1"))
	if entry2 == nil {
		t.Fatal("expected LPM match for 10.2.0.1")
	}
	if entry2.Prefix != netip.MustParsePrefix("10.0.0.0/8") {
		t.Errorf("expected 10.0.0.0/8, got %s", entry2.Prefix)
	}
}

// assertPathEqual compares two BGPPath values field by field for test clarity.
func assertPathEqual(t *testing.T, label string, want, got model.BGPPath) {
	t.Helper()

	if got.NextHop != want.NextHop {
		t.Errorf("%s: NextHop = %s, want %s", label, got.NextHop, want.NextHop)
	}
	if got.Origin != want.Origin {
		t.Errorf("%s: Origin = %s, want %s", label, got.Origin, want.Origin)
	}
	if got.MED != want.MED {
		t.Errorf("%s: MED = %d, want %d", label, got.MED, want.MED)
	}
	if got.MEDPresent != want.MEDPresent {
		t.Errorf("%s: MEDPresent = %v, want %v", label, got.MEDPresent, want.MEDPresent)
	}
	if got.LocalPref != want.LocalPref {
		t.Errorf("%s: LocalPref = %d, want %d", label, got.LocalPref, want.LocalPref)
	}
	if got.LocalPrefPresent != want.LocalPrefPresent {
		t.Errorf("%s: LocalPrefPresent = %v, want %v", label, got.LocalPrefPresent, want.LocalPrefPresent)
	}
	if got.IsBest != want.IsBest {
		t.Errorf("%s: IsBest = %v, want %v", label, got.IsBest, want.IsBest)
	}
	if got.PathID != want.PathID {
		t.Errorf("%s: PathID = %d, want %d", label, got.PathID, want.PathID)
	}
	if got.AtomicAggregate != want.AtomicAggregate {
		t.Errorf("%s: AtomicAggregate = %v, want %v", label, got.AtomicAggregate, want.AtomicAggregate)
	}
	if got.Age != want.Age {
		t.Errorf("%s: Age = %v, want %v", label, got.Age, want.Age)
	}
	if !got.ReceivedAt.Equal(want.ReceivedAt) {
		t.Errorf("%s: ReceivedAt = %v, want %v", label, got.ReceivedAt, want.ReceivedAt)
	}

	// AS path.
	if len(got.ASPath) != len(want.ASPath) {
		t.Errorf("%s: len(ASPath) = %d, want %d", label, len(got.ASPath), len(want.ASPath))
	} else {
		for i := range want.ASPath {
			if got.ASPath[i].Type != want.ASPath[i].Type {
				t.Errorf("%s: ASPath[%d].Type = %s, want %s", label, i, got.ASPath[i].Type, want.ASPath[i].Type)
			}
			if len(got.ASPath[i].ASNs) != len(want.ASPath[i].ASNs) {
				t.Errorf("%s: len(ASPath[%d].ASNs) = %d, want %d", label, i, len(got.ASPath[i].ASNs), len(want.ASPath[i].ASNs))
			}
		}
	}

	// Communities.
	if len(got.Communities) != len(want.Communities) {
		t.Errorf("%s: len(Communities) = %d, want %d", label, len(got.Communities), len(want.Communities))
	} else {
		for i := range want.Communities {
			if got.Communities[i].Value != want.Communities[i].Value {
				t.Errorf("%s: Communities[%d] = %d, want %d", label, i, got.Communities[i].Value, want.Communities[i].Value)
			}
		}
	}

	// Extended communities.
	if len(got.ExtendedCommunities) != len(want.ExtendedCommunities) {
		t.Errorf("%s: len(ExtendedCommunities) = %d, want %d", label, len(got.ExtendedCommunities), len(want.ExtendedCommunities))
	} else {
		for i := range want.ExtendedCommunities {
			if got.ExtendedCommunities[i].Raw != want.ExtendedCommunities[i].Raw {
				t.Errorf("%s: ExtendedCommunities[%d].Raw mismatch", label, i)
			}
		}
	}

	// Large communities.
	if len(got.LargeCommunities) != len(want.LargeCommunities) {
		t.Errorf("%s: len(LargeCommunities) = %d, want %d", label, len(got.LargeCommunities), len(want.LargeCommunities))
	} else {
		for i := range want.LargeCommunities {
			if got.LargeCommunities[i] != want.LargeCommunities[i] {
				t.Errorf("%s: LargeCommunities[%d] = %+v, want %+v", label, i, got.LargeCommunities[i], want.LargeCommunities[i])
			}
		}
	}

	// Aggregator.
	if (got.Aggregator == nil) != (want.Aggregator == nil) {
		t.Errorf("%s: Aggregator nil mismatch: got=%v, want=%v", label, got.Aggregator == nil, want.Aggregator == nil)
	} else if want.Aggregator != nil {
		if got.Aggregator.ASN != want.Aggregator.ASN {
			t.Errorf("%s: Aggregator.ASN = %d, want %d", label, got.Aggregator.ASN, want.Aggregator.ASN)
		}
		if got.Aggregator.Address != want.Aggregator.Address {
			t.Errorf("%s: Aggregator.Address = %s, want %s", label, got.Aggregator.Address, want.Aggregator.Address)
		}
	}
}
