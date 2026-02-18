package grpcclient

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/pobradovic08/route-beacon/internal/collector/commands"
	"github.com/pobradovic08/route-beacon/internal/shared/model"
	pb "github.com/pobradovic08/route-beacon/proto/routebeacon/v1"
)

// ackTimeout is the maximum time to wait for a server ACK after sending
// the final result chunk. Prevents holding semaphore slots indefinitely
// if the server never responds.
const ackTimeout = 10 * time.Second

// SubscribeCommands opens a server-side stream to receive diagnostic commands
// from central and executes each command in its own goroutine, bounded by the
// configured per-collector concurrency limit.
func (c *Client) SubscribeCommands(ctx context.Context) error {
	stream, err := c.client.SubscribeCommands(ctx, &pb.CollectorIdentity{
		CollectorId: c.cfg.Collector.ID,
	})
	if err != nil {
		return fmt.Errorf("subscribe commands: %w", err)
	}

	// Semaphore to enforce per-collector concurrency limit.
	limit := c.cfg.Commands.Concurrency.PerCollector
	if limit <= 0 {
		limit = 1
	}
	sem := make(chan struct{}, limit)

	slog.Info("subscribed to commands", "collector_id", c.cfg.Collector.ID, "concurrency", limit)

	for {
		cmd, err := stream.Recv()
		if err != nil {
			// Context cancellation is a normal shutdown path.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("receive command: %w", err)
		}

		slog.Info("received command",
			"command_id", cmd.GetCommandId(),
			"router_session_id", cmd.GetRouterSessionId(),
		)

		// Acquire a semaphore slot before launching the goroutine.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}

		go func(cmd *pb.DiagnosticCommand) {
			defer func() { <-sem }()
			c.executeCommand(ctx, cmd)
		}(cmd)
	}
}

// executeCommand dispatches a single diagnostic command based on its oneof type.
// It creates a per-command cancellable context so that if result streaming fails,
// the subprocess producing events is terminated and its goroutine is not leaked.
func (c *Client) executeCommand(ctx context.Context, cmd *pb.DiagnosticCommand) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	commandID := cmd.GetCommandId()

	switch cmdType := cmd.Command.(type) {
	case *pb.DiagnosticCommand_Ping:
		ping := cmdType.Ping

		dest, err := netip.ParseAddr(ping.GetDestination())
		if err != nil {
			slog.Error("invalid ping destination",
				"command_id", commandID,
				"destination", ping.GetDestination(),
				"error", err,
			)
			c.reportError(ctx, commandID, "INVALID_DESTINATION", fmt.Sprintf("invalid destination address: %v", err))
			return
		}

		count := ping.GetCount()
		if count > 255 {
			c.reportError(ctx, commandID, "INVALID_PARAMS", fmt.Sprintf("count %d exceeds maximum (255)", count))
			return
		}

		params := model.CommandParams{
			Count:   uint8(count),
			Timeout: time.Duration(ping.GetTimeoutMs()) * time.Millisecond,
		}

		events, err := commands.RunPing(ctx, dest, params, c.cfg.Commands.Ping)
		if err != nil {
			slog.Error("failed to start ping",
				"command_id", commandID,
				"destination", dest,
				"error", err,
			)
			c.reportError(ctx, commandID, "EXECUTION_ERROR", fmt.Sprintf("failed to start ping: %v", err))
			return
		}

		if err := c.reportCommandResult(ctx, cancel, commandID, dest.String(), events); err != nil {
			slog.Error("failed to report command result",
				"command_id", commandID,
				"error", err,
			)
		}

	case *pb.DiagnosticCommand_Traceroute:
		traceroute := cmdType.Traceroute

		dest, err := netip.ParseAddr(traceroute.GetDestination())
		if err != nil {
			slog.Error("invalid traceroute destination",
				"command_id", commandID,
				"destination", traceroute.GetDestination(),
				"error", err,
			)
			c.reportError(ctx, commandID, "INVALID_DESTINATION", fmt.Sprintf("invalid destination address: %v", err))
			return
		}

		maxHops := traceroute.GetMaxHops()
		if maxHops > 255 {
			c.reportError(ctx, commandID, "INVALID_PARAMS", fmt.Sprintf("max_hops %d exceeds maximum (255)", maxHops))
			return
		}

		params := model.CommandParams{
			MaxHops: uint8(maxHops),
			Timeout: time.Duration(traceroute.GetTimeoutMs()) * time.Millisecond,
		}

		events, err := commands.RunTraceroute(ctx, dest, params, c.cfg.Commands.Traceroute)
		if err != nil {
			slog.Error("failed to start traceroute",
				"command_id", commandID,
				"destination", dest,
				"error", err,
			)
			c.reportError(ctx, commandID, "EXECUTION_ERROR", fmt.Sprintf("failed to start traceroute: %v", err))
			return
		}

		if err := c.reportTracerouteResult(ctx, cancel, commandID, dest.String(), events); err != nil {
			slog.Error("failed to report traceroute result",
				"command_id", commandID,
				"error", err,
			)
		}

	default:
		slog.Error("unknown command type",
			"command_id", commandID,
		)
		c.reportError(ctx, commandID, "UNKNOWN_COMMAND", "unknown command type")
	}
}

