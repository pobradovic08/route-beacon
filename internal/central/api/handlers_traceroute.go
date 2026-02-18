package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
	"github.com/pobradovic08/route-beacon/internal/shared/validate"
	pb "github.com/pobradovic08/route-beacon/proto/routebeacon/v1"
)

// Default traceroute parameters used when optional fields are omitted.
const (
	defaultTracerouteMaxHops    = 30
	defaultTracerouteTimeoutMs  = 5000
	maxTracerouteMaxHops        = 64
	maxTracerouteTimeoutMs      = 10000
)

// SSE response type for traceroute hop events.
type tracerouteHopEvent struct {
	HopNumber uint32    `json:"hop_number"`
	Address   string    `json:"address"`
	RTTMs     []float64 `json:"rtt_ms"`
}

// SSE response type for traceroute complete events.
type tracerouteCompleteEvent struct {
	ReachedDestination bool `json:"reached_destination"`
}

// Traceroute handles POST /api/v1/targets/{targetId}/traceroute with SSE streaming.
func (h *APIHandler) Traceroute(w http.ResponseWriter, r *http.Request, targetId TargetId) {
	// 1. Look up the target.
	sess := h.Registry.GetTargetByString(targetId)
	if sess == nil {
		WriteProblem(w, http.StatusNotFound, "Not Found",
			fmt.Sprintf("LG target %q does not exist.", targetId))
		return
	}

	// 2. Parse the JSON request body.
	var req TracerouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteProblem(w, http.StatusBadRequest, "Bad Request",
			fmt.Sprintf("Invalid JSON body: %s", err))
		return
	}

	// 3. Parse and validate the destination address.
	addr, err := netip.ParseAddr(req.Destination)
	if err != nil {
		WriteProblem(w, http.StatusBadRequest, "Bad Request",
			fmt.Sprintf("Invalid destination address %q: %s", req.Destination, err))
		return
	}
	if err := validate.ValidateDestination(addr); err != nil {
		WriteProblem(w, http.StatusForbidden, "Forbidden",
			fmt.Sprintf("Destination address not allowed: %s", err))
		return
	}

	// Apply defaults for optional parameters.
	maxHops := defaultTracerouteMaxHops
	if req.MaxHops != nil {
		maxHops = *req.MaxHops
	}
	timeoutMs := defaultTracerouteTimeoutMs
	if req.TimeoutMs != nil {
		timeoutMs = *req.TimeoutMs
	}

	// Validate parameter ranges.
	if err := validate.ValidateTracerouteParams(uint8(maxHops), uint32(timeoutMs), maxTracerouteMaxHops, maxTracerouteTimeoutMs); err != nil {
		WriteProblem(w, http.StatusBadRequest, "Bad Request",
			fmt.Sprintf("Invalid traceroute parameters: %s", err))
		return
	}

	// 4. Build the diagnostic command.
	commandID, err := generateCommandID()
	if err != nil {
		WriteProblem(w, http.StatusInternalServerError, "Internal Server Error",
			"Failed to generate command identifier.")
		return
	}
	cmd := &model.DiagnosticCommand{
		ID:          commandID,
		Type:        model.CommandTypeTraceroute,
		LGTargetID:  sess.LGTargetID(),
		Destination: addr,
		Params: model.CommandParams{
			MaxHops: uint8(maxHops),
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
		},
		Status:    model.CommandStatusPending,
		CreatedAt: time.Now(),
	}

	// 5. Audit log the command before dispatch (FR-028).
	AuditLog(cmd, r)

	// 6. Dispatch the command.
	eventCh, err := h.Dispatcher.Dispatch(cmd)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "concurrency limit") {
			WriteProblem(w, http.StatusTooManyRequests, "Too Many Requests",
				"Concurrency limit reached for this target. Please try again shortly.")
			return
		}
		if strings.Contains(errMsg, "not registered") {
			WriteProblem(w, http.StatusBadGateway, "Bad Gateway",
				"The collector for this target is not currently connected.")
			return
		}
		WriteProblem(w, http.StatusBadGateway, "Bad Gateway",
			fmt.Sprintf("Failed to dispatch command: %s", err))
		return
	}

	// 7. Initialize SSE writer.
	sse, err := NewSSEWriter(w)
	if err != nil {
		slog.Error("failed to create SSE writer", "error", err)
		WriteProblem(w, http.StatusInternalServerError, "Internal Server Error",
			"Streaming not supported.")
		return
	}

	slog.Info("traceroute command started",
		"command_id", cmd.ID,
		"target", targetId,
		"destination", addr.String(),
		"max_hops", maxHops,
		"timeout_ms", timeoutMs,
	)

	startTime := time.Now()
	lastHopAddress := ""

	// 8. Stream events from the dispatcher's event channel.
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected.
			slog.Info("client disconnected during traceroute stream",
				"command_id", cmd.ID,
				"target", targetId,
			)
			return

		case event, ok := <-eventCh:
			if !ok {
				// Channel closed — stream finished.
				return
			}

			switch event.Type {
			case "traceroute_hop":
				if hop, ok := event.Data.(*pb.TracerouteHop); ok {
					// Convert RttUs (microseconds) to milliseconds.
					rttMs := make([]float64, len(hop.GetRttUs()))
					for i, us := range hop.GetRttUs() {
						rttMs[i] = float64(us) / 1000.0
					}
					// Track address of the last hop to determine if the destination was reached.
					lastHopAddress = hop.GetAddress()
					if err := sse.WriteEvent("hop", tracerouteHopEvent{
						HopNumber: hop.GetHopNumber(),
						Address:   hop.GetAddress(),
						RTTMs:     rttMs,
					}); err != nil {
						slog.Warn("failed to write hop SSE event",
							"command_id", cmd.ID, "error", err)
						return
					}
				}

			case "complete":
				reached := lastHopAddress == addr.String()
				if err := sse.WriteEvent("complete", tracerouteCompleteEvent{
					ReachedDestination: reached,
				}); err != nil {
					slog.Warn("failed to write complete SSE event",
						"command_id", cmd.ID, "error", err)
				}
				slog.Info("traceroute command completed",
					"command_id", cmd.ID,
					"target", targetId,
				)
				AuditLogComplete(cmd.ID, cmd.Type, time.Since(startTime), true)
				return

			case "error":
				var evt errorEvent
				switch e := event.Data.(type) {
				case *pb.CommandError:
					evt = errorEvent{
						Code:    e.GetCode(),
						Message: e.GetMessage(),
					}
				case string:
					evt = errorEvent{
						Code:    "ERROR",
						Message: e,
					}
				default:
					evt = errorEvent{
						Code:    "UNKNOWN",
						Message: fmt.Sprintf("%v", event.Data),
					}
				}
				if err := sse.WriteEvent("error", evt); err != nil {
					slog.Warn("failed to write error SSE event",
						"command_id", cmd.ID, "error", err)
				}
				slog.Warn("traceroute command error",
					"command_id", cmd.ID,
					"target", targetId,
					"code", evt.Code,
					"message", evt.Message,
				)
				AuditLogComplete(cmd.ID, cmd.Type, time.Since(startTime), false)
				return
			}
		}
	}
}
