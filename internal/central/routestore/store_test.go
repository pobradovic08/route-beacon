package routestore

import (
	"net/netip"
	"testing"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

func TestUpsertAndLookup(t *testing.T) {
	t.Run("upsert a route and verify exact lookup returns it", func(t *testing.T) {
		store := New()
		target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
		store.CreateTrie(target)

		prefix := netip.MustParsePrefix("10.0.0.0/24")
		path := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.1"),
			Origin:  "igp",
			IsBest:  true,
			PathID:  1,
			ASPath: []model.ASPathSegment{
				{Type: "AS_SEQUENCE", ASNs: []uint32{65000, 65001}},
			},
		}

		store.UpsertRoute(target, prefix, path)

		// Exact lookup should return the entry.
		entry := store.LookupExact(target, prefix)
		if entry == nil {
			t.Fatal("expected non-nil entry from LookupExact")
		}
		if entry.Prefix != prefix {
			t.Errorf("entry.Prefix = %s, want %s", entry.Prefix, prefix)
		}
		if len(entry.Paths) != 1 {
			t.Fatalf("expected 1 path, got %d", len(entry.Paths))
		}
		if entry.Paths[0].NextHop != path.NextHop {
			t.Errorf("path NextHop = %s, want %s", entry.Paths[0].NextHop, path.NextHop)
		}
		if entry.Paths[0].PathID != path.PathID {
			t.Errorf("path PathID = %d, want %d", entry.Paths[0].PathID, path.PathID)
		}

		// Count should be 1.
		if c := store.Count(target); c != 1 {
			t.Errorf("count = %d, want 1", c)
		}
	})

	t.Run("upsert with same PathID replaces existing path", func(t *testing.T) {
		store := New()
		target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
		store.CreateTrie(target)

		prefix := netip.MustParsePrefix("10.0.0.0/24")
		path1 := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.1"),
			Origin:  "igp",
			PathID:  1,
		}
		path1Updated := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.99"),
			Origin:  "egp",
			PathID:  1,
		}

		store.UpsertRoute(target, prefix, path1)
		store.UpsertRoute(target, prefix, path1Updated)

		entry := store.LookupExact(target, prefix)
		if entry == nil {
			t.Fatal("expected non-nil entry")
		}
		// Should still be 1 path since PathID matched.
		if len(entry.Paths) != 1 {
			t.Fatalf("expected 1 path after upsert with same PathID, got %d", len(entry.Paths))
		}
		if entry.Paths[0].NextHop != path1Updated.NextHop {
			t.Errorf("path NextHop = %s, want %s (updated)", entry.Paths[0].NextHop, path1Updated.NextHop)
		}
	})

	t.Run("upsert with different PathID appends", func(t *testing.T) {
		store := New()
		target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
		store.CreateTrie(target)

		prefix := netip.MustParsePrefix("10.0.0.0/24")
		path1 := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.1"),
			Origin:  "igp",
			PathID:  1,
		}
		path2 := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.2"),
			Origin:  "igp",
			PathID:  2,
		}

		store.UpsertRoute(target, prefix, path1)
		store.UpsertRoute(target, prefix, path2)

		entry := store.LookupExact(target, prefix)
		if entry == nil {
			t.Fatal("expected non-nil entry")
		}
		if len(entry.Paths) != 2 {
			t.Fatalf("expected 2 paths, got %d", len(entry.Paths))
		}
	})

	t.Run("lookup for non-existent prefix returns nil", func(t *testing.T) {
		store := New()
		target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
		store.CreateTrie(target)

		prefix := netip.MustParsePrefix("192.168.0.0/16")
		entry := store.LookupExact(target, prefix)
		if entry != nil {
			t.Errorf("expected nil for non-existent prefix, got %+v", entry)
		}
	})
}

func TestUpsertToMissingTrie(t *testing.T) {
	t.Run("upsert to non-existent trie does not panic", func(t *testing.T) {
		store := New()
		target := model.LGTargetID{CollectorID: "nonexistent", SessionID: "sess-1"}

		prefix := netip.MustParsePrefix("10.0.0.0/24")
		path := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.1"),
			Origin:  "igp",
			PathID:  1,
		}

		// This should not panic; it should silently return.
		store.UpsertRoute(target, prefix, path)

		// Lookup should also return nil since there is no trie.
		entry := store.LookupExact(target, prefix)
		if entry != nil {
			t.Errorf("expected nil for missing trie, got %+v", entry)
		}

		// Count should return 0 for a missing trie.
		if c := store.Count(target); c != 0 {
			t.Errorf("count = %d, want 0 for missing trie", c)
		}
	})
}

