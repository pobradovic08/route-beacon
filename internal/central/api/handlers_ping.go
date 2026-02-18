package api

import (
	crypto_rand "crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/pobradovic08/route-beacon/internal/central/grpcserver"
	"github.com/pobradovic08/route-beacon/internal/shared/model"
	"github.com/pobradovic08/route-beacon/internal/shared/validate"
	pb "github.com/pobradovic08/route-beacon/proto/routebeacon/v1"
)

// Default ping parameters used when optional fields are omitted.
const (
	defaultPingCount     = 5
	defaultPingTimeoutMs = 5000
	maxPingCount         = 10
	maxPingTimeoutMs     = 10000
)

// SSE response types for ping events.
type pingReplyEvent struct {
	Seq     uint32  `json:"seq"`
	RTTMS   float64 `json:"rtt_ms"`
	TTL     uint32  `json:"ttl"`
	Success bool    `json:"success"`
}

type pingSummaryEvent struct {
	PacketsSent     uint32  `json:"packets_sent"`
	PacketsReceived uint32  `json:"packets_received"`
	LossPct         float64 `json:"loss_pct"`
	RTTMinMS        float64 `json:"rtt_min_ms"`
	RTTAvgMS        float64 `json:"rtt_avg_ms"`
	RTTMaxMS        float64 `json:"rtt_max_ms"`
}

type errorEvent struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// generateCommandID produces a random 32-character hex string suitable for
// use as a diagnostic command identifier.
func generateCommandID() (string, error) {
	b := make([]byte, 16)
	if _, err := crypto_rand.Read(b); err != nil {
		return "", fmt.Errorf("generate command ID: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// Ping handles POST /api/v1/targets/{targetId}/ping with SSE streaming.
func (h *APIHandler) Ping(w http.ResponseWriter, r *http.Request, targetId TargetId) {
	// 1. Look up the target.
	sess := h.Registry.GetTargetByString(targetId)
	if sess == nil {
		WriteProblem(w, http.StatusNotFound, "Not Found",
			fmt.Sprintf("LG target %q does not exist.", targetId))
		return
	}

	// 2. Parse the JSON request body.
	var req PingRequest
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
	count := defaultPingCount
	if req.Count != nil {
		count = *req.Count
	}
	timeoutMs := defaultPingTimeoutMs
	if req.TimeoutMs != nil {
		timeoutMs = *req.TimeoutMs
	}

	// Validate parameter ranges.
	if err := validate.ValidatePingParams(uint8(count), uint32(timeoutMs), maxPingCount, maxPingTimeoutMs); err != nil {
		WriteProblem(w, http.StatusBadRequest, "Bad Request",
			fmt.Sprintf("Invalid ping parameters: %s", err))
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
		Type:        model.CommandTypePing,
		LGTargetID:  sess.LGTargetID(),
		Destination: addr,
		Params: model.CommandParams{
			Count:   uint8(count),
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

	slog.Info("ping command started",
		"command_id", cmd.ID,
		"target", targetId,
		"destination", addr.String(),
		"count", count,
		"timeout_ms", timeoutMs,
	)

	startTime := time.Now()

	// 8. Stream events from the dispatcher's event channel.
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected.
			slog.Info("client disconnected during ping stream",
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
			case "ping_reply":
				if pr, ok := event.Data.(*pb.PingResult); ok {
					rttMs := float64(pr.GetRttUs()) / 1000.0
					if err := sse.WriteEvent("reply", pingReplyEvent{
						Seq:     pr.GetSequence(),
						RTTMS:   rttMs,
						TTL:     pr.GetTtl(),
						Success: pr.GetSuccess(),
					}); err != nil {
						slog.Warn("failed to write reply SSE event",
							"command_id", cmd.ID, "error", err)
						return
					}
				}

			case "ping_summary":
				// The ping_summary event may be sent by collectors that produce
				// an aggregate summary before the final "complete" event.
				if data, ok := event.Data.(map[string]any); ok {
					if err := sse.WriteEvent("summary", data); err != nil {
						slog.Warn("failed to write summary SSE event",
							"command_id", cmd.ID, "error", err)
						return
					}
				}

			case "complete":
				if err := sse.WriteEvent("complete", struct{}{}); err != nil {
					slog.Warn("failed to write complete SSE event",
						"command_id", cmd.ID, "error", err)
				}
				slog.Info("ping command completed",
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
				slog.Warn("ping command error",
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

// Ensure the Dispatcher field is usable — this is a compile-time interface check.
var _ interface {
	Dispatch(cmd *model.DiagnosticCommand) (<-chan grpcserver.CommandEvent, error)
} = (*grpcserver.CommandDispatcher)(nil)
