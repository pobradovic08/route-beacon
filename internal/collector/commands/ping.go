package commands

import (
	"bufio"
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pobradovic08/route-beacon/internal/collector/config"
	"github.com/pobradovic08/route-beacon/internal/shared/model"
	"github.com/pobradovic08/route-beacon/internal/shared/validate"
)

// PingEventType distinguishes reply events from the final summary event.
type PingEventType int

const (
	PingEventReply   PingEventType = iota
	PingEventSummary
)

// PingEvent represents a single parsed event from ping output. It is either
// a per-packet reply or the final summary line. Exactly one of Reply or
// Summary is non-nil.
type PingEvent struct {
	Type    PingEventType
	Reply   *model.PingReply
	Summary *model.PingSummary
}

// Regex patterns for parsing ping output.
var (
	// Matches reply lines such as:
	//   64 bytes from 8.8.8.8: icmp_seq=1 ttl=118 time=5.26 ms
	replyRe = regexp.MustCompile(
		`(\d+) bytes from .+: icmp_seq=(\d+) ttl=(\d+) time=([\d.]+) ms`,
	)

	// Matches the packet statistics line:
	//   5 packets transmitted, 5 received, 0% packet loss, time 4005ms
	//   5 packets transmitted, 5 packets received, 0.0% packet loss
	statsRe = regexp.MustCompile(
		`(\d+) packets? transmitted, (\d+)(?: packets?)? received, ([\d.]+)% packet loss`,
	)

	// Matches the round-trip summary line:
	//   rtt min/avg/max/mdev = 4.123/5.456/6.789/0.901 ms
	//   round-trip min/avg/max/stddev = 4.123/5.456/6.789/0.901 ms
	rttRe = regexp.MustCompile(
		`(?:rtt|round-trip) min/avg/max/(?:mdev|stddev) = ([\d.]+)/([\d.]+)/([\d.]+)/([\d.]+) ms`,
	)
)

// RunPing executes a ping command against destination, streaming parsed results
// to the returned channel. The channel is closed when the command finishes.
//
// Parameters are validated and clamped to the limits defined in cfg. The
// destination address is validated via validate.ValidateDestination to reject
// private/reserved ranges.
func RunPing(ctx context.Context, destination netip.Addr, params model.CommandParams, cfg config.PingConfig) (<-chan PingEvent, error) {
	// Validate destination address.
	if err := validate.ValidateDestination(destination); err != nil {
		return nil, fmt.Errorf("invalid destination: %w", err)
	}

	// Apply defaults and enforce limits.
	count := params.Count
	if count == 0 {
		count = cfg.DefaultCount
	}
	if count == 0 {
		count = 5
	}
	if cfg.MaxCount > 0 && count > cfg.MaxCount {
		count = cfg.MaxCount
	}

	timeout := params.Timeout
	if timeout == 0 {
		timeout = time.Duration(cfg.DefaultTimeoutMs) * time.Millisecond
	}
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	maxTimeout := time.Duration(cfg.MaxTimeoutMs) * time.Millisecond
	if maxTimeout > 0 && timeout > maxTimeout {
		timeout = maxTimeout
	}

	// Build command arguments.
	binary := cfg.Binary
	if binary == "" {
		binary = "/usr/bin/ping"
	}

	args := buildPingArgs(destination.String(), count, timeout)

	// Create a context with the overall timeout. Ping sends one packet per
	// second and waits up to the probe timeout for each, so we budget
	// count * timeout plus a 5-second grace period for the process to
	// print its summary before we kill it.
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(count)*timeout+5*time.Second)

	cmd := exec.CommandContext(cmdCtx, binary, args...)

	// Run in its own process group so we can kill the entire group on cancel.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start ping: %w", err)
	}

	ch := make(chan PingEvent, int(count)+1)

	go func() {
		defer close(ch)
		defer cancel()
		defer func() {
			// Kill the entire process group if the process is still running.
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			_ = cmd.Wait()
		}()

		scanner := bufio.NewScanner(stdout)
		var summary model.PingSummary
		haveSummary := false

		for scanner.Scan() {
			line := scanner.Text()

			// Check for reply line.
			if reply, ok := parseReplyLine(line); ok {
				select {
				case ch <- PingEvent{Type: PingEventReply, Reply: &reply}:
				case <-cmdCtx.Done():
					return
				}
				continue
			}

			// Check for statistics line.
			if sent, received, loss, ok := parseStatsLine(line); ok {
				summary.PacketsSent = sent
				summary.PacketsReceived = received
				summary.PacketLoss = loss
				haveSummary = true
				continue
			}

			// Check for RTT summary line.
			if min, avg, max, stddev, ok := parseRTTLine(line); ok {
				summary.RTTMin = min
				summary.RTTAvg = avg
				summary.RTTMax = max
				summary.RTTStdDev = stddev
				haveSummary = true
				continue
			}
		}

		if haveSummary {
			select {
			case ch <- PingEvent{Type: PingEventSummary, Summary: &summary}:
			case <-cmdCtx.Done():
			}
		}
	}()

	return ch, nil
}

