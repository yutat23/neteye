package types

import (
	"strings"
	"testing"
	"time"
)

func TestConnectionString(t *testing.T) {
	conn := &Connection{
		Proto:      ProtocolTCP,
		LocalIP:    "192.168.1.100",
		LocalPort:  8080,
		RemoteIP:   "192.168.1.200",
		RemotePort: 443,
		State:      StateEstablished,
		PID:        1234,
		Process:    "test-process",
		CreatedAt:  time.Now(),
	}

	expected := "tcp 192.168.1.100:8080 -> 192.168.1.200:443 [ESTABLISHED] PID:1234 (test-process)"
	result := conn.String()

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestConnectionLocalAddress(t *testing.T) {
	conn := &Connection{
		LocalIP:   "127.0.0.1",
		LocalPort: 8080,
	}

	expected := "127.0.0.1:8080"
	result := conn.LocalAddress()

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestConnectionRemoteAddress(t *testing.T) {
	tests := []struct {
		name     string
		conn     *Connection
		expected string
	}{
		{
			name: "normal remote address",
			conn: &Connection{
				RemoteIP:   "192.168.1.100",
				RemotePort: 443,
			},
			expected: "192.168.1.100:443",
		},
		{
			name: "wildcard remote address",
			conn: &Connection{
				RemoteIP:   "0.0.0.0",
				RemotePort: 0,
			},
			expected: "*:*",
		},
		{
			name: "IPv6 wildcard remote address",
			conn: &Connection{
				RemoteIP:   "::",
				RemotePort: 0,
			},
			expected: "*:*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.conn.RemoteAddress()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestProtocolConstants(t *testing.T) {
	if ProtocolTCP != "tcp" {
		t.Errorf("Expected ProtocolTCP to be 'tcp', got %q", ProtocolTCP)
	}

	if ProtocolUDP != "udp" {
		t.Errorf("Expected ProtocolUDP to be 'udp', got %q", ProtocolUDP)
	}
}

func TestConnectionStateConstants(t *testing.T) {
	states := map[ConnectionState]string{
		StateListen:      "LISTEN",
		StateEstablished: "ESTABLISHED",
		StateTimeWait:    "TIME_WAIT",
		StateCloseWait:   "CLOSE_WAIT",
		StateFinWait1:    "FIN_WAIT1",
		StateFinWait2:    "FIN_WAIT2",
		StateClosing:     "CLOSING",
		StateLastAck:     "LAST_ACK",
		StateSynSent:     "SYN_SENT",
		StateSynRecv:     "SYN_RECV",
		StateUnknown:     "UNKNOWN",
	}

	for state, expected := range states {
		if string(state) != expected {
			t.Errorf("Expected state %q to be %q, got %q", state, expected, string(state))
		}
	}
}

func TestExitCodes(t *testing.T) {
	codes := map[ExitCode]int{
		ExitOK:           0,
		ExitGeneralError: 1,
		ExitUsageError:   2,
		ExitPermError:    3,
	}

	for code, expected := range codes {
		if int(code) != expected {
			t.Errorf("Expected exit code %v to be %d, got %d", code, expected, int(code))
		}
	}
}

func TestLogEntryJSON(t *testing.T) {
	entry := &LogEntry{
		Level:     "info",
		Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Operation: "test",
		Message:   "test message",
		Code:      "TEST_CODE",
	}

	json := entry.JSON()
	if json == "" {
		t.Error("Expected non-empty JSON string")
	}

	// Basic validation - should contain key fields
	if !contains(json, "\"level\":\"info\"") {
		t.Error("JSON should contain level field")
	}
	if !contains(json, "\"op\":\"test\"") {
		t.Error("JSON should contain operation field")
	}
	if !contains(json, "\"msg\":\"test message\"") {
		t.Error("JSON should contain message field")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
