package socketio

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"testing"
)

func TestNewNamespace_DefaultPath(t *testing.T) {
	ns := NewNamespace("")
	if ns.Path() != "/" {
		t.Errorf("expected path '/', got %q", ns.Path())
	}
}

func TestNewNamespace_CustomPath(t *testing.T) {
	ns := NewNamespace("/chat")
	if ns.Path() != "/chat" {
		t.Errorf("expected path '/chat', got %q", ns.Path())
	}
}

func TestNamespace_OnAndHandleEvent(t *testing.T) {
	ns := NewNamespace("/")
	called := false
	var receivedSID string
	var receivedArgs []json.RawMessage

	ns.On("hello", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		called = true
		receivedSID = sid
		receivedArgs = args
		return []interface{}{"world"}, nil
	})

	args := []json.RawMessage{json.RawMessage(`"test-data"`)}
	result, err := ns.HandleEvent("socket-1", "hello", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
	if receivedSID != "socket-1" {
		t.Errorf("expected sid 'socket-1', got %q", receivedSID)
	}
	if len(receivedArgs) != 1 || string(receivedArgs[0]) != `"test-data"` {
		t.Errorf("unexpected args: %v", receivedArgs)
	}
	if len(result) != 1 || result[0] != "world" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestNamespace_HandleEvent_NoHandler(t *testing.T) {
	ns := NewNamespace("/")
	_, err := ns.HandleEvent("sid", "unknown", nil)
	if err == nil {
		t.Fatal("expected error for unknown event")
	}
}

func TestNamespace_HasEvent(t *testing.T) {
	ns := NewNamespace("/")
	ns.On("ping", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, nil
	})
	if !ns.HasEvent("ping") {
		t.Error("expected HasEvent('ping') to be true")
	}
	if ns.HasEvent("pong") {
		t.Error("expected HasEvent('pong') to be false")
	}
}

func TestNamespace_Events(t *testing.T) {
	ns := NewNamespace("/")
	ns.On("a", func(sid string, args ...json.RawMessage) ([]interface{}, error) { return nil, nil })
	ns.On("b", func(sid string, args ...json.RawMessage) ([]interface{}, error) { return nil, nil })

	events := ns.Events()
	sort.Strings(events)
	if len(events) != 2 || events[0] != "a" || events[1] != "b" {
		t.Errorf("unexpected events: %v", events)
	}
}

func TestNamespace_OnConnect(t *testing.T) {
	ns := NewNamespace("/")
	called := false
	ns.OnConnect(func(sid string, auth json.RawMessage) error {
		called = true
		return nil
	})
	err := ns.HandleConnect("sid-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("connect handler was not called")
	}
}

func TestNamespace_OnConnect_NoHandler(t *testing.T) {
	ns := NewNamespace("/")
	err := ns.HandleConnect("sid-1", nil)
	if err != nil {
		t.Fatalf("expected nil error when no handler, got: %v", err)
	}
}

func TestNamespace_OnDisconnect(t *testing.T) {
	ns := NewNamespace("/")
	var receivedReason string
	ns.OnDisconnect(func(sid string, reason string) {
		receivedReason = reason
	})
	ns.HandleDisconnect("sid-1", "transport close")
	if receivedReason != "transport close" {
		t.Errorf("expected reason 'transport close', got %q", receivedReason)
	}
}

func TestNamespace_HasOwnRoomManager(t *testing.T) {
	ns := NewNamespace("/chat")
	if ns.Rooms == nil {
		t.Fatal("namespace should have its own RoomManager")
	}
}

func TestNamespace_RoomIsolation(t *testing.T) {
	ns1 := NewNamespace("/")
	ns2 := NewNamespace("/chat")

	// Same room name in different namespaces should be isolated
	ns1.Rooms.Join("sid-1", "room-a")
	ns2.Rooms.Join("sid-2", "room-a")

	members1 := ns1.Rooms.Members("room-a")
	members2 := ns2.Rooms.Members("room-a")

	if len(members1) != 1 || members1[0] != "sid-1" {
		t.Errorf("ns1 room-a should only contain sid-1, got %v", members1)
	}
	if len(members2) != 1 || members2[0] != "sid-2" {
		t.Errorf("ns2 room-a should only contain sid-2, got %v", members2)
	}
}

func TestNamespace_DisconnectCleansUpRooms(t *testing.T) {
	ns := NewNamespace("/")
	ns.HandleConnect("sid-1", nil)
	ns.Rooms.Join("sid-1", "room-a")
	ns.Rooms.Join("sid-1", "room-b")

	// Verify rooms before disconnect
	if !ns.Rooms.InRoom("sid-1", "room-a") || !ns.Rooms.InRoom("sid-1", "room-b") {
		t.Fatal("sid-1 should be in both rooms before disconnect")
	}

	// Disconnect should clean up all room memberships
	ns.HandleDisconnect("sid-1", "transport close")

	if ns.Rooms.InRoom("sid-1", "room-a") {
		t.Error("sid-1 should not be in room-a after disconnect")
	}
	if ns.Rooms.InRoom("sid-1", "room-b") {
		t.Error("sid-1 should not be in room-b after disconnect")
	}
	if ns.HasSocket("sid-1") {
		t.Error("sid-1 should not be in socket set after disconnect")
	}
}

