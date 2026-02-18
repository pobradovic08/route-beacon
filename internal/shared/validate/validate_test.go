package validate

import (
	"net/netip"
	"testing"
)

func TestParsePrefix(t *testing.T) {
	t.Run("bare IPv4 becomes /32", func(t *testing.T) {
		prefix, bareIP, err := ParsePrefix("8.8.8.8")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bareIP {
			t.Error("expected bareIP=true for bare IPv4 address")
		}
		if prefix.Bits() != 32 {
			t.Errorf("expected /32, got /%d", prefix.Bits())
		}
		if prefix.Addr() != netip.MustParseAddr("8.8.8.8") {
			t.Errorf("expected addr 8.8.8.8, got %s", prefix.Addr())
		}
	})

	t.Run("bare IPv6 becomes /128", func(t *testing.T) {
		prefix, bareIP, err := ParsePrefix("2001:4860::8888")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bareIP {
			t.Error("expected bareIP=true for bare IPv6 address")
		}
		if prefix.Bits() != 128 {
			t.Errorf("expected /128, got /%d", prefix.Bits())
		}
	})

	t.Run("valid IPv4 CIDR", func(t *testing.T) {
		prefix, bareIP, err := ParsePrefix("192.0.2.0/24")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bareIP {
			t.Error("expected bareIP=false for CIDR prefix")
		}
		if prefix.Bits() != 24 {
			t.Errorf("expected /24, got /%d", prefix.Bits())
		}
		expected := netip.MustParsePrefix("192.0.2.0/24")
		if prefix != expected {
			t.Errorf("expected %s, got %s", expected, prefix)
		}
	})

	t.Run("valid IPv6 CIDR", func(t *testing.T) {
		prefix, bareIP, err := ParsePrefix("2001:db8::/32")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bareIP {
			t.Error("expected bareIP=false for CIDR prefix")
		}
		if prefix.Bits() != 32 {
			t.Errorf("expected /32, got /%d", prefix.Bits())
		}
	})

	t.Run("CIDR is masked to network address", func(t *testing.T) {
		// 10.1.2.3/8 should be masked to 10.0.0.0/8
		prefix, _, err := ParsePrefix("10.1.2.3/8")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := netip.MustParsePrefix("10.0.0.0/8")
		if prefix != expected {
			t.Errorf("expected masked prefix %s, got %s", expected, prefix)
		}
	})

	t.Run("empty string returns error", func(t *testing.T) {
		_, _, err := ParsePrefix("")
		if err == nil {
			t.Fatal("expected error for empty string, got nil")
		}
	})

	t.Run("whitespace-only string returns error", func(t *testing.T) {
		_, _, err := ParsePrefix("   ")
		if err == nil {
			t.Fatal("expected error for whitespace-only string, got nil")
		}
	})

	t.Run("invalid IP returns error", func(t *testing.T) {
		_, _, err := ParsePrefix("not-an-ip")
		if err == nil {
			t.Fatal("expected error for invalid input, got nil")
		}
	})

	t.Run("invalid CIDR returns error", func(t *testing.T) {
		_, _, err := ParsePrefix("10.0.0.0/33")
		if err == nil {
			t.Fatal("expected error for invalid CIDR /33, got nil")
		}
	})

	t.Run("leading/trailing whitespace is trimmed", func(t *testing.T) {
		prefix, bareIP, err := ParsePrefix("  8.8.8.8  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bareIP {
			t.Error("expected bareIP=true")
		}
		if prefix.Addr() != netip.MustParseAddr("8.8.8.8") {
			t.Errorf("expected 8.8.8.8, got %s", prefix.Addr())
		}
	})
}

