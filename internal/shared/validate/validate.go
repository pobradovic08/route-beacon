package validate

import (
	"fmt"
	"net/netip"
	"strings"
)

// restrictedV4 contains IPv4 ranges that are not allowed as diagnostic command destinations.
var restrictedV4 = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),       // RFC 1918
	netip.MustParsePrefix("172.16.0.0/12"),     // RFC 1918
	netip.MustParsePrefix("192.168.0.0/16"),    // RFC 1918
	netip.MustParsePrefix("127.0.0.0/8"),       // Loopback
	netip.MustParsePrefix("169.254.0.0/16"),    // Link-local
	netip.MustParsePrefix("224.0.0.0/4"),       // Multicast
	netip.MustParsePrefix("240.0.0.0/4"),       // Reserved
	netip.MustParsePrefix("100.64.0.0/10"),     // RFC 6598 shared address
	netip.MustParsePrefix("192.0.2.0/24"),      // TEST-NET-1 (documentation)
	netip.MustParsePrefix("198.51.100.0/24"),   // TEST-NET-2 (documentation)
	netip.MustParsePrefix("203.0.113.0/24"),    // TEST-NET-3 (documentation)
	netip.MustParsePrefix("0.0.0.0/8"),         // "This network"
}

// restrictedV6 contains IPv6 ranges that are not allowed as diagnostic command destinations.
var restrictedV6 = []netip.Prefix{
	netip.MustParsePrefix("::1/128"),           // Loopback
	netip.MustParsePrefix("fe80::/10"),         // Link-local
	netip.MustParsePrefix("fc00::/7"),          // Unique local
	netip.MustParsePrefix("ff00::/8"),          // Multicast
	netip.MustParsePrefix("2001:db8::/32"),     // Documentation
	netip.MustParsePrefix("::ffff:0:0/96"),     // IPv4-mapped IPv6
	netip.MustParsePrefix("::/128"),            // Unspecified
}

// ParsePrefix parses a string into a netip.Prefix. It auto-detects IPv4/IPv6
// and handles bare IP addresses by treating them as /32 (IPv4) or /128 (IPv6).
// Returns the prefix, whether it was a bare IP (for LPM matching), and any error.
func ParsePrefix(s string) (netip.Prefix, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Prefix{}, false, fmt.Errorf("empty prefix")
	}

	// Try parsing as CIDR prefix first
	if strings.Contains(s, "/") {
		prefix, err := netip.ParsePrefix(s)
		if err != nil {
			return netip.Prefix{}, false, fmt.Errorf("invalid prefix %q: %w", s, err)
		}
		return prefix.Masked(), false, nil
	}

	// Bare IP address — convert to host prefix
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, false, fmt.Errorf("invalid IP address %q: %w", s, err)
	}

	bits := 32
	if addr.Is6() {
		bits = 128
	}
	prefix := netip.PrefixFrom(addr, bits)
	return prefix, true, nil
}

// ValidateDestination checks if an IP address is allowed as a diagnostic
// command destination. Returns an error describing why the address is
// restricted, or nil if the address is allowed.
func ValidateDestination(addr netip.Addr) error {
	if !addr.IsValid() {
		return fmt.Errorf("invalid IP address")
	}

	if addr.IsUnspecified() {
		return fmt.Errorf("address %s is the unspecified address", addr)
	}

	restricted := restrictedV4
	if addr.Is6() && !addr.Is4In6() {
		restricted = restrictedV6
	} else if addr.Is4In6() {
		// For IPv4-mapped IPv6, check against both lists
		// The v6 list already includes ::ffff:0:0/96
		for _, prefix := range restrictedV6 {
			if prefix.Contains(addr) {
				return fmt.Errorf("address %s is in restricted range %s", addr, prefix)
			}
		}
		// Also check the unmapped v4 address against v4 restrictions
		addr4 := addr.Unmap()
		for _, prefix := range restrictedV4 {
			if prefix.Contains(addr4) {
				return fmt.Errorf("address %s is in restricted range %s (mapped from %s)", addr, prefix, addr4)
			}
		}
		return nil
	}

	for _, prefix := range restricted {
		if prefix.Contains(addr) {
			return fmt.Errorf("address %s is in restricted range %s", addr, prefix)
		}
	}

	return nil
}

// ValidatePingParams validates ping command parameters.
// Returns an error if any parameter is out of the allowed range.
func ValidatePingParams(count uint8, timeoutMs uint32, maxCount uint8, maxTimeoutMs uint32) error {
	if count == 0 {
		return fmt.Errorf("count must be at least 1")
	}
	if count > maxCount {
		return fmt.Errorf("count %d exceeds maximum %d", count, maxCount)
	}
	if timeoutMs < 1000 {
		return fmt.Errorf("timeout must be at least 1000ms")
	}
	if timeoutMs > maxTimeoutMs {
		return fmt.Errorf("timeout %dms exceeds maximum %dms", timeoutMs, maxTimeoutMs)
	}
	return nil
}

// ValidateTracerouteParams validates traceroute command parameters.
func ValidateTracerouteParams(maxHops uint8, timeoutMs uint32, maxMaxHops uint8, maxTimeoutMs uint32) error {
	if maxHops == 0 {
		return fmt.Errorf("max_hops must be at least 1")
	}
	if maxHops > maxMaxHops {
		return fmt.Errorf("max_hops %d exceeds maximum %d", maxHops, maxMaxHops)
	}
	if timeoutMs < 1000 {
		return fmt.Errorf("timeout must be at least 1000ms")
	}
	if timeoutMs > maxTimeoutMs {
		return fmt.Errorf("timeout %dms exceeds maximum %dms", timeoutMs, maxTimeoutMs)
	}
	return nil
}
