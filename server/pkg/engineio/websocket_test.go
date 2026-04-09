package engineio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialWS upgrades to WebSocket on the test server at the given path.
func dialWS(t *testing.T, server *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + path
	header := http.Header{}
	header.Set("Origin", "http://localhost")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	return conn
}

// readTextMessage reads a text message from the WebSocket.
func readTextMessage(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("WebSocket read failed: %v", err)
	}
	return string(msg)
}

func TestWSHandshake(t *testing.T) {
	srv := newTestServer()
	server := httptest.NewServer(srv)
	defer server.Close()

	conn := dialWS(t, server, "/engine.io/?transport=websocket&EIO=4")
	defer conn.Close()

	// Should receive OPEN packet
	msg := readTextMessage(t, conn)
	if msg[0] != '0' {
		t.Fatalf("expected OPEN packet, got %q", msg)
	}

	var openData openPacketData
	if err := json.Unmarshal([]byte(msg[1:]), &openData); err != nil {
		t.Fatalf("failed to parse open packet: %v", err)
	}

	if openData.SID == "" {
		t.Error("SID should not be empty")
	}
	if openData.PingInterval != 10000 {
		t.Errorf("PingInterval = %d, want 10000", openData.PingInterval)
	}
	if openData.PingTimeout != 5000 {
		t.Errorf("PingTimeout = %d, want 5000", openData.PingTimeout)
	}
	// WebSocket connections should not offer further upgrades
	if len(openData.Upgrades) != 0 {
		t.Errorf("Upgrades = %v, want []", openData.Upgrades)
	}
	if openData.MaxPayload != 1_000_000 {
		t.Errorf("MaxPayload = %d, want 1000000", openData.MaxPayload)
	}

	// Session should exist with websocket transport
	session := srv.GetSession(openData.SID)
	if session == nil {
		t.Fatal("session not found after WS handshake")
	}
	if session.Transport != "websocket" {
		t.Errorf("transport = %q, want %q", session.Transport, "websocket")
	}
}

