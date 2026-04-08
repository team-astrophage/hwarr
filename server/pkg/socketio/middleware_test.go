package socketio

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func TestMiddlewareChain_Empty(t *testing.T) {
	chain := NewMiddlewareChain()
	if chain.Len() != 0 {
		t.Fatalf("expected empty chain, got len=%d", chain.Len())
	}

	called := false
	handler := func() ([]interface{}, error) {
		called = true
		return []interface{}{"ok"}, nil
	}

	result, err := chain.Execute("sid1", "test", nil, handler)
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

func TestMiddlewareChain_NilHandler(t *testing.T) {
	chain := NewMiddlewareChain()
	result, err := chain.Execute("sid1", "test", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

func TestMiddlewareChain_SingleMiddleware(t *testing.T) {
	chain := NewMiddlewareChain()

	var order []string
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw-before")
		result, err := next()
		order = append(order, "mw-after")
		return result, err
	})

	handler := func() ([]interface{}, error) {
		order = append(order, "handler")
		return []interface{}{"done"}, nil
	}

	result, err := chain.Execute("sid1", "test", nil, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "done" {
		t.Fatalf("unexpected result: %v", result)
	}

	expected := []string{"mw-before", "handler", "mw-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}

func TestMiddlewareChain_MultipleMiddlewares_Order(t *testing.T) {
	chain := NewMiddlewareChain()

	var order []string

	// First middleware added → outermost → runs first
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw1-before")
		result, err := next()
		order = append(order, "mw1-after")
		return result, err
	})

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw2-before")
		result, err := next()
		order = append(order, "mw2-after")
		return result, err
	})

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw3-before")
		result, err := next()
		order = append(order, "mw3-after")
		return result, err
	})

	if chain.Len() != 3 {
		t.Fatalf("expected len=3, got %d", chain.Len())
	}

	handler := func() ([]interface{}, error) {
		order = append(order, "handler")
		return nil, nil
	}

	_, err := chain.Execute("sid1", "test", nil, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"mw1-before", "mw2-before", "mw3-before",
		"handler",
		"mw3-after", "mw2-after", "mw1-after",
	}
	if len(order) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}

func TestMiddlewareChain_ShortCircuit(t *testing.T) {
	chain := NewMiddlewareChain()

	handlerCalled := false
	mw2Called := false

	// First middleware short-circuits by not calling next
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return []interface{}{"blocked"}, nil
	})

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		mw2Called = true
		return next()
	})

	handler := func() ([]interface{}, error) {
		handlerCalled = true
		return nil, nil
	}

	result, err := chain.Execute("sid1", "test", nil, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handlerCalled {
		t.Fatal("handler should not have been called")
	}
	if mw2Called {
		t.Fatal("mw2 should not have been called")
	}
	if len(result) != 1 || result[0] != "blocked" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestMiddlewareChain_ErrorPropagation(t *testing.T) {
	chain := NewMiddlewareChain()
	errForbidden := errors.New("forbidden")

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return nil, errForbidden
	})

	handlerCalled := false
	handler := func() ([]interface{}, error) {
		handlerCalled = true
		return nil, nil
	}

	_, err := chain.Execute("sid1", "test", nil, handler)
	if !errors.Is(err, errForbidden) {
		t.Fatalf("expected errForbidden, got %v", err)
	}
	if handlerCalled {
		t.Fatal("handler should not have been called")
	}
}

func TestMiddlewareChain_ReceivesSidEventArgs(t *testing.T) {
	chain := NewMiddlewareChain()

	var capturedSID, capturedEvent string
	var capturedArgs []json.RawMessage

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		capturedSID = sid
		capturedEvent = event
		capturedArgs = args
		return next()
	})

	args := []json.RawMessage{json.RawMessage(`"hello"`), json.RawMessage(`42`)}
	_, err := chain.Execute("socket-123", "chat:message", args, func() ([]interface{}, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedSID != "socket-123" {
		t.Fatalf("expected sid=%q, got %q", "socket-123", capturedSID)
	}
	if capturedEvent != "chat:message" {
		t.Fatalf("expected event=%q, got %q", "chat:message", capturedEvent)
	}
	if len(capturedArgs) != 2 {
		t.Fatalf("expected 2 args, got %d", len(capturedArgs))
	}
}

func TestMiddlewareChain_ConcurrentAdd(t *testing.T) {
	chain := NewMiddlewareChain()
	var wg sync.WaitGroup

	n := 100
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
				return next()
			})
		}()
	}
	wg.Wait()

	if chain.Len() != n {
		t.Fatalf("expected len=%d, got %d", n, chain.Len())
	}
}

