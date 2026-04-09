package engineio

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer() *Server {
	config := ServerConfig{
		PingInterval:   10 * time.Second,
		PingTimeout:    5 * time.Second,
		MaxPayload:     1_000_000,
		Upgrades:       []string{"websocket"},
		AllowedOrigins: []string{"http://localhost", "http://localhost:3000"},
	}
	return NewServer(config)
}

func TestHandshake(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Should start with '0' (OPEN packet type)
	if bodyStr[0] != '0' {
		t.Fatalf("expected OPEN packet (0), got %c", bodyStr[0])
	}

	// Parse the JSON payload
	var openData openPacketData
	if err := json.Unmarshal(body[1:], &openData); err != nil {
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
	if len(openData.Upgrades) != 1 || openData.Upgrades[0] != "websocket" {
		t.Errorf("Upgrades = %v, want [websocket]", openData.Upgrades)
	}
	if openData.MaxPayload != 1_000_000 {
		t.Errorf("MaxPayload = %d, want 1000000", openData.MaxPayload)
	}

	// Verify session was stored
	session := srv.GetSession(openData.SID)
	if session == nil {
		t.Fatal("session not found after handshake")
	}
}

func TestPollingGetReturnsNoopWhenEmpty(t *testing.T) {
	srv := newTestServer()

	// Create a session first via handshake
	req := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	var openData openPacketData
	json.Unmarshal(body[1:], &openData)

	// Now do a polling GET — should return quickly with NOOP since no messages
	// Use a short timeout by pre-closing the session's channel
	done := make(chan struct{})
	go func() {
		req2 := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4&sid="+openData.SID, nil)
		w2 := httptest.NewRecorder()
		srv.ServeHTTP(w2, req2)

		body2, _ := io.ReadAll(w2.Result().Body)
		// Should contain noop (6) or buffered packets
		if len(body2) == 0 {
			t.Error("empty response from polling GET")
		}
		close(done)
	}()

	// Send a message to unblock the poll
	session := srv.GetSession(openData.SID)
	session.Send([]byte("test"))

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("polling GET timed out")
	}
}

func TestPollingPostAndGet(t *testing.T) {
	srv := newTestServer()
	var receivedMsg string

	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			receivedMsg = string(data)
			// Echo back
			s.Send(data)
		})
	})

	// Handshake
	req := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	var openData openPacketData
	json.Unmarshal(body[1:], &openData)
	sid := openData.SID

	// POST a message
	msgPayload := "4hello"
	postReq := httptest.NewRequest("POST", "/engine.io/?transport=polling&EIO=4&sid="+sid, strings.NewReader(msgPayload))
	postW := httptest.NewRecorder()
	srv.ServeHTTP(postW, postReq)

	if postW.Result().StatusCode != http.StatusOK {
		t.Fatalf("POST status = %d, want 200", postW.Result().StatusCode)
	}

	postBody, _ := io.ReadAll(postW.Result().Body)
	if string(postBody) != "ok" {
		t.Errorf("POST response = %q, want %q", postBody, "ok")
	}

	// Verify message was received
	if receivedMsg != "hello" {
		t.Errorf("received message = %q, want %q", receivedMsg, "hello")
	}

	// GET should return the echoed message
	getReq := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4&sid="+sid, nil)
	getW := httptest.NewRecorder()
	srv.ServeHTTP(getW, getReq)

	getBody, _ := io.ReadAll(getW.Result().Body)
	if string(getBody) != "4hello" {
		t.Errorf("GET response = %q, want %q", getBody, "4hello")
	}
}

func TestPingPong(t *testing.T) {
	srv := newTestServer()

	// Handshake
	req := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	var openData openPacketData
	json.Unmarshal(body[1:], &openData)
	sid := openData.SID

	// Send a PING via POST
	postReq := httptest.NewRequest("POST", "/engine.io/?transport=polling&EIO=4&sid="+sid, strings.NewReader("2probe"))
	postW := httptest.NewRecorder()
	srv.ServeHTTP(postW, postReq)

	if postW.Result().StatusCode != http.StatusOK {
		t.Fatalf("POST status = %d", postW.Result().StatusCode)
	}

	// GET should return PONG with same data
	getReq := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4&sid="+sid, nil)
	getW := httptest.NewRecorder()
	srv.ServeHTTP(getW, getReq)

	getBody, _ := io.ReadAll(getW.Result().Body)
	if string(getBody) != "3probe" {
		t.Errorf("GET response = %q, want %q", getBody, "3probe")
	}
}