func TestWSMessageEcho(t *testing.T) {
	srv := newTestServer()

	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			// Echo back
			s.Send(data)
		})
	})

	server := httptest.NewServer(srv)
	defer server.Close()

	conn := dialWS(t, server, "/engine.io/?transport=websocket&EIO=4")
	defer conn.Close()

	// Read OPEN packet
	readTextMessage(t, conn)

	// Send a message
	err := conn.WriteMessage(websocket.TextMessage, []byte("4hello"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Should get echo back
	msg := readTextMessage(t, conn)
	if msg != "4hello" {
		t.Errorf("echo = %q, want %q", msg, "4hello")
	}
}

func TestWSPingPong(t *testing.T) {
	srv := newTestServer()
	server := httptest.NewServer(srv)
	defer server.Close()

	conn := dialWS(t, server, "/engine.io/?transport=websocket&EIO=4")
	defer conn.Close()

	// Read OPEN
	readTextMessage(t, conn)

	// Client sends PING
	err := conn.WriteMessage(websocket.TextMessage, []byte("2"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Should receive PONG
	msg := readTextMessage(t, conn)
	if msg != "3" {
		t.Errorf("pong = %q, want %q", msg, "3")
	}
}

func TestWSPingWithData(t *testing.T) {
	srv := newTestServer()
	server := httptest.NewServer(srv)
	defer server.Close()

	conn := dialWS(t, server, "/engine.io/?transport=websocket&EIO=4")
	defer conn.Close()

	// Read OPEN
	readTextMessage(t, conn)

	// Client sends PING with data
	err := conn.WriteMessage(websocket.TextMessage, []byte("2probe"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Should receive PONG with same data
	msg := readTextMessage(t, conn)
	if msg != "3probe" {
		t.Errorf("pong = %q, want %q", msg, "3probe")
	}
}

func TestWSClose(t *testing.T) {
	srv := newTestServer()
	server := httptest.NewServer(srv)
	defer server.Close()

	conn := dialWS(t, server, "/engine.io/?transport=websocket&EIO=4")
	defer conn.Close()

	// Read OPEN
	msg := readTextMessage(t, conn)
	var openData openPacketData
	json.Unmarshal([]byte(msg[1:]), &openData)

	session := srv.GetSession(openData.SID)
	if session == nil {
		t.Fatal("session not found")
	}

	// Send CLOSE packet
	err := conn.WriteMessage(websocket.TextMessage, []byte("1"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Wait a bit for close to propagate
	time.Sleep(100 * time.Millisecond)

	if !session.IsClosed() {
		t.Error("session should be closed after receiving CLOSE packet")
	}
}

func TestWSUpgradeFromPolling(t *testing.T) {
	srv := newTestServer()

	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			s.Send(data)
		})
	})

	server := httptest.NewServer(srv)
	defer server.Close()

	// Step 1: Handshake via polling
	resp, err := server.Client().Get(server.URL + "/engine.io/?transport=polling&EIO=4")
	if err != nil {
		t.Fatalf("polling handshake failed: %v", err)
	}
	defer resp.Body.Close()

	var buf [4096]byte
	n, _ := resp.Body.Read(buf[:])
	body := string(buf[:n])

	if body[0] != '0' {
		t.Fatalf("expected OPEN packet, got %q", body)
	}

	var openData openPacketData
	if err := json.Unmarshal([]byte(body[1:]), &openData); err != nil {
		t.Fatalf("failed to parse open packet: %v", err)
	}
	sid := openData.SID

	session := srv.GetSession(sid)
	if session == nil {
		t.Fatal("session not found")
	}
	if session.Transport != "polling" {
		t.Errorf("initial transport = %q, want %q", session.Transport, "polling")
	}

	// Step 2: Open WebSocket connection for upgrade
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/engine.io/?transport=websocket&EIO=4&sid=" + sid
	wsHeader := http.Header{}
	wsHeader.Set("Origin", "http://localhost")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, wsHeader)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	// Step 3: Send probe PING
	err = conn.WriteMessage(websocket.TextMessage, []byte("2probe"))
	if err != nil {
		t.Fatalf("write probe ping failed: %v", err)
	}

	// Step 4: Read probe PONG
	msg := readTextMessage(t, conn)
	if msg != "3probe" {
		t.Fatalf("expected probe pong, got %q", msg)
	}

	// Step 5: Send UPGRADE packet
	err = conn.WriteMessage(websocket.TextMessage, []byte("5"))
	if err != nil {
		t.Fatalf("write upgrade failed: %v", err)
	}

	// Wait for upgrade to complete
	time.Sleep(100 * time.Millisecond)

	// Session should now be on websocket transport
	if session.Transport != "websocket" {
		t.Errorf("transport after upgrade = %q, want %q", session.Transport, "websocket")
	}

	// Step 6: Verify messages work over WebSocket
	err = conn.WriteMessage(websocket.TextMessage, []byte("4upgraded!"))
	if err != nil {
		t.Fatalf("write message failed: %v", err)
	}

	echoMsg := readTextMessage(t, conn)
	if echoMsg != "4upgraded!" {
		t.Errorf("echo = %q, want %q", echoMsg, "4upgraded!")
	}
}

func TestWSServerPing(t *testing.T) {
	// Use a short ping interval to test server-initiated pings
	config := ServerConfig{
		PingInterval:   200 * time.Millisecond,
		PingTimeout:    100 * time.Millisecond,
		MaxPayload:     1_000_000,
		Upgrades:       []string{"websocket"},
		AllowedOrigins: []string{"http://localhost"},
	}
	srv := NewServer(config)
	server := httptest.NewServer(srv)
	defer server.Close()

	conn := dialWS(t, server, "/engine.io/?transport=websocket&EIO=4")
	defer conn.Close()

	// Read OPEN
	readTextMessage(t, conn)

	// Wait for server ping
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read server ping: %v", err)
	}

	if string(msg) != "2" {
		t.Errorf("expected ping packet '2', got %q", string(msg))
	}

	// Respond with pong
	err = conn.WriteMessage(websocket.TextMessage, []byte("3"))
	if err != nil {
		t.Fatalf("pong write failed: %v", err)
	}
}

func TestWSMultipleMessages(t *testing.T) {
	srv := newTestServer()

	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			s.Send(data)
		})
	})

	server := httptest.NewServer(srv)
	defer server.Close()

	conn := dialWS(t, server, "/engine.io/?transport=websocket&EIO=4")
	defer conn.Close()

	// Read OPEN
	readTextMessage(t, conn)

	// Send multiple messages
	messages := []string{"hello", "world", "foo"}
	for _, m := range messages {
		err := conn.WriteMessage(websocket.TextMessage, []byte("4"+m))
		if err != nil {
			t.Fatalf("write %q failed: %v", m, err)
		}
	}

	// Read back echoes
	for _, m := range messages {
		msg := readTextMessage(t, conn)
		expected := "4" + m
		if msg != expected {
			t.Errorf("echo = %q, want %q", msg, expected)
		}
	}
}

