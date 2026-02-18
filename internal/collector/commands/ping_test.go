package commands

import (
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestParseReplyLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantSeq uint16
		wantTTL uint8
		wantRTT time.Duration
	}{
		{
			name:    "valid IPv4 reply",
			line:    "64 bytes from 8.8.8.8: icmp_seq=1 ttl=118 time=5.26 ms",
			wantOK:  true,
			wantSeq: 1,
			wantTTL: 118,
			wantRTT: time.Duration(5.26 * float64(time.Millisecond)),
		},
		{
			name:    "valid IPv6 reply",
			line:    "64 bytes from 2001:4860:4860::8888: icmp_seq=3 ttl=57 time=12.345 ms",
			wantOK:  true,
			wantSeq: 3,
			wantTTL: 57,
			wantRTT: time.Duration(12.345 * float64(time.Millisecond)),
		},
		{
			name:    "valid reply with large seq",
			line:    "64 bytes from 1.1.1.1: icmp_seq=100 ttl=255 time=0.401 ms",
			wantOK:  true,
			wantSeq: 100,
			wantTTL: 255,
			wantRTT: time.Duration(0.401 * float64(time.Millisecond)),
		},
		{
			name:    "valid reply with hostname in from field",
			line:    "56 bytes from dns.google (8.8.8.8): icmp_seq=2 ttl=118 time=10.5 ms",
			wantOK:  true,
			wantSeq: 2,
			wantTTL: 118,
			wantRTT: time.Duration(10.5 * float64(time.Millisecond)),
		},
		{
			name:   "empty line",
			line:   "",
			wantOK: false,
		},
		{
			name:   "header line",
			line:   "PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.",
			wantOK: false,
		},
		{
			name:   "stats line (not a reply)",
			line:   "5 packets transmitted, 5 received, 0% packet loss, time 4005ms",
			wantOK: false,
		},
		{
			name:   "rtt summary line (not a reply)",
			line:   "rtt min/avg/max/mdev = 4.123/5.456/6.789/0.901 ms",
			wantOK: false,
		},
		{
			name:   "timeout line",
			line:   "Request timeout for icmp_seq 1",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply, ok := parseReplyLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseReplyLine(%q): ok = %v, want %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if reply.Seq != tt.wantSeq {
				t.Errorf("Seq = %d, want %d", reply.Seq, tt.wantSeq)
			}
			if reply.TTL != tt.wantTTL {
				t.Errorf("TTL = %d, want %d", reply.TTL, tt.wantTTL)
			}
			if reply.RTT != tt.wantRTT {
				t.Errorf("RTT = %v, want %v", reply.RTT, tt.wantRTT)
			}
			if !reply.Success {
				t.Error("Success = false, want true")
			}
		})
	}
}

func TestParseStatsLine(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		wantOK       bool
		wantSent     uint16
		wantReceived uint16
		wantLoss     float64
	}{
		{
			name:         "zero loss Linux format",
			line:         "5 packets transmitted, 5 received, 0% packet loss, time 4005ms",
			wantOK:       true,
			wantSent:     5,
			wantReceived: 5,
			wantLoss:     0,
		},
		{
			name:         "100% loss",
			line:         "5 packets transmitted, 0 received, 100% packet loss, time 4004ms",
			wantOK:       true,
			wantSent:     5,
			wantReceived: 0,
			wantLoss:     100,
		},
		{
			name:         "fractional loss",
			line:         "10 packets transmitted, 7 received, 30% packet loss, time 9012ms",
			wantOK:       true,
			wantSent:     10,
			wantReceived: 7,
			wantLoss:     30,
		},
		{
			name:         "macOS format with packets keyword",
			line:         "5 packets transmitted, 5 packets received, 0.0% packet loss",
			wantOK:       true,
			wantSent:     5,
			wantReceived: 5,
			wantLoss:     0.0,
		},
		{
			name:         "fractional loss BSD style",
			line:         "10 packets transmitted, 8 packets received, 20.0% packet loss",
			wantOK:       true,
			wantSent:     10,
			wantReceived: 8,
			wantLoss:     20.0,
		},
		{
			name:         "single packet",
			line:         "1 packet transmitted, 1 received, 0% packet loss, time 0ms",
			wantOK:       true,
			wantSent:     1,
			wantReceived: 1,
			wantLoss:     0,
		},
		{
			name:   "empty line",
			line:   "",
			wantOK: false,
		},
		{
			name:   "reply line (not stats)",
			line:   "64 bytes from 8.8.8.8: icmp_seq=1 ttl=118 time=5.26 ms",
			wantOK: false,
		},
		{
			name:   "rtt line (not stats)",
			line:   "rtt min/avg/max/mdev = 4.123/5.456/6.789/0.901 ms",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sent, received, loss, ok := parseStatsLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseStatsLine(%q): ok = %v, want %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if sent != tt.wantSent {
				t.Errorf("sent = %d, want %d", sent, tt.wantSent)
			}
			if received != tt.wantReceived {
				t.Errorf("received = %d, want %d", received, tt.wantReceived)
			}
			if loss != tt.wantLoss {
				t.Errorf("loss = %f, want %f", loss, tt.wantLoss)
			}
		})
	}
}

func TestParseRTTLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantOK     bool
		wantMin    time.Duration
		wantAvg    time.Duration
		wantMax    time.Duration
		wantStddev time.Duration
	}{
		{
			name:       "Linux rtt format",
			line:       "rtt min/avg/max/mdev = 4.123/5.456/6.789/0.901 ms",
			wantOK:     true,
			wantMin:    time.Duration(4.123 * float64(time.Millisecond)),
			wantAvg:    time.Duration(5.456 * float64(time.Millisecond)),
			wantMax:    time.Duration(6.789 * float64(time.Millisecond)),
			wantStddev: time.Duration(0.901 * float64(time.Millisecond)),
		},
		{
			name:       "macOS round-trip format",
			line:       "round-trip min/avg/max/stddev = 10.123/20.456/30.789/5.901 ms",
			wantOK:     true,
			wantMin:    time.Duration(10.123 * float64(time.Millisecond)),
			wantAvg:    time.Duration(20.456 * float64(time.Millisecond)),
			wantMax:    time.Duration(30.789 * float64(time.Millisecond)),
			wantStddev: time.Duration(5.901 * float64(time.Millisecond)),
		},
		{
			name:       "small sub-millisecond values",
			line:       "rtt min/avg/max/mdev = 0.045/0.067/0.089/0.015 ms",
			wantOK:     true,
			wantMin:    time.Duration(0.045 * float64(time.Millisecond)),
			wantAvg:    time.Duration(0.067 * float64(time.Millisecond)),
			wantMax:    time.Duration(0.089 * float64(time.Millisecond)),
			wantStddev: time.Duration(0.015 * float64(time.Millisecond)),
		},
		{
			name:   "empty line",
			line:   "",
			wantOK: false,
		},
		{
			name:   "stats line (not rtt)",
			line:   "5 packets transmitted, 5 received, 0% packet loss, time 4005ms",
			wantOK: false,
		},
		{
			name:   "reply line (not rtt)",
			line:   "64 bytes from 8.8.8.8: icmp_seq=1 ttl=118 time=5.26 ms",
			wantOK: false,
		},
		{
			name:   "malformed rtt line missing values",
			line:   "rtt min/avg/max/mdev = ms",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, avg, max, stddev, ok := parseRTTLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseRTTLine(%q): ok = %v, want %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if min != tt.wantMin {
				t.Errorf("min = %v, want %v", min, tt.wantMin)
			}
			if avg != tt.wantAvg {
				t.Errorf("avg = %v, want %v", avg, tt.wantAvg)
			}
			if max != tt.wantMax {
				t.Errorf("max = %v, want %v", max, tt.wantMax)
			}
			if stddev != tt.wantStddev {
				t.Errorf("stddev = %v, want %v", stddev, tt.wantStddev)
			}
		})
	}
}

func TestBuildPingArgs(t *testing.T) {
	tests := []struct {
		name        string
		dest        string
		count       uint8
		timeout     time.Duration
		wantContain []string
	}{
		{
			name:    "basic IPv4",
			dest:    "8.8.8.8",
			count:   5,
			timeout: 5 * time.Second,
			wantContain: []string{
				"-c", "5", "8.8.8.8",
			},
		},
		{
			name:    "basic IPv6",
			dest:    "2001:4860:4860::8888",
			count:   3,
			timeout: 2 * time.Second,
			wantContain: []string{
				"-c", "3", "2001:4860:4860::8888",
			},
		},
		{
			name:    "single packet",
			dest:    "1.1.1.1",
			count:   1,
			timeout: 10 * time.Second,
			wantContain: []string{
				"-c", "1", "1.1.1.1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildPingArgs(tt.dest, tt.count, tt.timeout)

			// Verify count argument.
			if args[0] != "-c" {
				t.Errorf("args[0] = %q, want %q", args[0], "-c")
			}
			if args[1] != strconv.Itoa(int(tt.count)) {
				t.Errorf("args[1] = %q, want %q", args[1], strconv.Itoa(int(tt.count)))
			}

			// Verify timeout flag is present.
			if args[2] != "-W" {
				t.Errorf("args[2] = %q, want %q", args[2], "-W")
			}

			// Verify timeout value is platform-appropriate.
			switch runtime.GOOS {
			case "darwin":
				// macOS uses milliseconds.
				wantTimeoutMs := strconv.FormatInt(tt.timeout.Milliseconds(), 10)
				if args[3] != wantTimeoutMs {
					t.Errorf("timeout value = %q, want %q (milliseconds for darwin)", args[3], wantTimeoutMs)
				}
			default:
				// Linux uses seconds.
				secs := int(tt.timeout.Seconds())
				if secs < 1 {
					secs = 1
				}
				wantTimeoutSecs := strconv.Itoa(secs)
				if args[3] != wantTimeoutSecs {
					t.Errorf("timeout value = %q, want %q (seconds for linux)", args[3], wantTimeoutSecs)
				}
			}

			// Verify destination is the last argument.
			if args[len(args)-1] != tt.dest {
				t.Errorf("last arg = %q, want %q (destination)", args[len(args)-1], tt.dest)
			}

			// Verify total argument count: -c <count> -W <timeout> <dest>
			if len(args) != 5 {
				t.Errorf("len(args) = %d, want 5", len(args))
			}
		})
	}
}
