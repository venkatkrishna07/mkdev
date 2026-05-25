package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

// Synthetic event types emitted by Subscribe (not sent by daemon over the wire).
// Consumers use these to drive UI affordances during transient disconnects.
const (
	EventClientDisconnected api.EventType = "client.disconnected"
	EventClientReconnected  api.EventType = "client.reconnected"
)

// Subscribe opens an SSE stream from the daemon's /v1/events endpoint and
// returns a channel of decoded events. The reader runs in a background
// goroutine until ctx is cancelled. On EOF / network error the channel
// emits EventClientDisconnected and the goroutine retries with backoff;
// on successful reconnect EventClientReconnected is emitted.
//
// The returned channel is closed only when ctx is done.
func (c *Client) Subscribe(ctx context.Context) <-chan api.Event {
	out := make(chan api.Event, 64)
	go c.subscribeLoop(ctx, out)
	return out
}

func (c *Client) subscribeLoop(ctx context.Context, out chan<- api.Event) {
	defer close(out)
	backoff := 100 * time.Millisecond
	const maxBackoff = 5 * time.Second
	connected := false
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.streamEvents(ctx, out, &connected)
		if ctx.Err() != nil {
			return
		}
		if connected {
			connected = false
			select {
			case out <- api.Event{Type: EventClientDisconnected, Data: nil}:
			case <-ctx.Done():
				return
			}
		}
		slog.Debug("client: SSE disconnected, retrying", "err", err, "backoff", backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// streamEvents opens one /v1/events connection and drains it until error / EOF.
// Sets *connected = true after the response header is read so the caller knows
// whether to emit a disconnected event on exit.
func (c *Client) streamEvents(ctx context.Context, out chan<- api.Event, connected *bool) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v1/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("client: SSE status %d", resp.StatusCode)
	}
	wasConnected := *connected
	*connected = true
	if !wasConnected {
		select {
		case out <- api.Event{Type: EventClientReconnected, Data: nil}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var dataBuf strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if dataBuf.Len() == 0 {
				continue
			}
			payload := dataBuf.String()
			dataBuf.Reset()
			var ev api.Event
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				slog.Warn("client: SSE decode failed", "err", err, "payload", payload)
				continue
			}
			select {
			case out <- ev:
			case <-ctx.Done():
				return ctx.Err()
			}
		case strings.HasPrefix(line, "data:"):
			chunk := strings.TrimPrefix(line, "data:")
			chunk = strings.TrimPrefix(chunk, " ")
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(chunk)
		default:
			// event/comment/id/retry lines are accepted but not needed —
			// the envelope JSON in `data:` carries the Type.
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}