func TestNamespace_DisconnectCallsHandlerAfterRoomCleanup(t *testing.T) {
	ns := NewNamespace("/")
	ns.HandleConnect("sid-1", nil)
	ns.Rooms.Join("sid-1", "room-a")

	var roomsAtDisconnect []string
	ns.OnDisconnect(func(sid string, reason string) {
		// At the time the handler is called, rooms should already be cleaned up
		roomsAtDisconnect = ns.Rooms.Rooms(sid)
	})

	ns.HandleDisconnect("sid-1", "test")

	if roomsAtDisconnect != nil {
		t.Errorf("rooms should be cleaned up before disconnect handler, got %v", roomsAtDisconnect)
	}
}

func TestNamespace_IndependentConnectDisconnect(t *testing.T) {
	ns1 := NewNamespace("/")
	ns2 := NewNamespace("/chat")

	var ns1ConnectCalled, ns2ConnectCalled bool
	var ns1DisconnectCalled, ns2DisconnectCalled bool

	ns1.OnConnect(func(sid string, auth json.RawMessage) error {
		ns1ConnectCalled = true
		return nil
	})
	ns2.OnConnect(func(sid string, auth json.RawMessage) error {
		ns2ConnectCalled = true
		return nil
	})
	ns1.OnDisconnect(func(sid string, reason string) {
		ns1DisconnectCalled = true
	})
	ns2.OnDisconnect(func(sid string, reason string) {
		ns2DisconnectCalled = true
	})

	// Connect to ns1 only
	ns1.HandleConnect("sid-1", nil)

	if !ns1ConnectCalled {
		t.Error("ns1 connect handler should have been called")
	}
	if ns2ConnectCalled {
		t.Error("ns2 connect handler should NOT have been called")
	}

	// Disconnect from ns1 only
	ns1.HandleDisconnect("sid-1", "test")

	if !ns1DisconnectCalled {
		t.Error("ns1 disconnect handler should have been called")
	}
	if ns2DisconnectCalled {
		t.Error("ns2 disconnect handler should NOT have been called")
	}
}

func TestNamespace_Use_MiddlewareRunsBeforeHandler(t *testing.T) {
	ns := NewNamespace("/")
	var order []string

	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw1-before")
		result, err := next()
		order = append(order, "mw1-after")
		return result, err
	})

	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw2-before")
		result, err := next()
		order = append(order, "mw2-after")
		return result, err
	})

	ns.On("ping", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		order = append(order, "handler")
		return []interface{}{"pong"}, nil
	})

	result, err := ns.HandleEvent("sid-1", "ping", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "pong" {
		t.Fatalf("unexpected result: %v", result)
	}

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}

func TestNamespace_Use_MiddlewareCanShortCircuit(t *testing.T) {
	ns := NewNamespace("/")
	handlerCalled := false

	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		// Short-circuit: don't call next
		return []interface{}{"blocked"}, nil
	})

	ns.On("ping", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		handlerCalled = true
		return []interface{}{"pong"}, nil
	})

	result, err := ns.HandleEvent("sid-1", "ping", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handlerCalled {
		t.Fatal("handler should not have been called")
	}
	if len(result) != 1 || result[0] != "blocked" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestNamespace_Use_MiddlewareReceivesSidAndEvent(t *testing.T) {
	ns := NewNamespace("/")
	var capturedSID, capturedEvent string

	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		capturedSID = sid
		capturedEvent = event
		return next()
	})

	ns.On("chat:message", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, nil
	})

	args := []json.RawMessage{json.RawMessage(`"hello"`)}
	_, err := ns.HandleEvent("socket-99", "chat:message", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedSID != "socket-99" {
		t.Fatalf("expected sid=%q, got %q", "socket-99", capturedSID)
	}
	if capturedEvent != "chat:message" {
		t.Fatalf("expected event=%q, got %q", "chat:message", capturedEvent)
	}
}

func TestNamespace_Use_NoMiddleware_DirectHandler(t *testing.T) {
	ns := NewNamespace("/")
	called := false

	ns.On("test", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		called = true
		return []interface{}{"ok"}, nil
	})

	result, err := ns.HandleEvent("sid-1", "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
	if len(result) != 1 || result[0] != "ok" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestNamespace_Use_MiddlewareNotCalledForUnknownEvent(t *testing.T) {
	ns := NewNamespace("/")
	mwCalled := false

	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		mwCalled = true
		return next()
	})

	_, err := ns.HandleEvent("sid-1", "unknown", nil)
	if err == nil {
		t.Fatal("expected error for unknown event")
	}
	if mwCalled {
		t.Fatal("middleware should not be called for unknown events")
	}
}