func TestWSInvalidSID(t *testing.T) {
	srv := newTestServer()
	server := httptest.NewServer(srv)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/engine.io/?transport=websocket&EIO=4&sid=nonexistent"
	wsHeader := http.Header{}
	wsHeader.Set("Origin", "http://localhost")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, wsHeader)
	if err != nil {
		// Connection may be refused; that's acceptable
		return
	}
	defer conn.Close()

	// Should receive error message or connection close
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err == nil {
		// Verify it's an error message
		if !strings.Contains(string(msg), "Session ID unknown") {
			t.Errorf("expected error message, got %q", string(msg))
		}
	}
	// Connection should close shortly after
}

func TestWSUpgradeReleasesPollingWithNoop(t *testing.T) {
	srv := newTestServer()

	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			s.Send(data)
		})
	})

	server := httptest.NewServer(srv)
	defer server.Close()

	// Step 1: Handshake via polling
	resp, err := server.Client().Get(server.URL + "/engine.io/?transport=polling&EIO=4")
	if err != nil {
		t.Fatalf("polling handshake failed: %v", err)
	}
	var buf [4096]byte
	n, _ := resp.Body.Read(buf[:])
	resp.Body.Close()

	var openData openPacketData
	json.Unmarshal(buf[1:n], &openData)
	sid := openData.SID

	// Step 2: Start a concurrent polling GET (simulates the client's pending poll)
	pollDone := make(chan string, 1)
	go func() {
		r, err := server.Client().Get(server.URL + "/engine.io/?transport=polling&EIO=4&sid=" + sid)
		if err != nil {
			pollDone <- "error:" + err.Error()
			return
		}
		var b [4096]byte
		n, _ := r.Body.Read(b[:])
		r.Body.Close()
		pollDone <- string(b[:n])
	}()

	// Give the poll request time to block in Drain
	time.Sleep(50 * time.Millisecond)

	// Step 3: Open WebSocket and perform probe
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/engine.io/?transport=websocket&EIO=4&sid=" + sid
	wsHeader := http.Header{}
	wsHeader.Set("Origin", "http://localhost")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, wsHeader)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	// Send probe ping
	conn.WriteMessage(websocket.TextMessage, []byte("2probe"))

	// Read probe pong
	msg := readTextMessage(t, conn)
	if msg != "3probe" {
		t.Fatalf("expected probe pong, got %q", msg)
	}

	// Step 4: The pending poll should have been released with a NOOP
	select {
	case pollResult := <-pollDone:
		// Should contain NOOP (6)
		if !strings.Contains(pollResult, "6") {
			t.Errorf("expected NOOP in poll response, got %q", pollResult)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pending poll was not released during upgrade probe")
	}

	// Step 5: Complete the upgrade
	conn.WriteMessage(websocket.TextMessage, []byte("5"))
	time.Sleep(50 * time.Millisecond)

	session := srv.GetSession(sid)
	if session.Transport != "websocket" {
		t.Errorf("transport = %q, want %q", session.Transport, "websocket")
	}

	// Verify messaging works after upgrade
	conn.WriteMessage(websocket.TextMessage, []byte("4test"))
	echo := readTextMessage(t, conn)
	if echo != "4test" {
		t.Errorf("echo = %q, want %q", echo, "4test")
	}
}

