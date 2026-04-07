package engineio

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// =============================================================================
// socket.io-client v4.8.3 compatibility tests
//
// These tests simulate the exact connection flow that socket.io-client v4.8.3
// performs, verifying:
//   - Polling handshake: EIO=4 parameter, OPEN packet with sid/pingInterval/pingTimeout
//   - Polling→WebSocket upgrade: probe ping/pong, UPGRADE packet, transport switch
//   - sid issuance: unique per session, URL-safe encoding
//   - pingInterval/pingTimeout negotiation: correct millisecond values in OPEN packet
//
// Reference: https://socket.io/docs/v4/engine-io-protocol/
// =============================================================================

// TestClientCompat_PollingHandshake_SIDAndTimings verifies the initial polling
// handshake returns a valid OPEN packet matching socket.io-client v4.8.3 expectations:
//   - Packet starts with '0' (OPEN type)
//   - Contains sid, upgrades, pingInterval, pingTimeout, maxPayload
//   - sid is non-empty and URL-safe (base64url without padding)
//   - pingInterval and pingTimeout are in milliseconds
//   - upgrades includes "websocket"
func TestClientCompat_PollingHandshake_SIDAndTimings(t *testing.T) {
	config := ServerConfig{
		PingInterval: 25 * time.Second,
		PingTimeout:  20 * time.Second,
		MaxPayload:   1_000_000,
		Upgrades:     []string{"websocket"},
	}
	srv := NewServer(config)

	// socket.io-client v4.8.3 sends: GET /socket.io/?EIO=4&transport=polling&t=<timestamp>
	req := httptest.NewRequest("GET", "/socket.io/?EIO=4&transport=polling&t=OxJEfOv", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("handshake status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Must start with '0' (OPEN packet type)
	if len(bodyStr) == 0 || bodyStr[0] != '0' {
		t.Fatalf("expected OPEN packet starting with '0', got %q", bodyStr)
	}

	// Parse the OPEN packet JSON
	var openData openPacketData
	if err := json.Unmarshal(body[1:], &openData); err != nil {
		t.Fatalf("failed to parse OPEN packet JSON: %v\nbody: %s", err, bodyStr)
	}

	// Verify sid
	if openData.SID == "" {
		t.Error("sid must not be empty")
	}
	if len(openData.SID) < 10 {
		t.Errorf("sid too short (%d chars), expected base64url-encoded value", len(openData.SID))
	}
	// sid should be URL-safe (no +, /, or = characters in base64url without padding)
	if strings.ContainsAny(openData.SID, "+/=") {
		t.Errorf("sid %q contains non-URL-safe characters", openData.SID)
	}

	// Verify pingInterval (25000ms)
	if openData.PingInterval != 25000 {
		t.Errorf("pingInterval = %d, want 25000 (ms)", openData.PingInterval)
	}

	// Verify pingTimeout (20000ms)
	if openData.PingTimeout != 20000 {
		t.Errorf("pingTimeout = %d, want 20000 (ms)", openData.PingTimeout)
	}

	// Verify upgrades contains "websocket"
	if len(openData.Upgrades) == 0 {
		t.Fatal("upgrades must not be empty, expected [\"websocket\"]")
	}
	foundWS := false
	for _, u := range openData.Upgrades {
		if u == "websocket" {
			foundWS = true
			break
		}
	}
	if !foundWS {
		t.Errorf("upgrades = %v, must include \"websocket\"", openData.Upgrades)
	}

	// Verify maxPayload
	if openData.MaxPayload != 1_000_000 {
		t.Errorf("maxPayload = %d, want 1000000", openData.MaxPayload)
	}

	// Verify CORS headers (socket.io-client sends Origin)
	if resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("CORS origin = %q, want %q",
			resp.Header.Get("Access-Control-Allow-Origin"), "http://localhost:5173")
	}
	if resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("missing Access-Control-Allow-Credentials: true")
	}
}