func TestValidateDestination(t *testing.T) {
	// Addresses that should be REJECTED (restricted ranges).
	rejectedCases := []struct {
		name string
		addr string
	}{
		// RFC 1918
		{"RFC1918 10.x", "10.0.0.1"},
		{"RFC1918 10.255.x", "10.255.255.255"},
		{"RFC1918 172.16.x", "172.16.0.1"},
		{"RFC1918 172.31.x", "172.31.255.255"},
		{"RFC1918 192.168.x", "192.168.1.1"},

		// Loopback
		{"IPv4 loopback", "127.0.0.1"},
		{"IPv4 loopback high", "127.255.255.254"},
		{"IPv6 loopback", "::1"},

		// Link-local
		{"IPv4 link-local", "169.254.1.1"},
		{"IPv6 link-local", "fe80::1"},

		// Multicast
		{"IPv4 multicast", "224.0.0.1"},
		{"IPv4 multicast high", "239.255.255.255"},
		{"IPv6 multicast", "ff02::1"},

		// Documentation ranges
		{"TEST-NET-1", "192.0.2.1"},
		{"TEST-NET-2", "198.51.100.1"},
		{"TEST-NET-3", "203.0.113.1"},
		{"IPv6 documentation", "2001:db8::1"},

		// RFC 6598 shared address
		{"RFC6598 100.64.x", "100.64.0.1"},
		{"RFC6598 100.127.x", "100.127.255.254"},

		// Reserved
		{"IPv4 reserved 240.x", "240.0.0.1"},

		// "This network"
		{"this network 0.x", "0.0.0.1"},
	}

	for _, tc := range rejectedCases {
		t.Run("reject/"+tc.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tc.addr)
			err := ValidateDestination(addr)
			if err == nil {
				t.Errorf("expected %s (%s) to be rejected, but it was allowed", tc.addr, tc.name)
			}
		})
	}

	// Addresses that should be ALLOWED (public IPs).
	allowedCases := []struct {
		name string
		addr string
	}{
		{"Google DNS IPv4", "8.8.8.8"},
		{"Cloudflare DNS IPv4", "1.1.1.1"},
		{"Google DNS IPv6", "2001:4860:4860::8888"},
		{"Cloudflare DNS IPv6", "2606:4700:4700::1111"},
		{"random public IPv4", "93.184.216.34"},
	}

	for _, tc := range allowedCases {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tc.addr)
			err := ValidateDestination(addr)
			if err != nil {
				t.Errorf("expected %s (%s) to be allowed, but got error: %v", tc.addr, tc.name, err)
			}
		})
	}

	t.Run("invalid address returns error", func(t *testing.T) {
		var addr netip.Addr // zero value is invalid
		err := ValidateDestination(addr)
		if err == nil {
			t.Error("expected error for invalid address, got nil")
		}
	})

	t.Run("unspecified IPv4 returns error", func(t *testing.T) {
		addr := netip.MustParseAddr("0.0.0.0")
		err := ValidateDestination(addr)
		if err == nil {
			t.Error("expected error for unspecified address 0.0.0.0, got nil")
		}
	})

	t.Run("unspecified IPv6 returns error", func(t *testing.T) {
		addr := netip.MustParseAddr("::")
		err := ValidateDestination(addr)
		if err == nil {
			t.Error("expected error for unspecified address ::, got nil")
		}
	})
}