// --- Error handling and chain interruption tests at Namespace level ---

func TestNamespace_Use_MiddlewareError_StopsChainAndReturnsError(t *testing.T) {
	ns := NewNamespace("/")
	errForbidden := errors.New("forbidden")

	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return nil, errForbidden
	})

	handlerCalled := false
	ns.On("ping", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		handlerCalled = true
		return nil, nil
	})

	_, err := ns.HandleEvent("sid-1", "ping", nil)
	if !errors.Is(err, errForbidden) {
		t.Fatalf("expected errForbidden, got %v", err)
	}
	if handlerCalled {
		t.Fatal("handler should not have been called when middleware returns error")
	}
}

func TestNamespace_Use_MiddlewareErrorInMiddle_StopsRemainingChain(t *testing.T) {
	ns := NewNamespace("/")
	errAuth := errors.New("unauthorized")
	var order []string

	// mw1 passes through
	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw1-before")
		result, err := next()
		order = append(order, "mw1-after")
		return result, err
	})

	// mw2 returns error
	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw2-error")
		return nil, errAuth
	})

	// mw3 should not be reached
	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw3")
		return next()
	})

	ns.On("action", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		order = append(order, "handler")
		return nil, nil
	})

	_, err := ns.HandleEvent("sid-1", "action", nil)
	if !errors.Is(err, errAuth) {
		t.Fatalf("expected errAuth, got %v", err)
	}

	expected := []string{"mw1-before", "mw2-error", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}

func TestNamespace_Use_MiddlewareNoNext_HandlerNotCalled(t *testing.T) {
	ns := NewNamespace("/")

	// Middleware does not call next — short-circuit
	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return []interface{}{"intercepted"}, nil
	})

	handlerCalled := false
	ns.On("action", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		handlerCalled = true
		return []interface{}{"original"}, nil
	})

	result, err := ns.HandleEvent("sid-1", "action", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handlerCalled {
		t.Fatal("handler should not have been called when middleware skips next()")
	}
	if len(result) != 1 || result[0] != "intercepted" {
		t.Fatalf("expected intercepted result, got %v", result)
	}
}

func TestNamespace_Use_HandlerError_PropagatesToCaller(t *testing.T) {
	ns := NewNamespace("/")
	errHandler := errors.New("handler failed")

	var mwReceivedErr error
	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		result, err := next()
		mwReceivedErr = err
		return result, err
	})

	ns.On("fail", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, errHandler
	})

	_, err := ns.HandleEvent("sid-1", "fail", nil)
	if !errors.Is(err, errHandler) {
		t.Fatalf("expected errHandler, got %v", err)
	}
	if !errors.Is(mwReceivedErr, errHandler) {
		t.Fatalf("middleware should have received errHandler, got %v", mwReceivedErr)
	}
}

func TestNamespace_Use_ConditionalMiddleware_EventFiltering(t *testing.T) {
	ns := NewNamespace("/")
	errBlocked := errors.New("event blocked")

	// Middleware that only blocks specific events
	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		if event == "restricted" {
			return nil, errBlocked
		}
		return next()
	})

	ns.On("restricted", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{"should-not-reach"}, nil
	})
	ns.On("allowed", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return []interface{}{"allowed-result"}, nil
	})

	// "restricted" event should be blocked
	_, err := ns.HandleEvent("sid-1", "restricted", nil)
	if !errors.Is(err, errBlocked) {
		t.Fatalf("expected errBlocked for restricted event, got %v", err)
	}

	// "allowed" event should pass through
	result, err := ns.HandleEvent("sid-1", "allowed", nil)
	if err != nil {
		t.Fatalf("unexpected error for allowed event: %v", err)
	}
	if len(result) != 1 || result[0] != "allowed-result" {
		t.Fatalf("expected allowed-result, got %v", result)
	}
}

func TestNamespace_Use_MiddlewareErrorWithResponse(t *testing.T) {
	ns := NewNamespace("/")

	// Middleware returns both error and response data (for ack error responses)
	ns.Use(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return []interface{}{map[string]interface{}{"error": "rate_limited", "retry_after": 60}}, errors.New("rate limited")
	})

	ns.On("action", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		return nil, nil
	})

	result, err := ns.HandleEvent("sid-1", "action", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "rate limited" {
		t.Fatalf("expected 'rate limited' error, got %v", err)
	}
	// Even with error, result data should be available for error responses
	if result == nil || len(result) != 1 {
		t.Fatalf("expected result data with error, got %v", result)
	}
}

func TestNamespace_ConcurrentAccess(t *testing.T) {
	ns := NewNamespace("/")
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			event := "event-" + json.Number(json.Number(string(rune('a'+n%26)))).String()
			ns.On(event, func(sid string, args ...json.RawMessage) ([]interface{}, error) {
				return nil, nil
			})
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ns.Events()
			_ = ns.HasEvent("event-a")
		}()
	}

	wg.Wait()
}