// TestClientCompat_PollingHandshake_UniqueSIDs verifies that each polling handshake
// produces a unique session ID, as socket.io-client v4.8.3 expects.
func TestClientCompat_PollingHandshake_UniqueSIDs(t *testing.T) {
	srv := NewServer(DefaultConfig())

	sids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/socket.io/?EIO=4&transport=polling", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		body, _ := io.ReadAll(w.Result().Body)
		var openData openPacketData
		if err := json.Unmarshal(body[1:], &openData); err != nil {
			t.Fatalf("iteration %d: failed to parse OPEN packet: %v", i, err)
		}

		if sids[openData.SID] {
			t.Fatalf("iteration %d: duplicate SID %q", i, openData.SID)
		}
		sids[openData.SID] = true
	}
}

// TestClientCompat_PollingHandshake_CustomPingValues verifies that different
// pingInterval/pingTimeout configurations are correctly negotiated in the
// OPEN packet, as the client uses these to configure its heartbeat timers.
func TestClientCompat_PollingHandshake_CustomPingValues(t *testing.T) {
	tests := []struct {
		name         string
		pingInterval time.Duration
		pingTimeout  time.Duration
		wantInterval int
		wantTimeout  int
	}{
		{
			name:         "default python-socketio values",
			pingInterval: 25 * time.Second,
			pingTimeout:  20 * time.Second,
			wantInterval: 25000,
			wantTimeout:  20000,
		},
		{
			name:         "fast heartbeat",
			pingInterval: 5 * time.Second,
			pingTimeout:  3 * time.Second,
			wantInterval: 5000,
			wantTimeout:  3000,
		},
		{
			name:         "slow heartbeat",
			pingInterval: 60 * time.Second,
			pingTimeout:  30 * time.Second,
			wantInterval: 60000,
			wantTimeout:  30000,
		},
		{
			name:         "sub-second precision",
			pingInterval: 500 * time.Millisecond,
			pingTimeout:  200 * time.Millisecond,
			wantInterval: 500,
			wantTimeout:  200,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := ServerConfig{
				PingInterval: tc.pingInterval,
				PingTimeout:  tc.pingTimeout,
				MaxPayload:   1_000_000,
				Upgrades:     []string{"websocket"},
			}
			srv := NewServer(config)

			req := httptest.NewRequest("GET", "/socket.io/?EIO=4&transport=polling", nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			body, _ := io.ReadAll(w.Result().Body)
			var openData openPacketData
			if err := json.Unmarshal(body[1:], &openData); err != nil {
				t.Fatalf("failed to parse OPEN packet: %v", err)
			}

			if openData.PingInterval != tc.wantInterval {
				t.Errorf("pingInterval = %d, want %d", openData.PingInterval, tc.wantInterval)
			}
			if openData.PingTimeout != tc.wantTimeout {
				t.Errorf("pingTimeout = %d, want %d", openData.PingTimeout, tc.wantTimeout)
			}
		})
	}
}

