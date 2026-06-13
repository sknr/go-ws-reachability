package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockTransport implements http.RoundTripper for in-memory testing.
type mockTransport struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestCheckWebsiteStateTransitions(t *testing.T) {
	var (
		mu         sync.Mutex
		statusCode = http.StatusOK
		failConn   = false
	)

	// Create custom transport to intercept requests
	transport := &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			defer mu.Unlock()

			if failConn {
				return nil, errors.New("connection refused")
			}

			return &http.Response{
				StatusCode: statusCode,
				Status:     http.StatusText(statusCode),
				Body:       io.NopCloser(bytes.NewReader([]byte{})),
				Request:    req,
			}, nil
		},
	}

	// Configuration for the test
	cfg := AppConfig{
		ClientRequestTimeout: "1s",
		Websites: []WSConfig{
			{
				Name:     "TestSite",
				URL:      "https://example.com/test",
				Interval: "50ms", // Short interval for fast tests
			},
		},
	}

	// Capture sent alerts
	var (
		alerts      []string
		alertsMu    sync.Mutex
		alertsCount int32
	)

	app := &App{
		config:     cfg,
		httpClient: &http.Client{Transport: transport},
		sendAlert: func(message string) {
			alertsMu.Lock()
			alerts = append(alerts, message)
			alertsMu.Unlock()
			atomic.AddInt32(&alertsCount, 1)
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start checkWebsite in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.checkWebsite(ctx, cfg.Websites[0])
	}()

	// 1. Initial State: Server returns 200 OK.
	// Wait a bit to let it run. No alerts should be sent.
	time.Sleep(150 * time.Millisecond)

	alertsMu.Lock()
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts initially, got %d: %v", len(alerts), alerts)
	}
	alertsMu.Unlock()

	// 2. Transition to DOWN: Server returns 500 Internal Server Error
	mu.Lock()
	statusCode = http.StatusInternalServerError
	mu.Unlock()

	// Wait for the next ticker tick and assertion
	waitForAlertCount(t, &alertsCount, 1, 300*time.Millisecond)

	alertsMu.Lock()
	if len(alerts) != 1 {
		t.Fatalf("Expected exactly 1 alert, got %d: %v", len(alerts), alerts)
	}
	if !contains(alerts[0], "is down!") || !contains(alerts[0], "StatusCode: 500") {
		t.Errorf("Alert content incorrect: %q", alerts[0])
	}
	alertsMu.Unlock()

	// 3. Keep DOWN: Wait another period. No new alerts should be sent (spam prevention)
	time.Sleep(150 * time.Millisecond)
	alertsMu.Lock()
	if len(alerts) != 1 {
		t.Errorf("Expected still exactly 1 alert (spam prevention), got %d: %v", len(alerts), alerts)
	}
	alertsMu.Unlock()

	// 4. Transition to UP: Server returns 200 OK again
	mu.Lock()
	statusCode = http.StatusOK
	mu.Unlock()

	// Wait for the recovery alert
	waitForAlertCount(t, &alertsCount, 2, 300*time.Millisecond)

	alertsMu.Lock()
	if len(alerts) != 2 {
		t.Fatalf("Expected exactly 2 alerts (including recovery), got %d: %v", len(alerts), alerts)
	}
	if !contains(alerts[1], "recovered") || !contains(alerts[1], "OK!") {
		t.Errorf("Recovery alert content incorrect: %q", alerts[1])
	}
	alertsMu.Unlock()

	// 5. Keep UP: Wait another period. No new alerts.
	time.Sleep(150 * time.Millisecond)
	alertsMu.Lock()
	if len(alerts) != 2 {
		t.Errorf("Expected still exactly 2 alerts, got %d: %v", len(alerts), alerts)
	}
	alertsMu.Unlock()

	// 6. Transition to DOWN due to connection failure
	mu.Lock()
	failConn = true
	mu.Unlock()

	// Wait for the down alert
	waitForAlertCount(t, &alertsCount, 3, 300*time.Millisecond)

	alertsMu.Lock()
	if len(alerts) != 3 {
		t.Fatalf("Expected exactly 3 alerts (connection failure down), got %d: %v", len(alerts), alerts)
	}
	if !contains(alerts[2], "is down!") || !contains(alerts[2], "Error:") {
		t.Errorf("Connection failure alert content incorrect: %q", alerts[2])
	}
	alertsMu.Unlock()

	// Stop monitoring and wait for goroutine to finish
	cancel()
	wg.Wait()
}

func contains(s, substr string) bool {
	// Simple manual contains to avoid importing strings package
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func waitForAlertCount(t *testing.T, count *int32, target int32, timeout time.Duration) {
	start := time.Now()
	for time.Since(start) < timeout {
		if atomic.LoadInt32(count) >= target {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Timed out waiting for alert count to reach %d (current: %d)", target, atomic.LoadInt32(count))
}