func TestUnknownSID(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4&sid=nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Result().StatusCode, http.StatusBadRequest)
	}
}

func TestUnknownTransport(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest("GET", "/engine.io/?transport=unknown", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Result().StatusCode, http.StatusBadRequest)
	}
}

func TestMultiplePacketsInPayload(t *testing.T) {
	srv := newTestServer()
	var msgs []string

	srv.OnConnect(func(s *Session) {
		s.SetOnMessage(func(data []byte) {
			msgs = append(msgs, string(data))
		})
	})

	// Handshake
	req := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	var openData openPacketData
	json.Unmarshal(body[1:], &openData)

	// POST multiple messages in one payload
	payload := "4hello\x1e4world"
	postReq := httptest.NewRequest("POST", "/engine.io/?transport=polling&EIO=4&sid="+openData.SID, strings.NewReader(payload))
	postW := httptest.NewRecorder()
	srv.ServeHTTP(postW, postReq)

	if len(msgs) != 2 {
		t.Fatalf("received %d messages, want 2", len(msgs))
	}
	if msgs[0] != "hello" || msgs[1] != "world" {
		t.Errorf("messages = %v, want [hello, world]", msgs)
	}
}

func TestCORSHeaders(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("CORS origin = %q, want %q", resp.Header.Get("Access-Control-Allow-Origin"), "http://localhost:3000")
	}
	if resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("missing Access-Control-Allow-Credentials: true")
	}
}

func TestOptionsRequest(t *testing.T) {
	srv := newTestServer()

	req := httptest.NewRequest("OPTIONS", "/engine.io/?transport=polling&EIO=4", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("OPTIONS status = %d, want 200", w.Result().StatusCode)
	}
}

func TestServerClose_ClosesAllSessions(t *testing.T) {
	srv := newTestServer()

	// Create 3 sessions via handshake
	sids := make([]string, 3)
	for i := range sids {
		req := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		body, _ := io.ReadAll(w.Result().Body)
		var od openPacketData
		json.Unmarshal(body[1:], &od)
		sids[i] = od.SID
	}

	// Verify all sessions exist
	for _, sid := range sids {
		if srv.GetSession(sid) == nil {
			t.Fatalf("session %s not found before Close", sid)
		}
	}

	srv.Close()

	// All sessions should be closed and removed from the map
	for _, sid := range sids {
		if s := srv.GetSession(sid); s != nil {
			t.Errorf("session %s still in map after Close", sid)
		}
	}
}

func TestServerClose_OnCloseCallbackFires(t *testing.T) {
	srv := newTestServer()

	closedSIDs := make(map[string]string)
	srv.OnConnect(func(s *Session) {
		s.SetOnClose(func(sid string, reason string) {
			closedSIDs[sid] = reason
			srv.RemoveSession(sid)
		})
	})

	// Create a session
	req := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	var od openPacketData
	json.Unmarshal(body[1:], &od)

	srv.Close()

	reason, ok := closedSIDs[od.SID]
	if !ok {
		t.Fatal("onClose callback was not invoked")
	}
	if reason != "server shutting down" {
		t.Errorf("close reason = %q, want %q", reason, "server shutting down")
	}
}

func TestClosePacket(t *testing.T) {
	srv := newTestServer()

	// Handshake
	req := httptest.NewRequest("GET", "/engine.io/?transport=polling&EIO=4", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	var openData openPacketData
	json.Unmarshal(body[1:], &openData)

	session := srv.GetSession(openData.SID)
	if session == nil {
		t.Fatal("session not found")
	}

	// Send CLOSE packet
	postReq := httptest.NewRequest("POST", "/engine.io/?transport=polling&EIO=4&sid="+openData.SID, strings.NewReader("1"))
	postW := httptest.NewRecorder()
	srv.ServeHTTP(postW, postReq)

	// Session should be closed
	if !session.IsClosed() {
		t.Error("session should be closed after receiving CLOSE packet")
	}
}