// --- Error handling and chain interruption tests ---

func TestMiddlewareChain_ErrorInMiddleMiddleware_StopsChain(t *testing.T) {
	chain := NewMiddlewareChain()
	errAuth := errors.New("authentication failed")

	var order []string

	// mw1: passthrough
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw1-before")
		result, err := next()
		order = append(order, "mw1-after")
		return result, err
	})

	// mw2: returns error — should stop chain
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw2-error")
		return nil, errAuth
	})

	// mw3: should never be reached
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "mw3")
		return next()
	})

	handlerCalled := false
	handler := func() ([]interface{}, error) {
		handlerCalled = true
		return nil, nil
	}

	_, err := chain.Execute("sid1", "test", nil, handler)
	if !errors.Is(err, errAuth) {
		t.Fatalf("expected errAuth, got %v", err)
	}
	if handlerCalled {
		t.Fatal("handler should not have been called")
	}

	// mw1 wraps mw2, so mw1-before runs, then mw2 errors, then mw1-after runs
	// (because mw1 called next() and processes the return)
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

func TestMiddlewareChain_ErrorFromHandler_PropagatesThroughChain(t *testing.T) {
	chain := NewMiddlewareChain()
	errHandler := errors.New("handler error")

	var mw1ReceivedErr, mw2ReceivedErr error

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		result, err := next()
		mw1ReceivedErr = err
		return result, err
	})

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		result, err := next()
		mw2ReceivedErr = err
		return result, err
	})

	handler := func() ([]interface{}, error) {
		return nil, errHandler
	}

	_, err := chain.Execute("sid1", "test", nil, handler)
	if !errors.Is(err, errHandler) {
		t.Fatalf("expected errHandler, got %v", err)
	}
	if !errors.Is(mw2ReceivedErr, errHandler) {
		t.Fatalf("mw2 should have received errHandler, got %v", mw2ReceivedErr)
	}
	if !errors.Is(mw1ReceivedErr, errHandler) {
		t.Fatalf("mw1 should have received errHandler, got %v", mw1ReceivedErr)
	}
}

func TestMiddlewareChain_MiddlewareCanTransformError(t *testing.T) {
	chain := NewMiddlewareChain()
	errOriginal := errors.New("original error")
	errTransformed := errors.New("transformed error")

	// Outer middleware catches and transforms the error
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		_, err := next()
		if err != nil {
			return []interface{}{"error_response"}, errTransformed
		}
		return nil, nil
	})

	// Inner middleware returns an error
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return nil, errOriginal
	})

	result, err := chain.Execute("sid1", "test", nil, nil)
	if !errors.Is(err, errTransformed) {
		t.Fatalf("expected errTransformed, got %v", err)
	}
	if len(result) != 1 || result[0] != "error_response" {
		t.Fatalf("expected error_response, got %v", result)
	}
}

func TestMiddlewareChain_MiddlewareCanSuppressError(t *testing.T) {
	chain := NewMiddlewareChain()
	errSuppressed := errors.New("suppressed error")

	// Outer middleware catches and suppresses the error
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		_, err := next()
		if err != nil {
			// Suppress error and return a fallback response
			return []interface{}{"fallback"}, nil
		}
		return nil, nil
	})

	// Inner middleware returns an error
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return nil, errSuppressed
	})

	result, err := chain.Execute("sid1", "test", nil, nil)
	if err != nil {
		t.Fatalf("error should have been suppressed, got %v", err)
	}
	if len(result) != 1 || result[0] != "fallback" {
		t.Fatalf("expected fallback response, got %v", result)
	}
}

func TestMiddlewareChain_NoNextCall_HandlerAndSubsequentMiddlewaresSkipped(t *testing.T) {
	chain := NewMiddlewareChain()

	mw2Called := false
	mw3Called := false
	handlerCalled := false

	// mw1: does NOT call next — chain should stop here
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return []interface{}{"stopped-at-mw1"}, nil
	})

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		mw2Called = true
		return next()
	})

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		mw3Called = true
		return next()
	})

	handler := func() ([]interface{}, error) {
		handlerCalled = true
		return nil, nil
	}

	result, err := chain.Execute("sid1", "test", nil, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw2Called {
		t.Fatal("mw2 should not have been called")
	}
	if mw3Called {
		t.Fatal("mw3 should not have been called")
	}
	if handlerCalled {
		t.Fatal("handler should not have been called")
	}
	if len(result) != 1 || result[0] != "stopped-at-mw1" {
		t.Fatalf("expected stopped-at-mw1 result, got %v", result)
	}
}

