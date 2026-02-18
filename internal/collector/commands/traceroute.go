package commands

import (
	"bufio"
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pobradovic08/route-beacon/internal/collector/config"
	"github.com/pobradovic08/route-beacon/internal/shared/model"
	"github.com/pobradovic08/route-beacon/internal/shared/validate"
)

// TracerouteEventType distinguishes hop events from the final summary event.
type TracerouteEventType int

const (
	TracerouteEventHop     TracerouteEventType = iota
	TracerouteEventSummary
)

// TracerouteEvent represents a single parsed event from traceroute output. It
// is either a per-hop result or the final summary. Exactly one of Hop or
// Summary is non-nil.
type TracerouteEvent struct {
	Type    TracerouteEventType
	Hop     *model.TracerouteHop
	Summary *TracerouteSummary
}

// TracerouteSummary holds the final state of a traceroute execution.
type TracerouteSummary struct {
	TotalHops int
	Completed bool
}

// Regex patterns for parsing traceroute output.
var (
	// hopLineRe matches a hop line starting with the hop number.
	// Example: " 1  192.168.1.1  0.529 ms  0.402 ms  0.378 ms"
	// Example: " 3  * * *"
	hopLineRe = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)

	// probeRe matches individual probes within a hop line: either an RTT
	// value in ms or a timeout asterisk.
	probeRe = regexp.MustCompile(`([\d.]+)\s+ms|\*`)

	// ipRe matches an IP address (v4 or v6) at the beginning of the probe
	// section or before an RTT value, used to detect address changes within
	// a hop line.
	ipRe = regexp.MustCompile(`([\d.]+|[0-9a-fA-F:]+)\s`)
)

// RunTraceroute executes a traceroute command against destination, streaming
// parsed results to the returned channel. The channel is closed when the
// command finishes.
//
// Parameters are validated and clamped to the limits defined in cfg. The
// destination address is validated via validate.ValidateDestination to reject
// private/reserved ranges.
func RunTraceroute(ctx context.Context, destination netip.Addr, params model.CommandParams, cfg config.TracerouteConfig) (<-chan TracerouteEvent, error) {
	// Validate destination address.
	if err := validate.ValidateDestination(destination); err != nil {
		return nil, fmt.Errorf("invalid destination: %w", err)
	}

	// Apply defaults and enforce limits.
	maxHops := params.MaxHops
	if maxHops == 0 {
		maxHops = cfg.DefaultMaxHops
	}
	if maxHops == 0 {
		maxHops = 30
	}
	if cfg.MaxHops > 0 && maxHops > cfg.MaxHops {
		maxHops = cfg.MaxHops
	}
	// Enforce absolute maximum of 64 hops.
	if maxHops > 64 {
		maxHops = 64
	}

	timeout := params.Timeout
	if timeout == 0 {
		timeout = time.Duration(cfg.DefaultTimeoutMs) * time.Millisecond
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	maxTimeout := time.Duration(cfg.MaxTimeoutMs) * time.Millisecond
	if maxTimeout > 0 && timeout > maxTimeout {
		timeout = maxTimeout
	}

	// Build command arguments.
	binary := cfg.Binary
	if binary == "" {
		binary = "/usr/bin/traceroute"
	}

	args := buildTracerouteArgs(destination.String(), maxHops, timeout)

	// Create a context with the overall timeout. In the worst case each hop
	// can take up to one full per-probe timeout (probes within a hop run
	// concurrently), so we budget maxHops * timeout plus a 30-second grace
	// period to allow the process to finish naturally.
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(maxHops)*timeout+30*time.Second)

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
		return nil, fmt.Errorf("start traceroute: %w", err)
	}

	ch := make(chan TracerouteEvent, int(maxHops)+1)

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
		totalHops := 0
		completed := false

		for scanner.Scan() {
			line := scanner.Text()

			hop, ok := parseHopLine(line)
			if !ok {
				continue
			}

			totalHops++
			select {
			case ch <- TracerouteEvent{Type: TracerouteEventHop, Hop: &hop}:
			case <-cmdCtx.Done():
				return
			}

			// If any probe in this hop reached the destination, the
			// traceroute is complete.
			for _, p := range hop.Probes {
				if p.Success && p.Address == destination.String() {
					completed = true
				}
			}
		}

		// Send summary event.
		select {
		case ch <- TracerouteEvent{
			Type: TracerouteEventSummary,
			Summary: &TracerouteSummary{
				TotalHops: totalHops,
				Completed: completed,
			},
		}:
		case <-cmdCtx.Done():
		}
	}()

	return ch, nil
}

