package grpcserver

import (
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
	tlsutil "github.com/pobradovic08/route-beacon/internal/shared/tls"
	pb "github.com/pobradovic08/route-beacon/proto/routebeacon/v1"
)

// CommandEvent represents a single event in a command's lifecycle, streamed
// back to the waiting API handler.
type CommandEvent struct {
	Type string // "ping_reply", "ping_summary", "traceroute_hop", "complete", "error" (internal event types)
	Data any
}

// inflightCommand tracks a dispatched command that is awaiting results from a collector.
type inflightCommand struct {
	cmd        *model.DiagnosticCommand
	eventCh    chan CommandEvent
	createdAt  time.Time
}

// CommandDispatcher routes diagnostic commands to the correct collector and
// forwards result events back to the waiting API handler.
type CommandDispatcher struct {
	mu          sync.RWMutex
	collectors  map[string]chan *model.DiagnosticCommand // per-collector command channels
	inflight    map[string]*inflightCommand              // tracked by command ID
	maxPerTarget int                                     // concurrency limit per LG target
}

// NewCommandDispatcher creates a new CommandDispatcher with default settings.
func NewCommandDispatcher() *CommandDispatcher {
	return &CommandDispatcher{
		collectors:   make(map[string]chan *model.DiagnosticCommand),
		inflight:     make(map[string]*inflightCommand),
		maxPerTarget: 2,
	}
}

// RegisterCollector creates a buffered command channel for a collector. If the
// collector is already registered, this is a no-op.
func (d *CommandDispatcher) RegisterCollector(collectorID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.collectors[collectorID]; exists {
		return
	}
	d.collectors[collectorID] = make(chan *model.DiagnosticCommand, 16)
	slog.Info("command dispatcher: registered collector", "collector_id", collectorID)
}

// UnregisterCollector closes and removes the command channel for a collector.
// Any commands still in the channel will be drained and their inflight entries
// cleaned up with an error event.
func (d *CommandDispatcher) UnregisterCollector(collectorID string) {
	d.mu.Lock()
	ch, exists := d.collectors[collectorID]
	if !exists {
		d.mu.Unlock()
		return
	}
	delete(d.collectors, collectorID)
	d.mu.Unlock()

	// Close the channel so any blocking SubscribeCommands reader returns.
	close(ch)

	// Drain remaining commands and notify waiters that the collector went away.
	for cmd := range ch {
		d.mu.Lock()
		ifc, ok := d.inflight[cmd.ID]
		if ok {
			delete(d.inflight, cmd.ID)
		}
		d.mu.Unlock()
		if ok {
			select {
			case ifc.eventCh <- CommandEvent{Type: "error", Data: "collector disconnected"}:
			default:
			}
			close(ifc.eventCh)
		}
	}

	// Sweep inflight commands that were already sent to the collector (read
	// from the channel) but have not yet completed. Without this cleanup,
	// their eventCh channels would never be closed, leaving API handlers
	// blocked forever and leaking per-target concurrency slots.
	d.mu.Lock()
	var staleInflight []*inflightCommand
	for id, ifc := range d.inflight {
		if ifc.cmd.LGTargetID.CollectorID == collectorID {
			staleInflight = append(staleInflight, ifc)
			delete(d.inflight, id)
		}
	}
	d.mu.Unlock()

	for _, ifc := range staleInflight {
		select {
		case ifc.eventCh <- CommandEvent{Type: "error", Data: "collector disconnected"}:
		default:
		}
		close(ifc.eventCh)
	}

	slog.Info("command dispatcher: unregistered collector", "collector_id", collectorID)
}

// GetCommandChannel returns the read side of a collector's command channel.
// Returns nil if the collector is not registered.
func (d *CommandDispatcher) GetCommandChannel(collectorID string) <-chan *model.DiagnosticCommand {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.collectors[collectorID]
}

// Dispatch validates that the target collector exists, checks the concurrency
// limit, enqueues the command, and returns an event channel that the API handler
// can read from until "complete" or "error" arrives.
func (d *CommandDispatcher) Dispatch(cmd *model.DiagnosticCommand) (<-chan CommandEvent, error) {
	collectorID := cmd.LGTargetID.CollectorID

	d.mu.Lock()
	defer d.mu.Unlock()

	ch, exists := d.collectors[collectorID]
	if !exists {
		return nil, fmt.Errorf("collector %q not registered", collectorID)
	}

	// Check concurrency limit: count inflight commands for the same LG target.
	active := 0
	for _, ifc := range d.inflight {
		if ifc.cmd.LGTargetID == cmd.LGTargetID {
			active++
		}
	}
	if active >= d.maxPerTarget {
		return nil, fmt.Errorf("concurrency limit reached for target %s (max %d)", cmd.LGTargetID, d.maxPerTarget)
	}

	eventCh := make(chan CommandEvent, 32)
	d.inflight[cmd.ID] = &inflightCommand{
		cmd:       cmd,
		eventCh:   eventCh,
		createdAt: time.Now(),
	}

	// Non-blocking send; if the channel is full, return an error rather than
	// blocking the API handler.
	select {
	case ch <- cmd:
	default:
		delete(d.inflight, cmd.ID)
		close(eventCh)
		return nil, fmt.Errorf("command channel full for collector %q", collectorID)
	}

	slog.Info("command dispatched",
		"command_id", cmd.ID,
		"type", cmd.Type,
		"collector", collectorID,
		"destination", cmd.Destination,
	)

	return eventCh, nil
}