func TestValidatePingParams(t *testing.T) {
	const maxCount uint8 = 10
	const maxTimeout uint32 = 30000

	t.Run("count=0 returns error", func(t *testing.T) {
		err := ValidatePingParams(0, 5000, maxCount, maxTimeout)
		if err == nil {
			t.Error("expected error for count=0, got nil")
		}
	})

	t.Run("count within range is ok", func(t *testing.T) {
		err := ValidatePingParams(5, 5000, maxCount, maxTimeout)
		if err != nil {
			t.Errorf("unexpected error for valid count: %v", err)
		}
	})

	t.Run("count=1 minimum is ok", func(t *testing.T) {
		err := ValidatePingParams(1, 1000, maxCount, maxTimeout)
		if err != nil {
			t.Errorf("unexpected error for count=1: %v", err)
		}
	})

	t.Run("count at max is ok", func(t *testing.T) {
		err := ValidatePingParams(maxCount, 5000, maxCount, maxTimeout)
		if err != nil {
			t.Errorf("unexpected error for count=max: %v", err)
		}
	})

	t.Run("count exceeds max returns error", func(t *testing.T) {
		err := ValidatePingParams(maxCount+1, 5000, maxCount, maxTimeout)
		if err == nil {
			t.Error("expected error for count exceeding max, got nil")
		}
	})

	t.Run("timeout below minimum returns error", func(t *testing.T) {
		err := ValidatePingParams(1, 999, maxCount, maxTimeout)
		if err == nil {
			t.Error("expected error for timeout=999ms (below 1000ms minimum), got nil")
		}
	})

	t.Run("timeout=0 returns error", func(t *testing.T) {
		err := ValidatePingParams(1, 0, maxCount, maxTimeout)
		if err == nil {
			t.Error("expected error for timeout=0, got nil")
		}
	})

	t.Run("timeout at minimum is ok", func(t *testing.T) {
		err := ValidatePingParams(1, 1000, maxCount, maxTimeout)
		if err != nil {
			t.Errorf("unexpected error for timeout=1000ms: %v", err)
		}
	})

	t.Run("timeout at max is ok", func(t *testing.T) {
		err := ValidatePingParams(1, maxTimeout, maxCount, maxTimeout)
		if err != nil {
			t.Errorf("unexpected error for timeout=max: %v", err)
		}
	})

	t.Run("timeout exceeds max returns error", func(t *testing.T) {
		err := ValidatePingParams(1, maxTimeout+1, maxCount, maxTimeout)
		if err == nil {
			t.Error("expected error for timeout exceeding max, got nil")
		}
	})
}

func TestValidateTracerouteParams(t *testing.T) {
	const maxMaxHops uint8 = 64
	const maxTimeout uint32 = 30000

	t.Run("maxHops=0 returns error", func(t *testing.T) {
		err := ValidateTracerouteParams(0, 5000, maxMaxHops, maxTimeout)
		if err == nil {
			t.Error("expected error for maxHops=0, got nil")
		}
	})

	t.Run("maxHops within range is ok", func(t *testing.T) {
		err := ValidateTracerouteParams(30, 5000, maxMaxHops, maxTimeout)
		if err != nil {
			t.Errorf("unexpected error for valid maxHops: %v", err)
		}
	})

	t.Run("maxHops=1 minimum is ok", func(t *testing.T) {
		err := ValidateTracerouteParams(1, 1000, maxMaxHops, maxTimeout)
		if err != nil {
			t.Errorf("unexpected error for maxHops=1: %v", err)
		}
	})

	t.Run("maxHops at max is ok", func(t *testing.T) {
		err := ValidateTracerouteParams(maxMaxHops, 5000, maxMaxHops, maxTimeout)
		if err != nil {
			t.Errorf("unexpected error for maxHops=max: %v", err)
		}
	})

	t.Run("maxHops exceeds max returns error", func(t *testing.T) {
		err := ValidateTracerouteParams(maxMaxHops+1, 5000, maxMaxHops, maxTimeout)
		if err == nil {
			t.Error("expected error for maxHops exceeding max, got nil")
		}
	})

	t.Run("timeout below minimum returns error", func(t *testing.T) {
		err := ValidateTracerouteParams(1, 999, maxMaxHops, maxTimeout)
		if err == nil {
			t.Error("expected error for timeout=999ms (below 1000ms minimum), got nil")
		}
	})

	t.Run("timeout=0 returns error", func(t *testing.T) {
		err := ValidateTracerouteParams(1, 0, maxMaxHops, maxTimeout)
		if err == nil {
			t.Error("expected error for timeout=0, got nil")
		}
	})

	t.Run("timeout at minimum is ok", func(t *testing.T) {
		err := ValidateTracerouteParams(1, 1000, maxMaxHops, maxTimeout)
		if err != nil {
			t.Errorf("unexpected error for timeout=1000ms: %v", err)
		}
	})

	t.Run("timeout at max is ok", func(t *testing.T) {
		err := ValidateTracerouteParams(1, maxTimeout, maxMaxHops, maxTimeout)
		if err != nil {
			t.Errorf("unexpected error for timeout=max: %v", err)
		}
	})

	t.Run("timeout exceeds max returns error", func(t *testing.T) {
		err := ValidateTracerouteParams(1, maxTimeout+1, maxMaxHops, maxTimeout)
		if err == nil {
			t.Error("expected error for timeout exceeding max, got nil")
		}
	})
}
