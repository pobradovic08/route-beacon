package api

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// AuditLog writes a structured audit log entry for a diagnostic command execution.
// It captures the command details and client IP per FR-028.
func AuditLog(cmd *model.DiagnosticCommand, r *http.Request) {
	clientIP := extractClientIP(r)

	slog.Info("audit: diagnostic command",
		"command_id", cmd.ID,
		"command_type", cmd.Type,
		"destination", cmd.Destination.String(),
		"collector_id", cmd.LGTargetID.CollectorID,
		"session_id", cmd.LGTargetID.SessionID,
		"client_ip", clientIP,
		"timestamp", cmd.CreatedAt.Format(time.RFC3339),
		"params_count", cmd.Params.Count,
		"params_timeout", cmd.Params.Timeout.String(),
		"params_max_hops", cmd.Params.MaxHops,
	)
}

// AuditLogComplete writes a structured audit log entry when a command completes.
func AuditLogComplete(commandID string, commandType model.CommandType, duration time.Duration, success bool) {
	slog.Info("audit: diagnostic command completed",
		"command_id", commandID,
		"command_type", commandType,
		"duration", duration.String(),
		"success", success,
	)
}

// extractClientIP extracts the client IP from the request using RemoteAddr.
// Forwarding headers (X-Forwarded-For, X-Real-IP) are intentionally ignored
// because there is no trusted-proxy configuration to validate them against,
// and accepting them directly would allow clients to forge logged source IPs.
func extractClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
