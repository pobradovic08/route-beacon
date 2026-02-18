package api

import (
	"encoding/json"
	"testing"
)

func TestErrorEventSerialization(t *testing.T) {
	tests := []struct {
		name     string
		event    errorEvent
		wantJSON string
	}{
		{
			name:     "basic error",
			event:    errorEvent{Code: "ERROR", Message: "test"},
			wantJSON: `{"code":"ERROR","message":"test"}`,
		},
		{
			name:     "unknown error code",
			event:    errorEvent{Code: "UNKNOWN", Message: "something went wrong"},
			wantJSON: `{"code":"UNKNOWN","message":"something went wrong"}`,
		},
		{
			name:     "empty fields",
			event:    errorEvent{Code: "", Message: ""},
			wantJSON: `{"code":"","message":""}`,
		},
		{
			name:     "special characters in message",
			event:    errorEvent{Code: "TIMEOUT", Message: "connection to 10.0.0.1:8080 timed out after 5s"},
			wantJSON: `{"code":"TIMEOUT","message":"connection to 10.0.0.1:8080 timed out after 5s"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("json.Marshal(errorEvent): %v", err)
			}
			got := string(data)
			if got != tt.wantJSON {
				t.Errorf("json.Marshal = %s, want %s", got, tt.wantJSON)
			}

			// Verify round-trip deserialization.
			var decoded errorEvent
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal(errorEvent): %v", err)
			}
			if decoded.Code != tt.event.Code {
				t.Errorf("decoded.Code = %q, want %q", decoded.Code, tt.event.Code)
			}
			if decoded.Message != tt.event.Message {
				t.Errorf("decoded.Message = %q, want %q", decoded.Message, tt.event.Message)
			}
		})
	}
}

func TestPingSummaryEventSerialization(t *testing.T) {
	tests := []struct {
		name     string
		event    pingSummaryEvent
		wantKeys []string
	}{
		{
			name: "full summary",
			event: pingSummaryEvent{
				PacketsSent:     5,
				PacketsReceived: 4,
				LossPct:         20.0,
				RTTMinMS:        1.234,
				RTTAvgMS:        5.678,
				RTTMaxMS:        10.123,
			},
			wantKeys: []string{
				"packets_sent", "packets_received", "loss_pct",
				"rtt_min_ms", "rtt_avg_ms", "rtt_max_ms",
			},
		},
		{
			name:  "zero values",
			event: pingSummaryEvent{},
			wantKeys: []string{
				"packets_sent", "packets_received", "loss_pct",
				"rtt_min_ms", "rtt_avg_ms", "rtt_max_ms",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("json.Marshal(pingSummaryEvent): %v", err)
			}

			// Verify all expected JSON field names are present.
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("json.Unmarshal to map: %v", err)
			}

			for _, key := range tt.wantKeys {
				if _, ok := raw[key]; !ok {
					t.Errorf("missing expected JSON key %q in output: %s", key, string(data))
				}
			}

			// Verify field count matches expected keys.
			if len(raw) != len(tt.wantKeys) {
				t.Errorf("JSON has %d keys, want %d; output: %s", len(raw), len(tt.wantKeys), string(data))
			}

			// Verify round-trip values for the full summary case.
			if tt.name == "full summary" {
				var decoded pingSummaryEvent
				if err := json.Unmarshal(data, &decoded); err != nil {
					t.Fatalf("json.Unmarshal(pingSummaryEvent): %v", err)
				}
				if decoded.PacketsSent != tt.event.PacketsSent {
					t.Errorf("PacketsSent = %d, want %d", decoded.PacketsSent, tt.event.PacketsSent)
				}
				if decoded.PacketsReceived != tt.event.PacketsReceived {
					t.Errorf("PacketsReceived = %d, want %d", decoded.PacketsReceived, tt.event.PacketsReceived)
				}
				if decoded.LossPct != tt.event.LossPct {
					t.Errorf("LossPct = %f, want %f", decoded.LossPct, tt.event.LossPct)
				}
				if decoded.RTTMinMS != tt.event.RTTMinMS {
					t.Errorf("RTTMinMS = %f, want %f", decoded.RTTMinMS, tt.event.RTTMinMS)
				}
				if decoded.RTTAvgMS != tt.event.RTTAvgMS {
					t.Errorf("RTTAvgMS = %f, want %f", decoded.RTTAvgMS, tt.event.RTTAvgMS)
				}
				if decoded.RTTMaxMS != tt.event.RTTMaxMS {
					t.Errorf("RTTMaxMS = %f, want %f", decoded.RTTMaxMS, tt.event.RTTMaxMS)
				}
			}
		})
	}
}

func TestPingReplyEventSerialization(t *testing.T) {
	event := pingReplyEvent{
		Seq:     1,
		RTTMS:   5.26,
		TTL:     118,
		Success: true,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal(pingReplyEvent): %v", err)
	}

	// Verify expected JSON keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal to map: %v", err)
	}

	expectedKeys := []string{"seq", "rtt_ms", "ttl", "success"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing expected JSON key %q in output: %s", key, string(data))
		}
	}

	// Verify round-trip.
	var decoded pingReplyEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(pingReplyEvent): %v", err)
	}
	if decoded != event {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, event)
	}
}

func TestTracerouteHopEventSerialization(t *testing.T) {
	event := tracerouteHopEvent{
		HopNumber: 3,
		Address:   "10.0.0.1",
		RTTMs:     []float64{1.234, 2.345, 3.456},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal(tracerouteHopEvent): %v", err)
	}

	// Verify expected JSON keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal to map: %v", err)
	}

	expectedKeys := []string{"hop_number", "address", "rtt_ms"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing expected JSON key %q in output: %s", key, string(data))
		}
	}

	// Verify round-trip.
	var decoded tracerouteHopEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(tracerouteHopEvent): %v", err)
	}
	if decoded.HopNumber != event.HopNumber {
		t.Errorf("HopNumber = %d, want %d", decoded.HopNumber, event.HopNumber)
	}
	if decoded.Address != event.Address {
		t.Errorf("Address = %q, want %q", decoded.Address, event.Address)
	}
	if len(decoded.RTTMs) != len(event.RTTMs) {
		t.Fatalf("len(RTTMs) = %d, want %d", len(decoded.RTTMs), len(event.RTTMs))
	}
	for i, v := range decoded.RTTMs {
		if v != event.RTTMs[i] {
			t.Errorf("RTTMs[%d] = %f, want %f", i, v, event.RTTMs[i])
		}
	}
}

func TestTracerouteCompleteEventSerialization(t *testing.T) {
	tests := []struct {
		name     string
		event    tracerouteCompleteEvent
		wantJSON string
	}{
		{
			name:     "reached destination",
			event:    tracerouteCompleteEvent{ReachedDestination: true},
			wantJSON: `{"reached_destination":true}`,
		},
		{
			name:     "did not reach destination",
			event:    tracerouteCompleteEvent{ReachedDestination: false},
			wantJSON: `{"reached_destination":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("json.Marshal(tracerouteCompleteEvent): %v", err)
			}
			got := string(data)
			if got != tt.wantJSON {
				t.Errorf("json.Marshal = %s, want %s", got, tt.wantJSON)
			}
		})
	}
}