// buildPingArgs constructs the platform-specific ping arguments.
func buildPingArgs(dest string, count uint8, timeout time.Duration) []string {
	args := []string{"-c", strconv.Itoa(int(count))}

	switch runtime.GOOS {
	case "darwin":
		// macOS -W expects timeout in milliseconds.
		args = append(args, "-W", strconv.FormatInt(timeout.Milliseconds(), 10))
	default:
		// Linux -W expects timeout in seconds (integer).
		secs := int(timeout.Seconds())
		if secs < 1 {
			secs = 1
		}
		args = append(args, "-W", strconv.Itoa(secs))
	}

	args = append(args, dest)
	return args
}

// parseReplyLine attempts to parse a ping reply line.
func parseReplyLine(line string) (model.PingReply, bool) {
	m := replyRe.FindStringSubmatch(line)
	if m == nil {
		return model.PingReply{}, false
	}

	seq, err := strconv.ParseUint(m[2], 10, 16)
	if err != nil {
		return model.PingReply{}, false
	}

	ttl, err := strconv.ParseUint(m[3], 10, 8)
	if err != nil {
		return model.PingReply{}, false
	}

	rttMs, err := strconv.ParseFloat(m[4], 64)
	if err != nil {
		return model.PingReply{}, false
	}

	return model.PingReply{
		Seq:     uint16(seq),
		TTL:     uint8(ttl),
		RTT:     time.Duration(rttMs * float64(time.Millisecond)),
		Success: true,
	}, true
}

// parseStatsLine attempts to parse the packet statistics line.
func parseStatsLine(line string) (sent, received uint16, loss float64, ok bool) {
	m := statsRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, 0, false
	}

	s, err := strconv.ParseUint(m[1], 10, 16)
	if err != nil {
		return 0, 0, 0, false
	}

	r, err := strconv.ParseUint(m[2], 10, 16)
	if err != nil {
		return 0, 0, 0, false
	}

	l, err := strconv.ParseFloat(m[3], 64)
	if err != nil {
		return 0, 0, 0, false
	}

	return uint16(s), uint16(r), l, true
}

// parseRTTLine attempts to parse the RTT summary line.
func parseRTTLine(line string) (min, avg, max, stddev time.Duration, ok bool) {
	m := rttRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, 0, 0, false
	}

	minMs, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, 0, 0, 0, false
	}
	avgMs, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		return 0, 0, 0, 0, false
	}
	maxMs, err := strconv.ParseFloat(m[3], 64)
	if err != nil {
		return 0, 0, 0, 0, false
	}
	stddevMs, err := strconv.ParseFloat(m[4], 64)
	if err != nil {
		return 0, 0, 0, 0, false
	}

	return time.Duration(minMs * float64(time.Millisecond)),
		time.Duration(avgMs * float64(time.Millisecond)),
		time.Duration(maxMs * float64(time.Millisecond)),
		time.Duration(stddevMs * float64(time.Millisecond)),
		true
}

// FormatPingPlainText produces a human-readable plain text summary from ping
// results, suitable for the PlainText field of model.CommandResult.
func FormatPingPlainText(dest string, replies []model.PingReply, summary *model.PingSummary) string {
	var b strings.Builder

	for _, r := range replies {
		if r.Success {
			fmt.Fprintf(&b, "Reply from %s: seq=%d ttl=%d time=%.3f ms\n",
				dest, r.Seq, r.TTL, float64(r.RTT)/float64(time.Millisecond))
		} else {
			fmt.Fprintf(&b, "Request timeout for seq=%d\n", r.Seq)
		}
	}

	if summary != nil {
		fmt.Fprintf(&b, "\n--- %s ping statistics ---\n", dest)
		fmt.Fprintf(&b, "%d packets transmitted, %d received, %.1f%% packet loss\n",
			summary.PacketsSent, summary.PacketsReceived, summary.PacketLoss)

		if summary.PacketsReceived > 0 {
			fmt.Fprintf(&b, "rtt min/avg/max/stddev = %.3f/%.3f/%.3f/%.3f ms\n",
				float64(summary.RTTMin)/float64(time.Millisecond),
				float64(summary.RTTAvg)/float64(time.Millisecond),
				float64(summary.RTTMax)/float64(time.Millisecond),
				float64(summary.RTTStdDev)/float64(time.Millisecond))
		}
	}

	return b.String()
}