// reportCommandResult opens a client-side stream and sends ping results back
// to central as they arrive from the events channel. The cancel function is
// used to terminate the subprocess on send failure so the producer goroutine
// does not leak.
func (c *Client) reportCommandResult(ctx context.Context, cancel context.CancelFunc, commandID string, destination string, events <-chan commands.PingEvent) error {
	stream, err := c.client.ReportCommandResult(ctx)
	if err != nil {
		return fmt.Errorf("open result stream: %w", err)
	}

	var replies []model.PingReply
	var summary *model.PingSummary

	for event := range events {
		switch event.Type {
		case commands.PingEventReply:
			if event.Reply == nil {
				continue
			}
			reply := event.Reply
			replies = append(replies, *reply)

			err := stream.Send(&pb.CommandResultChunk{
				CommandId: commandID,
				Result: &pb.CommandResultChunk_PingResult{
					PingResult: &pb.PingResult{
						Sequence: uint32(reply.Seq),
						RttUs:    uint32(reply.RTT.Microseconds()),
						Ttl:      uint32(reply.TTL),
						Success:  reply.Success,
					},
				},
			})
			if err != nil {
				cancel()
				//nolint:revive // drain channel to unblock producer goroutine
				for range events {
				}
				return fmt.Errorf("send ping result: %w", err)
			}

		case commands.PingEventSummary:
			summary = event.Summary
		}
	}

	// Send the completion message with a plain-text summary.
	plainText := commands.FormatPingPlainText(destination, replies, summary)
	err = stream.Send(&pb.CommandResultChunk{
		CommandId: commandID,
		Result: &pb.CommandResultChunk_Complete{
			Complete: &pb.CommandComplete{
				Summary: plainText,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("send command complete: %w", err)
	}

	// Close the stream and receive the server's acknowledgment with a
	// deadline to avoid holding a semaphore slot indefinitely.
	ackDone := make(chan error, 1)
	go func() {
		_, err := stream.CloseAndRecv()
		ackDone <- err
	}()
	select {
	case err := <-ackDone:
		if err != nil {
			return fmt.Errorf("close result stream: %w", err)
		}
	case <-time.After(ackTimeout):
		cancel()
		<-ackDone
		return fmt.Errorf("timed out waiting for server ACK")
	}

	slog.Info("command completed",
		"command_id", commandID,
		"destination", destination,
		"replies", len(replies),
	)

	return nil
}

// reportTracerouteResult opens a client-side stream and sends traceroute hop
// results back to central as they arrive from the events channel. The cancel
// function is used to terminate the subprocess on send failure so the producer
// goroutine does not leak.
func (c *Client) reportTracerouteResult(ctx context.Context, cancel context.CancelFunc, commandID string, destination string, events <-chan commands.TracerouteEvent) error {
	stream, err := c.client.ReportCommandResult(ctx)
	if err != nil {
		return fmt.Errorf("open result stream: %w", err)
	}

	var hops []model.TracerouteHop

	for event := range events {
		switch event.Type {
		case commands.TracerouteEventHop:
			if event.Hop == nil {
				continue
			}
			hop := event.Hop
			hops = append(hops, *hop)

			// Determine the address from the first successful probe.
			probeAddress := ""
			for _, p := range hop.Probes {
				if p.Success {
					probeAddress = p.Address
					break
				}
			}

			// Convert each probe's RTT to microseconds; use 0 for timed-out probes.
			rttUs := make([]uint32, len(hop.Probes))
			for i, p := range hop.Probes {
				if p.Success {
					rttUs[i] = uint32(p.RTT.Microseconds())
				} else {
					rttUs[i] = 0
				}
			}

			err := stream.Send(&pb.CommandResultChunk{
				CommandId: commandID,
				Result: &pb.CommandResultChunk_TracerouteHop{
					TracerouteHop: &pb.TracerouteHop{
						HopNumber: uint32(hop.TTL),
						Address:   probeAddress,
						RttUs:     rttUs,
					},
				},
			})
			if err != nil {
				cancel()
				//nolint:revive // drain channel to unblock producer goroutine
				for range events {
				}
				return fmt.Errorf("send traceroute hop: %w", err)
			}

		case commands.TracerouteEventSummary:
			// Summary event is consumed but not sent as a separate chunk;
			// the plain-text summary is included in the CommandComplete below.
		}
	}

	// Send the completion message with a plain-text summary.
	plainText := commands.FormatTraceroutePlainText(destination, hops)
	err = stream.Send(&pb.CommandResultChunk{
		CommandId: commandID,
		Result: &pb.CommandResultChunk_Complete{
			Complete: &pb.CommandComplete{
				Summary: plainText,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("send command complete: %w", err)
	}

	// Close the stream and receive the server's acknowledgment with a
	// deadline to avoid holding a semaphore slot indefinitely.
	ackDone := make(chan error, 1)
	go func() {
		_, err := stream.CloseAndRecv()
		ackDone <- err
	}()
	select {
	case err := <-ackDone:
		if err != nil {
			return fmt.Errorf("close result stream: %w", err)
		}
	case <-time.After(ackTimeout):
		cancel()
		<-ackDone
		return fmt.Errorf("timed out waiting for server ACK")
	}

	slog.Info("traceroute command completed",
		"command_id", commandID,
		"destination", destination,
		"hops", len(hops),
	)

	return nil
}

// reportError sends a single error chunk for a command that failed before
// producing any results, then closes the stream.
func (c *Client) reportError(ctx context.Context, commandID string, code string, message string) {
	ctx, cancel := context.WithTimeout(ctx, ackTimeout)
	defer cancel()

	stream, err := c.client.ReportCommandResult(ctx)
	if err != nil {
		slog.Error("failed to open error result stream",
			"command_id", commandID,
			"error", err,
		)
		return
	}

	err = stream.Send(&pb.CommandResultChunk{
		CommandId: commandID,
		Result: &pb.CommandResultChunk_Error{
			Error: &pb.CommandError{
				Code:    code,
				Message: message,
			},
		},
	})
	if err != nil {
		slog.Error("failed to send error result",
			"command_id", commandID,
			"error", err,
		)
		return
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		slog.Error("failed to close error result stream",
			"command_id", commandID,
			"error", err,
		)
	}
}