// TestClientCompat_PollingToWebSocketUpgrade simulates the exact upgrade flow
// that socket.io-client v4.8.3 performs:
//
//  1. GET /socket.io/?EIO=4&transport=polling → OPEN packet (sid issued)
//  2. POST SIO CONNECT packet (type "40") → "ok"
//  3. GET → SIO CONNECT ACK (type "40{\"sid\":\"...\"}") [optional, depends on timing]
//  4. WS dial /socket.io/?EIO=4&transport=websocket&sid=<sid>
//  5. WS send "2probe" (EIO PING with "probe" data)
//  6. WS recv "3probe" (EIO PONG with "probe" data)
//  7. WS send "5" (EIO UPGRADE)
//  8. Transport switches to websocket
//  9. WS messaging works
func TestClientCompat_PollingToWebSocketUpgrade(t *testing.T) {
	srv := NewServer(DefaultConfig())

	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			// Echo back any message
			s.Send(data)
		})
	})

	server := httptest.NewServer(srv)
	defer server.Close()

	// --- Step 1: Polling handshake ---
	resp, err := server.Client().Get(server.URL + "/socket.io/?EIO=4&transport=polling&t=OxJEfOv")
	if err != nil {
		t.Fatalf("polling handshake failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("handshake status = %d, want 200", resp.StatusCode)
	}

	if body[0] != '0' {
		t.Fatalf("expected OPEN packet, got %q", string(body))
	}

	var openData openPacketData
	if err := json.Unmarshal(body[1:], &openData); err != nil {
		t.Fatalf("failed to parse OPEN: %v", err)
	}

	sid := openData.SID
	if sid == "" {
		t.Fatal("sid is empty")
	}

	// Verify session exists on server with polling transport
	session := srv.GetSession(sid)
	if session == nil {
		t.Fatal("session not found after handshake")
	}
	if session.Transport != "polling" {
		t.Errorf("initial transport = %q, want \"polling\"", session.Transport)
	}

	// --- Step 2: Client sends SIO CONNECT via polling POST ---
	// socket.io-client v4.8.3 sends "40" (EIO MESSAGE + SIO CONNECT for default namespace)
	postResp, err := server.Client().Post(
		server.URL+"/socket.io/?EIO=4&transport=polling&sid="+sid,
		"text/plain",
		strings.NewReader("40"),
	)
	if err != nil {
		t.Fatalf("SIO CONNECT POST failed: %v", err)
	}
	postBody, _ := io.ReadAll(postResp.Body)
	postResp.Body.Close()

	if string(postBody) != "ok" {
		t.Errorf("POST response = %q, want \"ok\"", postBody)
	}

	// --- Step 3: Open WebSocket for upgrade ---
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/socket.io/?EIO=4&transport=websocket&sid=" + sid
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer wsConn.Close()

	// --- Step 4: Probe sequence ---
	// Client sends PING "probe"
	if err := wsConn.WriteMessage(websocket.TextMessage, []byte("2probe")); err != nil {
		t.Fatalf("failed to send probe ping: %v", err)
	}

	// Server responds with PONG "probe"
	wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := wsConn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read probe pong: %v", err)
	}
	if string(msg) != "3probe" {
		t.Fatalf("probe response = %q, want \"3probe\"", string(msg))
	}

	// --- Step 5: Client sends UPGRADE packet ---
	if err := wsConn.WriteMessage(websocket.TextMessage, []byte("5")); err != nil {
		t.Fatalf("failed to send UPGRADE: %v", err)
	}

	// Wait for transport switch
	time.Sleep(100 * time.Millisecond)

	// --- Step 6: Verify transport switched ---
	if session.Transport != "websocket" {
		t.Errorf("transport after upgrade = %q, want \"websocket\"", session.Transport)
	}

	// --- Step 7: Verify messaging works over WebSocket ---
	// After upgrade, any buffered packets (e.g., SIO CONNECT ACK from the POST)
	// may be delivered first. Drain them before testing echo.
	wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Send an EIO MESSAGE (4) + payload
	if err := wsConn.WriteMessage(websocket.TextMessage, []byte("4hello_after_upgrade")); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Read messages until we get our echo (skip any buffered packets from polling phase)
	var gotEcho bool
	for i := 0; i < 5; i++ {
		_, echoMsg, err := wsConn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message: %v", err)
		}
		if string(echoMsg) == "4hello_after_upgrade" {
			gotEcho = true
			break
		}
		// Other messages (e.g., SIO CONNECT ACK "40{...}") are expected during upgrade
		t.Logf("skipped buffered packet: %q", string(echoMsg))
	}
	if !gotEcho {
		t.Error("did not receive echo of \"4hello_after_upgrade\" after upgrade")
	}
}