// buildTracerouteArgs constructs the traceroute arguments.
func buildTracerouteArgs(dest string, maxHops uint8, timeout time.Duration) []string {
	// -n: no DNS resolution
	// -m: max number of hops
	// -w: per-probe timeout in seconds (float)
	args := []string{
		"-n",
		"-m", strconv.Itoa(int(maxHops)),
		"-w", strconv.FormatFloat(timeout.Seconds(), 'f', 1, 64),
		dest,
	}
	return args
}

// parseHopLine attempts to parse a single traceroute hop line. It returns
// a TracerouteHop and true if the line was successfully parsed.
func parseHopLine(line string) (model.TracerouteHop, bool) {
	m := hopLineRe.FindStringSubmatch(line)
	if m == nil {
		return model.TracerouteHop{}, false
	}

	hopNum, err := strconv.ParseUint(m[1], 10, 8)
	if err != nil {
		return model.TracerouteHop{}, false
	}

	probes := parseProbes(m[2])

	return model.TracerouteHop{
		TTL:    uint8(hopNum),
		Probes: probes,
	}, true
}

// parseProbes extracts individual probe results from the remainder of a hop
// line. It handles lines like:
//
//	"192.168.1.1  0.529 ms  0.402 ms  0.378 ms"
//	"* * *"
//	"172.16.0.1  10.123 ms  10.456 ms  *"
//	"10.0.0.1  4.146 ms 10.0.0.2  5.789 ms  10.0.0.1  4.090 ms"
func parseProbes(rest string) []model.TracerouteProbe {
	var probes []model.TracerouteProbe
	currentAddr := ""

	// Tokenize the rest of the line by whitespace.
	tokens := strings.Fields(rest)

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		if token == "*" {
			// Timeout probe.
			probes = append(probes, model.TracerouteProbe{
				Success: false,
			})
			continue
		}

		if token == "ms" {
			// Skip "ms" tokens; they are consumed by the RTT parsing below.
			continue
		}

		// Check if this token looks like an IP address (contains dots for
		// IPv4 or colons for IPv6).
		if looksLikeIP(token) {
			currentAddr = token
			continue
		}

		// Try to parse as an RTT value (float).
		rttMs, err := strconv.ParseFloat(token, 64)
		if err != nil {
			continue
		}

		// Consume the trailing "ms" if present.
		if i+1 < len(tokens) && tokens[i+1] == "ms" {
			i++
		}

		probes = append(probes, model.TracerouteProbe{
			Address: currentAddr,
			RTT:     time.Duration(rttMs * float64(time.Millisecond)),
			Success: true,
		})
	}

	return probes
}

// looksLikeIP returns true if the string looks like an IPv4 or IPv6 address.
func looksLikeIP(s string) bool {
	// Quick heuristic: if it parses as a valid IP, it is one.
	_, err := netip.ParseAddr(s)
	return err == nil
}

// FormatTraceroutePlainText produces a human-readable plain text representation
// of traceroute results, suitable for the PlainText field of
// model.CommandResult.
func FormatTraceroutePlainText(dest string, hops []model.TracerouteHop) string {
	var b strings.Builder

	maxHops := 30
	if len(hops) > 0 {
		lastHop := hops[len(hops)-1]
		if int(lastHop.TTL) > maxHops {
			maxHops = int(lastHop.TTL)
		}
	}

	fmt.Fprintf(&b, "traceroute to %s, %d hops max\n", dest, maxHops)

	for _, hop := range hops {
		fmt.Fprintf(&b, "%2d  ", hop.TTL)

		allTimeout := true
		for _, p := range hop.Probes {
			if p.Success {
				allTimeout = false
				break
			}
		}

		if allTimeout {
			stars := make([]string, len(hop.Probes))
			for i := range stars {
				stars[i] = "*"
			}
			b.WriteString(strings.Join(stars, " "))
		} else {
			prevAddr := ""
			parts := make([]string, 0, len(hop.Probes))
			for _, p := range hop.Probes {
				if !p.Success {
					parts = append(parts, "*")
					continue
				}
				if p.Address != prevAddr {
					parts = append(parts, fmt.Sprintf("%s  %.3f ms",
						p.Address, float64(p.RTT)/float64(time.Millisecond)))
					prevAddr = p.Address
				} else {
					parts = append(parts, fmt.Sprintf("%.3f ms",
						float64(p.RTT)/float64(time.Millisecond)))
				}
			}
			b.WriteString(strings.Join(parts, "  "))
		}

		b.WriteByte('\n')
	}

	return b.String()
}
