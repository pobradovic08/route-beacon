package commands

import (
	"strconv"
	"testing"
	"time"
)

func TestParseHopLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantOK    bool
		wantTTL   uint8
		wantProbe int // expected number of probes
	}{
		{
			name:      "responding hop with three probes",
			line:      " 1  192.168.1.1  0.529 ms  0.402 ms  0.378 ms",
			wantOK:    true,
			wantTTL:   1,
			wantProbe: 3,
		},
		{
			name:      "timeout hop",
			line:      " 3  * * *",
			wantOK:    true,
			wantTTL:   3,
			wantProbe: 3,
		},
		{
			name:      "mixed probes with timeout",
			line:      " 5  172.16.0.1  10.123 ms  10.456 ms  *",
			wantOK:    true,
			wantTTL:   5,
			wantProbe: 3,
		},
		{
			name:      "double-digit hop number",
			line:      "15  10.0.0.1  4.146 ms  4.090 ms  3.998 ms",
			wantOK:    true,
			wantTTL:   15,
			wantProbe: 3,
		},
		{
			name:      "address change within hop",
			line:      " 4  10.0.0.1  4.146 ms 10.0.0.2  5.789 ms  10.0.0.1  4.090 ms",
			wantOK:    true,
			wantTTL:   4,
			wantProbe: 3,
		},
		{
			name:   "empty line",
			line:   "",
			wantOK: false,
		},
		{
			name:   "traceroute header line",
			line:   "traceroute to 8.8.8.8 (8.8.8.8), 30 hops max, 60 byte packets",
			wantOK: false,
		},
		{
			name:   "blank line with whitespace only",
			line:   "   ",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hop, ok := parseHopLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseHopLine(%q): ok = %v, want %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if hop.TTL != tt.wantTTL {
				t.Errorf("TTL = %d, want %d", hop.TTL, tt.wantTTL)
			}
			if len(hop.Probes) != tt.wantProbe {
				t.Errorf("len(Probes) = %d, want %d", len(hop.Probes), tt.wantProbe)
			}
		})
	}
}

func TestParseProbes(t *testing.T) {
	tests := []struct {
		name       string
		rest       string
		wantCount  int
		wantAddrs  []string
		wantRTTs   []time.Duration // zero for timeout probes
		wantSucc   []bool
	}{
		{
			name:      "three successful probes from same address",
			rest:      "192.168.1.1  0.529 ms  0.402 ms  0.378 ms",
			wantCount: 3,
			wantAddrs: []string{"192.168.1.1", "192.168.1.1", "192.168.1.1"},
			wantRTTs: []time.Duration{
				time.Duration(0.529 * float64(time.Millisecond)),
				time.Duration(0.402 * float64(time.Millisecond)),
				time.Duration(0.378 * float64(time.Millisecond)),
			},
			wantSucc: []bool{true, true, true},
		},
		{
			name:      "all timeouts",
			rest:      "* * *",
			wantCount: 3,
			wantAddrs: []string{"", "", ""},
			wantRTTs:  []time.Duration{0, 0, 0},
			wantSucc:  []bool{false, false, false},
		},
		{
			name:      "mixed success and timeout",
			rest:      "172.16.0.1  10.123 ms  10.456 ms  *",
			wantCount: 3,
			wantAddrs: []string{"172.16.0.1", "172.16.0.1", ""},
			wantRTTs: []time.Duration{
				time.Duration(10.123 * float64(time.Millisecond)),
				time.Duration(10.456 * float64(time.Millisecond)),
				0,
			},
			wantSucc: []bool{true, true, false},
		},
		{
			name:      "address change within hop",
			rest:      "10.0.0.1  4.146 ms 10.0.0.2  5.789 ms  10.0.0.1  4.090 ms",
			wantCount: 3,
			wantAddrs: []string{"10.0.0.1", "10.0.0.2", "10.0.0.1"},
			wantRTTs: []time.Duration{
				time.Duration(4.146 * float64(time.Millisecond)),
				time.Duration(5.789 * float64(time.Millisecond)),
				time.Duration(4.090 * float64(time.Millisecond)),
			},
			wantSucc: []bool{true, true, true},
		},
		{
			name:      "IPv6 address probes",
			rest:      "2001:db8::1  1.234 ms  2.345 ms  3.456 ms",
			wantCount: 3,
			wantAddrs: []string{"2001:db8::1", "2001:db8::1", "2001:db8::1"},
			wantRTTs: []time.Duration{
				time.Duration(1.234 * float64(time.Millisecond)),
				time.Duration(2.345 * float64(time.Millisecond)),
				time.Duration(3.456 * float64(time.Millisecond)),
			},
			wantSucc: []bool{true, true, true},
		},
		{
			name:      "empty rest",
			rest:      "",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probes := parseProbes(tt.rest)
			if len(probes) != tt.wantCount {
				t.Fatalf("len(probes) = %d, want %d", len(probes), tt.wantCount)
			}
			for i, p := range probes {
				if i < len(tt.wantAddrs) && p.Address != tt.wantAddrs[i] {
					t.Errorf("probes[%d].Address = %q, want %q", i, p.Address, tt.wantAddrs[i])
				}
				if i < len(tt.wantRTTs) && p.RTT != tt.wantRTTs[i] {
					t.Errorf("probes[%d].RTT = %v, want %v", i, p.RTT, tt.wantRTTs[i])
				}
				if i < len(tt.wantSucc) && p.Success != tt.wantSucc[i] {
					t.Errorf("probes[%d].Success = %v, want %v", i, p.Success, tt.wantSucc[i])
				}
			}
		})
	}
}