// TestClientCompat_PollingUpgrade_ReleasesBlockedPoll verifies that during the
// upgrade probe phase, any pending polling GET request is released with a NOOP
// packet. socket.io-client v4.8.3 keeps a polling GET outstanding and expects
// it to return during the upgrade so it can complete the transport switch.
func TestClientCompat_PollingUpgrade_ReleasesBlockedPoll(t *testing.T) {
	srv := NewServer(DefaultConfig())
	server := httptest.NewServer(srv)
	defer server.Close()

	// Handshake
	resp, err := server.Client().Get(server.URL + "/socket.io/?EIO=4&transport=polling&t=test1")
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var openData openPacketData
	json.Unmarshal(body[1:], &openData)
	sid := openData.SID

	// Start a blocking polling GET (simulates client's pending poll)
	pollDone := make(chan string, 1)
	go func() {
		r, err := server.Client().Get(server.URL + "/socket.io/?EIO=4&transport=polling&sid=" + sid)
		if err != nil {
			pollDone <- "error:" + err.Error()
			return
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		pollDone <- string(b)
	}()

	// Give poll time to block
	time.Sleep(50 * time.Millisecond)

	// Start WebSocket upgrade
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/socket.io/?EIO=4&transport=websocket&sid=" + sid
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer wsConn.Close()

	// Send probe
	wsConn.WriteMessage(websocket.TextMessage, []byte("2probe"))
	wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, _ := wsConn.ReadMessage()
	if string(msg) != "3probe" {
		t.Fatalf("probe pong = %q, want \"3probe\"", string(msg))
	}

	// The pending poll should be released with NOOP
	select {
	case result := <-pollDone:
		if !strings.Contains(result, "6") {
			t.Errorf("pending poll response = %q, expected NOOP packet (6)", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pending poll was not released during upgrade probe")
	}

	// Complete upgrade
	wsConn.WriteMessage(websocket.TextMessage, []byte("5"))
}

// TestClientCompat_DirectWebSocketHandshake verifies that socket.io-client v4.8.3
// can also connect directly via WebSocket (transports: ['websocket'] option).
// In this mode, there's no polling phase — the client opens a WebSocket directly.
func TestClientCompat_DirectWebSocketHandshake(t *testing.T) {
	config := ServerConfig{
		PingInterval: 25 * time.Second,
		PingTimeout:  20 * time.Second,
		MaxPayload:   1_000_000,
		Upgrades:     []string{"websocket"},
	}
	srv := NewServer(config)
	server := httptest.NewServer(srv)
	defer server.Close()

	// socket.io-client with transports: ['websocket'] connects directly
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/socket.io/?EIO=4&transport=websocket"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	// Read OPEN packet
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read OPEN packet: %v", err)
	}

	msgStr := string(msg)
	if msgStr[0] != '0' {
		t.Fatalf("expected OPEN packet, got %q", msgStr)
	}

	var openData openPacketData
	if err := json.Unmarshal([]byte(msgStr[1:]), &openData); err != nil {
		t.Fatalf("failed to parse OPEN: %v", err)
	}

	// sid must be present
	if openData.SID == "" {
		t.Error("sid must not be empty")
	}

	// pingInterval/pingTimeout must match config
	if openData.PingInterval != 25000 {
		t.Errorf("pingInterval = %d, want 25000", openData.PingInterval)
	}
	if openData.PingTimeout != 20000 {
		t.Errorf("pingTimeout = %d, want 20000", openData.PingTimeout)
	}

	// Direct WS connections should NOT offer further upgrades
	if len(openData.Upgrades) != 0 {
		t.Errorf("upgrades = %v, want [] for direct WebSocket", openData.Upgrades)
	}

	// maxPayload must be present
	if openData.MaxPayload != 1_000_000 {
		t.Errorf("maxPayload = %d, want 1000000", openData.MaxPayload)
	}

	// Session should be websocket transport
	session := srv.GetSession(openData.SID)
	if session == nil {
		t.Fatal("session not found")
	}
	if session.Transport != "websocket" {
		t.Errorf("transport = %q, want \"websocket\"", session.Transport)
	}
}

// TestClientCompat_UpgradeProbeTimeout verifies that if the WebSocket probe
// times out (client doesn't send "2probe"), the upgrade is cancelled and
// polling continues to work. socket.io-client v4.8.3 handles this gracefully.
func TestClientCompat_UpgradeProbeTimeout(t *testing.T) {
	config := ServerConfig{
		PingInterval: 10 * time.Second,
		PingTimeout:  500 * time.Millisecond, // short for test
		MaxPayload:   1_000_000,
		Upgrades:     []string{"websocket"},
	}
	srv := NewServer(config)

	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			s.Send(data)
		})
	})

	server := httptest.NewServer(srv)
	defer server.Close()

	// Handshake via polling
	resp, err := server.Client().Get(server.URL + "/socket.io/?EIO=4&transport=polling")
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var openData openPacketData
	json.Unmarshal(body[1:], &openData)
	sid := openData.SID

	// Open WebSocket but don't send probe — let it timeout
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/socket.io/?EIO=4&transport=websocket&sid=" + sid
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer wsConn.Close()

	// Wait for probe timeout
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, readErr := wsConn.ReadMessage()
	if readErr == nil {
		t.Error("expected read error after probe timeout")
	}

	// Session should remain alive on polling transport
	session := srv.GetSession(sid)
	if session == nil {
		t.Fatal("session should still exist after failed upgrade")
	}
	if session.Transport != "polling" {
		t.Errorf("transport = %q, want \"polling\" after failed upgrade", session.Transport)
	}

	// Polling should still work
	postResp, err := server.Client().Post(
		server.URL+"/socket.io/?EIO=4&transport=polling&sid="+sid,
		"text/plain",
		strings.NewReader("4test_after_failed_upgrade"),
	)
	if err != nil {
		t.Fatalf("polling POST after failed upgrade: %v", err)
	}
	postBody, _ := io.ReadAll(postResp.Body)
	postResp.Body.Close()
	if string(postBody) != "ok" {
		t.Errorf("POST response = %q, want \"ok\"", postBody)
	}
}