func TestWithdrawRoute(t *testing.T) {
	t.Run("withdraw the only path removes the prefix", func(t *testing.T) {
		store := New()
		target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
		store.CreateTrie(target)

		prefix := netip.MustParsePrefix("10.0.0.0/24")
		path := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.1"),
			Origin:  "igp",
			PathID:  1,
		}

		store.UpsertRoute(target, prefix, path)

		// Verify the route exists.
		entry := store.LookupExact(target, prefix)
		if entry == nil {
			t.Fatal("expected route to exist before withdrawal")
		}

		// Withdraw the route.
		store.WithdrawRoute(target, prefix, path.PathID)

		// Verify lookup returns nil after withdrawal.
		entry = store.LookupExact(target, prefix)
		if entry != nil {
			t.Errorf("expected nil after withdrawal, got %+v", entry)
		}

		// Count should be 0.
		if c := store.Count(target); c != 0 {
			t.Errorf("count = %d, want 0 after withdrawal", c)
		}
	})

	t.Run("withdraw one of two paths keeps the prefix", func(t *testing.T) {
		store := New()
		target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
		store.CreateTrie(target)

		prefix := netip.MustParsePrefix("10.0.0.0/24")
		path1 := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.1"),
			Origin:  "igp",
			PathID:  1,
		}
		path2 := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.2"),
			Origin:  "igp",
			PathID:  2,
		}

		store.UpsertRoute(target, prefix, path1)
		store.UpsertRoute(target, prefix, path2)

		// Withdraw path1.
		store.WithdrawRoute(target, prefix, path1.PathID)

		entry := store.LookupExact(target, prefix)
		if entry == nil {
			t.Fatal("expected entry to still exist after partial withdrawal")
		}
		if len(entry.Paths) != 1 {
			t.Fatalf("expected 1 remaining path, got %d", len(entry.Paths))
		}
		if entry.Paths[0].PathID != path2.PathID {
			t.Errorf("remaining path PathID = %d, want %d", entry.Paths[0].PathID, path2.PathID)
		}

		// Count should still be 1 (one prefix).
		if c := store.Count(target); c != 1 {
			t.Errorf("count = %d, want 1", c)
		}
	})

	t.Run("withdraw with pathID 0 removes all paths", func(t *testing.T) {
		store := New()
		target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
		store.CreateTrie(target)

		prefix := netip.MustParsePrefix("10.0.0.0/24")
		path1 := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.1"),
			Origin:  "igp",
			PathID:  1,
		}
		path2 := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.2"),
			Origin:  "igp",
			PathID:  2,
		}

		store.UpsertRoute(target, prefix, path1)
		store.UpsertRoute(target, prefix, path2)

		// Withdraw with pathID=0 should remove the entire prefix.
		store.WithdrawRoute(target, prefix, 0)

		entry := store.LookupExact(target, prefix)
		if entry != nil {
			t.Errorf("expected nil after pathID=0 withdrawal, got %+v", entry)
		}
		if c := store.Count(target); c != 0 {
			t.Errorf("count = %d, want 0", c)
		}
	})
}

func TestLPMLookup(t *testing.T) {
	t.Run("LPM returns most specific matching prefix", func(t *testing.T) {
		store := New()
		target := model.LGTargetID{CollectorID: "col-1", SessionID: "sess-1"}
		store.CreateTrie(target)

		// Insert a /16 route.
		prefix16 := netip.MustParsePrefix("10.1.0.0/16")
		path16 := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.1"),
			Origin:  "igp",
			PathID:  1,
		}
		store.UpsertRoute(target, prefix16, path16)

		// Insert a /24 route within the /16.
		prefix24 := netip.MustParsePrefix("10.1.1.0/24")
		path24 := model.BGPPath{
			NextHop: netip.MustParseAddr("10.0.0.2"),
			Origin:  "igp",
			PathID:  2,
		}
		store.UpsertRoute(target, prefix24, path24)

		// LPM for an IP in the /24 should return the /24 (more specific).
		addr := netip.MustParseAddr("10.1.1.100")
		entry := store.LookupLPM(target, addr)
		if entry == nil {
			t.Fatal("expected LPM match for 10.1.1.100")
		}
		if entry.Prefix != prefix24 {
			t.Errorf("LPM result = %s, want %s", entry.Prefix, prefix24)
		}

		// LPM for an IP in the /16 but outside the /24 should return the /16.
		addr2 := netip.MustParseAddr("10.1.2.50")
		entry2 := store.LookupLPM(target, addr2)
		if entry2 == nil {
			t.Fatal("expected LPM match for 10.1.2.50")
		}
		if entry2.Prefix != prefix16 {
			t.Errorf("LPM result = %s, want %s", entry2.Prefix, prefix16)
		}

		// LPM for an IP completely outside both prefixes should return nil.
		addr3 := netip.MustParseAddr("192.168.1.1")
		entry3 := store.LookupLPM(target, addr3)
		if entry3 != nil {
			t.Errorf("expected nil LPM for 192.168.1.1, got prefix %s", entry3.Prefix)
		}
	})

	t.Run("LPM on missing trie returns nil", func(t *testing.T) {
		store := New()
		target := model.LGTargetID{CollectorID: "nonexistent", SessionID: "sess-1"}

		entry := store.LookupLPM(target, netip.MustParseAddr("10.0.0.1"))
		if entry != nil {
			t.Errorf("expected nil for missing trie, got %+v", entry)
		}
	})
}