func TestMiddlewareChain_NoNextCall_ReturnsNilResult(t *testing.T) {
	chain := NewMiddlewareChain()

	// Middleware does not call next and returns nil
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return nil, nil
	})

	handlerCalled := false
	handler := func() ([]interface{}, error) {
		handlerCalled = true
		return []interface{}{"should-not-reach"}, nil
	}

	result, err := chain.Execute("sid1", "test", nil, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handlerCalled {
		t.Fatal("handler should not have been called")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

type middlewareTestError struct {
	Code    int
	Message string
}

func (e *middlewareTestError) Error() string {
	return e.Message
}

func TestMiddlewareChain_ErrorWithCustomData(t *testing.T) {
	chain := NewMiddlewareChain()

	errCustom := &middlewareTestError{Code: 403, Message: "forbidden"}

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		return nil, errCustom
	})

	handler := func() ([]interface{}, error) {
		return nil, nil
	}

	_, err := chain.Execute("sid1", "test", nil, handler)
	var ce *middlewareTestError
	if !errors.As(err, &ce) {
		t.Fatalf("expected middlewareTestError, got %T: %v", err, err)
	}
	if ce.Code != 403 || ce.Message != "forbidden" {
		t.Fatalf("unexpected custom error: %+v", ce)
	}
}

func TestMiddlewareChain_ErrorAndShortCircuitCombined(t *testing.T) {
	// Test: mw1 passes through, mw2 short-circuits with error, mw3 and handler not reached
	// mw1 should still see the error from mw2
	chain := NewMiddlewareChain()
	errReject := errors.New("rejected")

	var mw1SeenErr error

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		result, err := next()
		mw1SeenErr = err
		return result, err
	})

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		// Short-circuit with error — don't call next
		return nil, errReject
	})

	mw3Called := false
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		mw3Called = true
		return next()
	})

	handlerCalled := false
	handler := func() ([]interface{}, error) {
		handlerCalled = true
		return nil, nil
	}

	_, err := chain.Execute("sid1", "test", nil, handler)
	if !errors.Is(err, errReject) {
		t.Fatalf("expected errReject, got %v", err)
	}
	if !errors.Is(mw1SeenErr, errReject) {
		t.Fatalf("mw1 should have seen errReject, got %v", mw1SeenErr)
	}
	if mw3Called {
		t.Fatal("mw3 should not have been called")
	}
	if handlerCalled {
		t.Fatal("handler should not have been called")
	}
}

func TestMiddlewareChain_MultipleNextCalls_SecondCallSafe(t *testing.T) {
	// Verify calling next() multiple times doesn't cause issues
	chain := NewMiddlewareChain()
	callCount := 0

	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		// Call next twice
		result1, err1 := next()
		result2, err2 := next()
		_ = result2
		_ = err2
		return result1, err1
	})

	handler := func() ([]interface{}, error) {
		callCount++
		return []interface{}{"ok"}, nil
	}

	result, err := chain.Execute("sid1", "test", nil, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Handler should be called twice since next() is called twice
	if callCount != 2 {
		t.Fatalf("expected handler called 2 times, got %d", callCount)
	}
	if len(result) != 1 || result[0] != "ok" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestMiddlewareChain_MiddlewareModifiesResult(t *testing.T) {
	chain := NewMiddlewareChain()

	// Middleware wraps the handler result
	chain.Add(func(sid, event string, args []json.RawMessage, next func() ([]interface{}, error)) ([]interface{}, error) {
		result, err := next()
		if err != nil {
			return nil, err
		}
		return append(result, "added-by-mw"), nil
	})

	handler := func() ([]interface{}, error) {
		return []interface{}{"original"}, nil
	}

	result, err := chain.Execute("sid1", "test", nil, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 || result[0] != "original" || result[1] != "added-by-mw" {
		t.Fatalf("unexpected result: %v", result)
	}
}