// TestClientCompat_UpgradeInvalidProbe verifies that a bad probe message
// (not "2probe") causes the upgrade to fail gracefully without affecting
// the existing polling session.
func TestClientCompat_UpgradeInvalidProbe(t *testing.T) {
	srv := NewServer(DefaultConfig())
	server := httptest.NewServer(srv)
	defer server.Close()

	// Handshake
	resp, err := server.Client().Get(server.URL + "/socket.io/?EIO=4&transport=polling")
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var openData openPacketData
	json.Unmarshal(body[1:], &openData)
	sid := openData.SID

	// Open WebSocket and send invalid probe
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/socket.io/?EIO=4&transport=websocket&sid=" + sid
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer wsConn.Close()

	// Send PING with wrong data (not "probe")
	wsConn.WriteMessage(websocket.TextMessage, []byte("2invalid"))

	// Server should close the WebSocket
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, readErr := wsConn.ReadMessage()
	if readErr == nil {
		t.Error("expected connection close after invalid probe")
	}

	// Session should remain on polling
	session := srv.GetSession(sid)
	if session == nil {
		t.Fatal("session should still exist")
	}
	if session.Transport != "polling" {
		t.Errorf("transport = %q, want \"polling\"", session.Transport)
	}
}

// TestClientCompat_ServerPingFromClient verifies the server's ping/pong
// heartbeat behavior. In EIO v4, the SERVER sends PING and the CLIENT
// responds with PONG. socket.io-client v4.8.3 expects this pattern.
func TestClientCompat_ServerPingFromClient(t *testing.T) {
	config := ServerConfig{
		PingInterval: 200 * time.Millisecond, // fast for testing
		PingTimeout:  100 * time.Millisecond,
		MaxPayload:   1_000_000,
		Upgrades:     []string{"websocket"},
	}
	srv := NewServer(config)
	server := httptest.NewServer(srv)
	defer server.Close()

	// Connect directly via WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/socket.io/?EIO=4&transport=websocket"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	// Read OPEN
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = conn.ReadMessage() // OPEN packet
	if err != nil {
		t.Fatalf("failed to read OPEN: %v", err)
	}

	// Wait for server PING
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read server PING: %v", err)
	}
	if string(msg) != "2" {
		t.Errorf("expected PING packet \"2\", got %q", string(msg))
	}

	// Client responds with PONG (socket.io-client v4.8.3 behavior)
	if err := conn.WriteMessage(websocket.TextMessage, []byte("3")); err != nil {
		t.Fatalf("failed to send PONG: %v", err)
	}

	// Should receive another PING after the interval
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg2, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read second PING: %v", err)
	}
	if string(msg2) != "2" {
		t.Errorf("expected second PING \"2\", got %q", string(msg2))
	}
}

