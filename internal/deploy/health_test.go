package deploy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestHTTPHealthCheckerRetriesUntilSuccess(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		status := http.StatusServiceUnavailable
		if attempts == 3 {
			status = http.StatusNoContent
		}
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Body:       io.NopCloser(strings.NewReader("status")),
		}, nil
	})}
	checker := httpHealthChecker{Client: client, Interval: time.Millisecond}
	if err := checker.Wait(context.Background(), "https://check.example.com/healthz", 100*time.Millisecond); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestHTTPHealthCheckerTimesOut(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("not ready")
	})}
	checker := httpHealthChecker{Client: client, Interval: time.Millisecond}
	err := checker.Wait(context.Background(), "https://check.example.com/healthz", 5*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("Wait error = %v", err)
	}
}
