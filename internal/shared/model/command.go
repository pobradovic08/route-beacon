package model

import (
	"net/netip"
	"time"
)

type CommandType string

const (
	CommandTypePing       CommandType = "ping"
	CommandTypeTraceroute CommandType = "traceroute"
)

type CommandStatus string

const (
	CommandStatusPending   CommandStatus = "pending"
	CommandStatusExecuting CommandStatus = "executing"
	CommandStatusCompleted CommandStatus = "completed"
	CommandStatusFailed    CommandStatus = "failed"
	CommandStatusTimeout   CommandStatus = "timeout"
)

type DiagnosticCommand struct {
	ID          string         `json:"id"`
	Type        CommandType    `json:"type"`
	LGTargetID  LGTargetID     `json:"lg_target_id"`
	Destination netip.Addr     `json:"destination"`
	Params      CommandParams  `json:"params"`
	Status      CommandStatus  `json:"status"`
	ClientIP    netip.Addr     `json:"client_ip"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   time.Time      `json:"started_at,omitempty"`
	CompletedAt time.Time      `json:"completed_at,omitempty"`
	Result      *CommandResult `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
}

type CommandParams struct {
	Count      uint8         `json:"count,omitempty"`
	Timeout    time.Duration `json:"timeout,omitempty"`
	MaxHops    uint8         `json:"max_hops,omitempty"`
	PacketSize uint16        `json:"packet_size,omitempty"`
}

type CommandResult struct {
	PingResult       *PingResult       `json:"ping_result,omitempty"`
	TracerouteResult *TracerouteResult `json:"traceroute_result,omitempty"`
	PlainText        string            `json:"plain_text"`
}

type PingResult struct {
	Replies []PingReply `json:"replies"`
	Summary PingSummary `json:"summary"`
}

type PingReply struct {
	Seq     uint16        `json:"seq"`
	RTT     time.Duration `json:"rtt"`
	Success bool          `json:"success"`
	TTL     uint8         `json:"ttl,omitempty"`
}

type PingSummary struct {
	PacketsSent     uint16        `json:"packets_sent"`
	PacketsReceived uint16        `json:"packets_received"`
	PacketLoss      float64       `json:"packet_loss"`
	RTTMin          time.Duration `json:"rtt_min"`
	RTTMax          time.Duration `json:"rtt_max"`
	RTTAvg          time.Duration `json:"rtt_avg"`
	RTTStdDev       time.Duration `json:"rtt_stddev"`
}

type TracerouteResult struct {
	Hops      []TracerouteHop `json:"hops"`
	Completed bool            `json:"completed"`
}

type TracerouteHop struct {
	TTL    uint8             `json:"ttl"`
	Probes []TracerouteProbe `json:"probes"`
}

type TracerouteProbe struct {
	Address string        `json:"address,omitempty"`
	RTT     time.Duration `json:"rtt"`
	Success bool          `json:"success"`
}