// TestClientCompat_MultiPayload_RecordSeparator verifies that the server
// correctly handles multiple EIO packets in a single polling payload,
// separated by the record separator (0x1e) as per EIO v4 spec.
// socket.io-client v4.8.3 may batch multiple packets in one POST.
func TestClientCompat_MultiPayload_RecordSeparator(t *testing.T) {
	srv := NewServer(DefaultConfig())
	var received []string

	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			received = append(received, string(data))
		})
	})

	// Handshake
	req := httptest.NewRequest("GET", "/socket.io/?EIO=4&transport=polling", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	var openData openPacketData
	json.Unmarshal(body[1:], &openData)

	// Post multiple packets separated by record separator (0x1e)
	payload := "4msg1\x1e4msg2\x1e4msg3"
	postReq := httptest.NewRequest("POST",
		"/socket.io/?EIO=4&transport=polling&sid="+openData.SID,
		strings.NewReader(payload))
	postW := httptest.NewRecorder()
	srv.ServeHTTP(postW, postReq)

	if postW.Result().StatusCode != http.StatusOK {
		t.Fatalf("POST status = %d, want 200", postW.Result().StatusCode)
	}

	if len(received) != 3 {
		t.Fatalf("received %d messages, want 3", len(received))
	}
	expected := []string{"msg1", "msg2", "msg3"}
	for i, want := range expected {
		if received[i] != want {
			t.Errorf("received[%d] = %q, want %q", i, received[i], want)
		}
	}
}