func TestWSUpgradeProbeTimeout(t *testing.T) {
	config := ServerConfig{
		PingInterval:   10 * time.Second,
		PingTimeout:    500 * time.Millisecond, // short timeout for test
		MaxPayload:     1_000_000,
		Upgrades:       []string{"websocket"},
		AllowedOrigins: []string{"http://localhost"},
	}
	srv := NewServer(config)
	server := httptest.NewServer(srv)
	defer server.Close()

	// Handshake via polling
	resp, err := server.Client().Get(server.URL + "/engine.io/?transport=polling&EIO=4")
	if err != nil {
		t.Fatalf("polling handshake failed: %v", err)
	}
	var buf [4096]byte
	n, _ := resp.Body.Read(buf[:])
	resp.Body.Close()

	var openData openPacketData
	json.Unmarshal(buf[1:n], &openData)
	sid := openData.SID

	// Open WebSocket but don't send probe — should timeout
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/engine.io/?transport=websocket&EIO=4&sid=" + sid
	wsHeader := http.Header{}
	wsHeader.Set("Origin", "http://localhost")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, wsHeader)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	// Wait for the server to close the WS due to probe timeout
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, readErr := conn.ReadMessage()
	if readErr == nil {
		t.Error("expected read error after probe timeout")
	}

	// Original session should still be alive on polling
	session := srv.GetSession(sid)
	if session == nil {
		t.Fatal("session should still exist after failed upgrade")
	}
	if session.Transport != "polling" {
		t.Errorf("transport = %q, want %q (should remain polling after failed upgrade)", session.Transport, "polling")
	}
}

func TestWSUpgradeInvalidProbe(t *testing.T) {
	srv := newTestServer()
	server := httptest.NewServer(srv)
	defer server.Close()

	// Handshake via polling
	resp, err := server.Client().Get(server.URL + "/engine.io/?transport=polling&EIO=4")
	if err != nil {
		t.Fatalf("polling handshake failed: %v", err)
	}
	var buf [4096]byte
	n, _ := resp.Body.Read(buf[:])
	resp.Body.Close()

	var openData openPacketData
	json.Unmarshal(buf[1:n], &openData)
	sid := openData.SID

	// Open WebSocket and send an invalid probe (wrong data)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/engine.io/?transport=websocket&EIO=4&sid=" + sid
	wsHeader := http.Header{}
	wsHeader.Set("Origin", "http://localhost")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, wsHeader)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	// Send PING without "probe" data — should be rejected
	conn.WriteMessage(websocket.TextMessage, []byte("2notprobe"))

	// Server should close the WebSocket
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, readErr := conn.ReadMessage()
	if readErr == nil {
		t.Error("expected connection close after invalid probe")
	}

	// Session should remain on polling
	session := srv.GetSession(sid)
	if session == nil {
		t.Fatal("session should still exist")
	}
	if session.Transport != "polling" {
		t.Errorf("transport = %q, want %q", session.Transport, "polling")
	}
}

func TestWSReadLimitRejectsOversizedFrame(t *testing.T) {
	config := ServerConfig{
		PingInterval:   10 * time.Second,
		PingTimeout:    5 * time.Second,
		MaxPayload:     1024, // 1KB limit for testing
		Upgrades:       []string{"websocket"},
		AllowedOrigins: []string{"http://localhost"},
	}
	srv := NewServer(config)
	server := httptest.NewServer(srv)
	defer server.Close()

	conn := dialWS(t, server, "/engine.io/?transport=websocket&EIO=4")
	defer conn.Close()

	// Read OPEN packet
	readTextMessage(t, conn)

	// Send a message larger than MaxPayload (2KB > 1KB limit)
	oversized := make([]byte, 2048)
	for i := range oversized {
		oversized[i] = 'A'
	}
	payload := append([]byte("4"), oversized...)
	err := conn.WriteMessage(websocket.TextMessage, payload)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// The server should close the connection due to read limit exceeded
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, readErr := conn.ReadMessage()
	if readErr == nil {
		t.Error("expected connection close after oversized frame")
	}
}

func TestWSReadLimitAllowsNormalFrame(t *testing.T) {
	config := ServerConfig{
		PingInterval:   10 * time.Second,
		PingTimeout:    5 * time.Second,
		MaxPayload:     1024, // 1KB limit for testing
		Upgrades:       []string{"websocket"},
		AllowedOrigins: []string{"http://localhost"},
	}
	srv := NewServer(config)

	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			s.Send(data)
		})
	})

	server := httptest.NewServer(srv)
	defer server.Close()

	conn := dialWS(t, server, "/engine.io/?transport=websocket&EIO=4")
	defer conn.Close()

	// Read OPEN packet
	readTextMessage(t, conn)

	// Send a message under MaxPayload (512 bytes < 1KB limit)
	small := make([]byte, 512)
	for i := range small {
		small[i] = 'B'
	}
	payload := append([]byte("4"), small...)
	err := conn.WriteMessage(websocket.TextMessage, payload)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Should receive echo back successfully
	msg := readTextMessage(t, conn)
	if msg[0] != '4' {
		t.Errorf("expected message packet, got %q", msg[:1])
	}
}
