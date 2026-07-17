package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type healthChecker interface {
	Wait(context.Context, string, time.Duration) error
}

type httpHealthChecker struct {
	Client   *http.Client
	Interval time.Duration
}

func (checker httpHealthChecker) Wait(ctx context.Context, endpoint string, timeout time.Duration) error {
	client := checker.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	interval := checker.Interval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var lastErr error
	for {
		request, err := http.NewRequestWithContext(waitCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8<<10))
			closeErr := response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 && closeErr == nil {
				return nil
			}
			lastErr = fmt.Errorf("HTTP %s", response.Status)
		} else {
			lastErr = err
		}
		timer := time.NewTimer(interval)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("health check timed out after %s (last error: %v)", timeout, lastErr)
			}
			return waitCtx.Err()
		case <-timer.C:
		}
	}
}