// TestClientCompat_FullProtocolSequence_PollingToWS performs the complete
// protocol sequence that socket.io-client v4.8.3 executes from connection
// to messaging, matching the exact order of HTTP/WS requests.
func TestClientCompat_FullProtocolSequence_PollingToWS(t *testing.T) {
	config := ServerConfig{
		PingInterval: 25 * time.Second,
		PingTimeout:  20 * time.Second,
		MaxPayload:   1_000_000,
		Upgrades:     []string{"websocket"},
	}
	srv := NewServer(config)

	var echoMessages []string
	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			echoMessages = append(echoMessages, string(data))
			s.Send(data)
		})
	})

	server := httptest.NewServer(srv)
	defer server.Close()

	// ===== Phase 1: Polling handshake =====
	resp, err := server.Client().Get(
		fmt.Sprintf("%s/socket.io/?EIO=4&transport=polling&t=OxJEfOv", server.URL))
	if err != nil {
		t.Fatalf("Phase 1 failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if body[0] != '0' {
		t.Fatalf("expected OPEN, got %q", string(body))
	}

	var openData openPacketData
	json.Unmarshal(body[1:], &openData)
	sid := openData.SID

	// Verify negotiated values
	if openData.PingInterval != 25000 {
		t.Errorf("pingInterval = %d, want 25000", openData.PingInterval)
	}
	if openData.PingTimeout != 20000 {
		t.Errorf("pingTimeout = %d, want 20000", openData.PingTimeout)
	}

	// ===== Phase 2: Send SIO CONNECT + message via polling =====
	// socket.io-client first sends the SIO CONNECT packet
	_, err = server.Client().Post(
		server.URL+"/socket.io/?EIO=4&transport=polling&sid="+sid,
		"text/plain",
		strings.NewReader("40"),
	)
	if err != nil {
		t.Fatalf("Phase 2 SIO CONNECT failed: %v", err)
	}

	// ===== Phase 3: Start pending poll + WS upgrade in parallel =====
	// (This is what socket.io-client v4.8.3 actually does)
	pollDone := make(chan string, 1)
	go func() {
		r, err := server.Client().Get(
			server.URL + "/socket.io/?EIO=4&transport=polling&sid=" + sid)
		if err != nil {
			pollDone <- "error"
			return
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		pollDone <- string(b)
	}()

	time.Sleep(50 * time.Millisecond)

	// ===== Phase 4: WebSocket upgrade =====
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/socket.io/?EIO=4&transport=websocket&sid=" + sid
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Phase 4 WS dial failed: %v", err)
	}
	defer wsConn.Close()

	// Probe sequence
	wsConn.WriteMessage(websocket.TextMessage, []byte("2probe"))
	wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, probeResp, err := wsConn.ReadMessage()
	if err != nil {
		t.Fatalf("probe pong read failed: %v", err)
	}
	if string(probeResp) != "3probe" {
		t.Fatalf("probe = %q, want \"3probe\"", string(probeResp))
	}

	// Pending poll should be released
	select {
	case result := <-pollDone:
		if !strings.Contains(result, "6") && !strings.Contains(result, "4") {
			t.Logf("poll result during upgrade: %q", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pending poll not released during upgrade")
	}

	// Complete upgrade
	wsConn.WriteMessage(websocket.TextMessage, []byte("5"))
	time.Sleep(100 * time.Millisecond)

	// Verify transport switched
	session := srv.GetSession(sid)
	if session.Transport != "websocket" {
		t.Errorf("transport = %q, want \"websocket\"", session.Transport)
	}

	// ===== Phase 5: Messaging over WebSocket =====
	wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	wsConn.WriteMessage(websocket.TextMessage, []byte("4test_message"))
	_, echo, err := wsConn.ReadMessage()
	if err != nil {
		t.Fatalf("Phase 5 echo read failed: %v", err)
	}
	if string(echo) != "4test_message" {
		t.Errorf("echo = %q, want \"4test_message\"", string(echo))
	}

	// ===== Phase 6: Clean close =====
	wsConn.WriteMessage(websocket.TextMessage, []byte("1")) // EIO CLOSE
	time.Sleep(100 * time.Millisecond)

	if !session.IsClosed() {
		t.Error("session should be closed after CLOSE packet")
	}
}

// TestClientCompat_EIO4_VersionParameter verifies that the server accepts
// the EIO=4 query parameter, which socket.io-client v4.8.3 always sends.
func TestClientCompat_EIO4_VersionParameter(t *testing.T) {
	srv := NewServer(DefaultConfig())

	// With EIO=4 (correct)
	req := httptest.NewRequest("GET", "/socket.io/?EIO=4&transport=polling", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("EIO=4 request should succeed, got status %d", w.Result().StatusCode)
	}
}

// TestClientCompat_CORSPreflightForPolling verifies that OPTIONS preflight
// requests for polling transport are handled correctly, as browsers send
// these before the actual socket.io-client requests.
func TestClientCompat_CORSPreflightForPolling(t *testing.T) {
	srv := NewServer(DefaultConfig())

	req := httptest.NewRequest("OPTIONS", "/socket.io/?EIO=4&transport=polling", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("OPTIONS status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("ACAO = %q, want \"http://localhost:5173\"",
			resp.Header.Get("Access-Control-Allow-Origin"))
	}
	if resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("missing ACAC: true")
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Methods"), "GET") {
		t.Error("ACAM must include GET")
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Methods"), "POST") {
		t.Error("ACAM must include POST")
	}
}