func TestLooksLikeIP(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"2001:db8::1", true},
		{"::1", true},
		{"fe80::1", true},
		{"2001:4860:4860::8888", true},
		{"not-an-ip", false},
		{"ms", false},
		{"*", false},
		{"", false},
		{"12.34", false},
		{"hello.world.foo.bar", false},
		{"999.999.999.999", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := looksLikeIP(tt.input)
			if got != tt.want {
				t.Errorf("looksLikeIP(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildTracerouteArgs(t *testing.T) {
	tests := []struct {
		name       string
		dest       string
		maxHops    uint8
		timeout    time.Duration
		wantDest   string
		wantHops   string
		wantNoResolve bool
	}{
		{
			name:          "default parameters",
			dest:          "8.8.8.8",
			maxHops:       30,
			timeout:       5 * time.Second,
			wantDest:      "8.8.8.8",
			wantHops:      "30",
			wantNoResolve: true,
		},
		{
			name:          "IPv6 destination with custom hops",
			dest:          "2001:4860:4860::8888",
			maxHops:       64,
			timeout:       2 * time.Second,
			wantDest:      "2001:4860:4860::8888",
			wantHops:      "64",
			wantNoResolve: true,
		},
		{
			name:          "short timeout",
			dest:          "1.1.1.1",
			maxHops:       10,
			timeout:       500 * time.Millisecond,
			wantDest:      "1.1.1.1",
			wantHops:      "10",
			wantNoResolve: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildTracerouteArgs(tt.dest, tt.maxHops, tt.timeout)

			// Verify -n flag (no DNS resolution) is present.
			foundN := false
			for _, a := range args {
				if a == "-n" {
					foundN = true
					break
				}
			}
			if tt.wantNoResolve && !foundN {
				t.Error("expected -n flag in args, not found")
			}

			// Verify -m flag and max hops value.
			foundM := false
			for i, a := range args {
				if a == "-m" && i+1 < len(args) {
					foundM = true
					if args[i+1] != tt.wantHops {
						t.Errorf("max hops = %q, want %q", args[i+1], tt.wantHops)
					}
					break
				}
			}
			if !foundM {
				t.Error("expected -m flag in args, not found")
			}

			// Verify -w flag and timeout value.
			foundW := false
			for i, a := range args {
				if a == "-w" && i+1 < len(args) {
					foundW = true
					wantTimeout := strconv.FormatFloat(tt.timeout.Seconds(), 'f', 1, 64)
					if args[i+1] != wantTimeout {
						t.Errorf("timeout = %q, want %q", args[i+1], wantTimeout)
					}
					break
				}
			}
			if !foundW {
				t.Error("expected -w flag in args, not found")
			}

			// Verify destination is last argument.
			if args[len(args)-1] != tt.wantDest {
				t.Errorf("last arg = %q, want %q", args[len(args)-1], tt.wantDest)
			}

			// Verify total argument count: -n -m <hops> -w <timeout> <dest>
			if len(args) != 6 {
				t.Errorf("len(args) = %d, want 6; args = %v", len(args), args)
			}
		})
	}
}
