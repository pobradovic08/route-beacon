package grpcserver

import (
	"net/netip"
	"testing"
	"time"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// newTestCommand builds a minimal DiagnosticCommand for testing.
func newTestCommand(id string, collectorID, sessionID string) *model.DiagnosticCommand {
	return &model.DiagnosticCommand{
		ID:   id,
		Type: model.CommandTypePing,
		LGTargetID: model.LGTargetID{
			CollectorID: collectorID,
			SessionID:   sessionID,
		},
		Destination: netip.MustParseAddr("8.8.8.8"),
		Params: model.CommandParams{
			Count:   4,
			Timeout: 5 * time.Second,
		},
	}
}

func TestDispatchAndComplete(t *testing.T) {
	t.Run("dispatch, report result, complete, verify channel closed", func(t *testing.T) {
		d := NewCommandDispatcher()
		collectorID := "collector-1"
		d.RegisterCollector(collectorID)

		cmd := newTestCommand("cmd-1", collectorID, "session-1")

		// Dispatch the command.
		eventCh, err := d.Dispatch(cmd)
		if err != nil {
			t.Fatalf("Dispatch returned unexpected error: %v", err)
		}
		if eventCh == nil {
			t.Fatal("Dispatch returned nil event channel")
		}

		// Drain the command from the collector's command channel so it does
		// not block future operations and simulates a collector picking it up.
		cmdCh := d.GetCommandChannel(collectorID)
		select {
		case received := <-cmdCh:
			if received.ID != cmd.ID {
				t.Fatalf("received command ID = %q, want %q", received.ID, cmd.ID)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for command on collector channel")
		}

		// Report a result event.
		d.ReportResult(cmd.ID, CommandEvent{
			Type: "ping_reply",
			Data: "test-data",
		})

		// Read the event from the channel.
		select {
		case ev := <-eventCh:
			if ev.Type != "ping_reply" {
				t.Errorf("event type = %q, want %q", ev.Type, "ping_reply")
			}
			if ev.Data != "test-data" {
				t.Errorf("event data = %v, want %q", ev.Data, "test-data")
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event on event channel")
		}

		// Complete the command.
		d.CompleteCommand(cmd.ID)

		// Verify the event channel is closed: reading should return zero value
		// with ok=false.
		select {
		case _, ok := <-eventCh:
			if ok {
				// We may get a buffered event; drain and check again.
				// In this test, no further events were sent, so ok should be false.
				t.Error("expected event channel to be closed, but got a value")
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event channel to close")
		}
	})
}

func TestDispatchConcurrencyLimit(t *testing.T) {
	t.Run("exceeding maxPerTarget returns concurrency error", func(t *testing.T) {
		d := NewCommandDispatcher()
		collectorID := "collector-1"
		sessionID := "session-1"
		d.RegisterCollector(collectorID)

		// Dispatch maxPerTarget commands for the same target (default is 2).
		for i := 0; i < d.maxPerTarget; i++ {
			cmd := newTestCommand(
				"cmd-"+string(rune('A'+i)),
				collectorID,
				sessionID,
			)
			_, err := d.Dispatch(cmd)
			if err != nil {
				t.Fatalf("Dispatch #%d returned unexpected error: %v", i, err)
			}
		}

		// The next dispatch for the same target should fail.
		overflowCmd := newTestCommand("cmd-overflow", collectorID, sessionID)
		_, err := d.Dispatch(overflowCmd)
		if err == nil {
			t.Fatal("expected concurrency limit error, got nil")
		}

		// Verify the error message mentions "concurrency limit".
		if got := err.Error(); !containsSubstring(got, "concurrency limit") {
			t.Errorf("error message = %q, want it to contain %q", got, "concurrency limit")
		}
	})
}

func TestUnregisterCleansUpInflight(t *testing.T) {
	t.Run("unregister sends error event and closes inflight channels", func(t *testing.T) {
		d := NewCommandDispatcher()
		collectorID := "collector-1"
		d.RegisterCollector(collectorID)

		cmd := newTestCommand("cmd-1", collectorID, "session-1")

		eventCh, err := d.Dispatch(cmd)
		if err != nil {
			t.Fatalf("Dispatch returned unexpected error: %v", err)
		}

		// Read the command from the collector channel to simulate it being
		// picked up by a collector. This moves the command from the channel
		// to the "already sent" state.
		cmdCh := d.GetCommandChannel(collectorID)
		select {
		case <-cmdCh:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting to drain command from collector channel")
		}

		// Unregister the collector. This should clean up inflight commands
		// that were already read from the channel.
		d.UnregisterCollector(collectorID)

		// The event channel should receive an error event and then be closed.
		gotError := false
		timeout := time.After(2 * time.Second)
		for {
			select {
			case ev, ok := <-eventCh:
				if !ok {
					// Channel closed.
					if !gotError {
						t.Error("event channel closed without receiving error event")
					}
					return
				}
				if ev.Type == "error" {
					gotError = true
				}
			case <-timeout:
				t.Fatal("timed out waiting for error event and channel close")
				return
			}
		}
	})
}

func TestDispatchToUnknownCollector(t *testing.T) {
	t.Run("dispatch to unregistered collector returns error", func(t *testing.T) {
		d := NewCommandDispatcher()

		cmd := newTestCommand("cmd-1", "nonexistent-collector", "session-1")

		eventCh, err := d.Dispatch(cmd)
		if err == nil {
			t.Fatal("expected error dispatching to unknown collector, got nil")
		}
		if eventCh != nil {
			t.Error("expected nil event channel on error, got non-nil")
		}

		// Verify the error message mentions the collector.
		if got := err.Error(); !containsSubstring(got, "not registered") {
			t.Errorf("error message = %q, want it to contain %q", got, "not registered")
		}
	})
}

// containsSubstring checks if s contains substr. Uses a simple linear scan
// instead of importing strings to keep test dependencies minimal.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