// ReportResult forwards a result event to the API handler waiting on the
// command's event channel. If the command ID is unknown (e.g. already completed
// or timed out), the event is silently dropped.
func (d *CommandDispatcher) ReportResult(commandID string, event CommandEvent) {
	d.mu.RLock()
	ifc, ok := d.inflight[commandID]
	d.mu.RUnlock()

	if !ok {
		slog.Warn("command dispatcher: result for unknown command", "command_id", commandID)
		return
	}

	// Non-blocking send to avoid stalling the collector stream if the reader
	// is slow. Events may be dropped if the buffer is exhausted, but this is
	// preferable to a deadlock.
	select {
	case ifc.eventCh <- event:
	default:
		slog.Warn("command dispatcher: event channel full, dropping event",
			"command_id", commandID, "event_type", event.Type)
	}
}

// CompleteCommand cleans up the inflight entry and closes the event channel so
// the API handler knows the stream is finished.
func (d *CommandDispatcher) CompleteCommand(commandID string) {
	d.mu.Lock()
	ifc, ok := d.inflight[commandID]
	if ok {
		delete(d.inflight, commandID)
	}
	d.mu.Unlock()

	if ok {
		close(ifc.eventCh)
		slog.Info("command completed", "command_id", commandID)
	}
}

// ---------------------------------------------------------------------------
// gRPC handler implementations
// ---------------------------------------------------------------------------

// subscribeCommands is the real implementation of the SubscribeCommands RPC.
// It reads from the collector's command channel and streams proto
// DiagnosticCommand messages to the collector.
func (s *Server) subscribeCommands(req *pb.CollectorIdentity, stream pb.CollectorService_SubscribeCommandsServer) error {
	collectorID := req.GetCollectorId()

	// Override with mTLS peer identity when available, matching the Register
	// handler's behaviour so that a client cannot subscribe under a spoofed ID.
	if p, ok := peer.FromContext(stream.Context()); ok && p.AuthInfo != nil {
		if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			if cn, err := tlsutil.ExtractCollectorID(tlsInfo.State); err == nil {
				collectorID = cn
			}
		}
	}

	if collectorID == "" {
		return fmt.Errorf("collector_id is required")
	}

	s.dispatcher.RegisterCollector(collectorID)
	defer s.dispatcher.UnregisterCollector(collectorID)

	ch := s.dispatcher.GetCommandChannel(collectorID)
	if ch == nil {
		return fmt.Errorf("failed to get command channel for collector %q", collectorID)
	}

	slog.Info("collector subscribed to commands", "collector_id", collectorID)

	for {
		select {
		case <-stream.Context().Done():
			slog.Info("SubscribeCommands stream closed", "collector_id", collectorID)
			return stream.Context().Err()

		case cmd, ok := <-ch:
			if !ok {
				// Channel closed (collector unregistered).
				return nil
			}

			pbCmd := modelCommandToProto(cmd)
			if err := stream.Send(pbCmd); err != nil {
				slog.Error("SubscribeCommands send error",
					"collector_id", collectorID,
					"command_id", cmd.ID,
					"error", err,
				)
				return err
			}
		}
	}
}

// reportCommandResult is the real implementation of the ReportCommandResult RPC.
// It receives CommandResultChunk messages from a collector and dispatches events
// to the inflight command's result channel.
func (s *Server) reportCommandResult(stream pb.CollectorService_ReportCommandResultServer) error {
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.CommandAck{Accepted: true})
		}
		if err != nil {
			slog.Error("ReportCommandResult recv error", "error", err)
			return err
		}

		commandID := chunk.GetCommandId()

		switch r := chunk.Result.(type) {
		case *pb.CommandResultChunk_PingResult:
			s.dispatcher.ReportResult(commandID, CommandEvent{
				Type: "ping_reply",
				Data: r.PingResult,
			})

		case *pb.CommandResultChunk_TracerouteHop:
			s.dispatcher.ReportResult(commandID, CommandEvent{
				Type: "traceroute_hop",
				Data: r.TracerouteHop,
			})

		case *pb.CommandResultChunk_Complete:
			s.dispatcher.ReportResult(commandID, CommandEvent{
				Type: "complete",
				Data: r.Complete,
			})
			s.dispatcher.CompleteCommand(commandID)

		case *pb.CommandResultChunk_Error:
			s.dispatcher.ReportResult(commandID, CommandEvent{
				Type: "error",
				Data: r.Error,
			})
			s.dispatcher.CompleteCommand(commandID)
		}
	}
}

// modelCommandToProto converts a model.DiagnosticCommand into a proto
// DiagnosticCommand suitable for sending over the gRPC stream.
func modelCommandToProto(cmd *model.DiagnosticCommand) *pb.DiagnosticCommand {
	pbCmd := &pb.DiagnosticCommand{
		CommandId:       cmd.ID,
		RouterSessionId: cmd.LGTargetID.SessionID,
	}

	dest := cmd.Destination.String()

	switch cmd.Type {
	case model.CommandTypePing:
		pbCmd.Command = &pb.DiagnosticCommand_Ping{
			Ping: &pb.PingCommand{
				Destination: dest,
				Count:       uint32(cmd.Params.Count),
				TimeoutMs:   uint32(cmd.Params.Timeout.Milliseconds()),
			},
		}
	case model.CommandTypeTraceroute:
		pbCmd.Command = &pb.DiagnosticCommand_Traceroute{
			Traceroute: &pb.TracerouteCommand{
				Destination: dest,
				MaxHops:     uint32(cmd.Params.MaxHops),
				TimeoutMs:   uint32(cmd.Params.Timeout.Milliseconds()),
			},
		}
	}

	return pbCmd
}
