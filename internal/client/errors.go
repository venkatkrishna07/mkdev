package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

var ErrDaemonDown = errors.New("daemon not running")

func isDaemonDown(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if errors.Is(err, io.EOF) {
		return true
	}

	for _, frag := range []string{"connect: no such file", "connect: connection refused", "connect: permission denied"} {
		if containsFold(msg, frag) {
			return true
		}
	}
	return false
}

func containsFold(s, frag string) bool {
	return len(frag) > 0 && len(s) >= len(frag) && indexFold(s, frag) >= 0
}

func indexFold(s, frag string) int {
	for i := 0; i+len(frag) <= len(s); i++ {
		if equalFold(s[i:i+len(frag)], frag) {
			return i
		}
	}
	return -1
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func decodeError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var apiErr api.Error
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Code != "" {
		return apiErr
	}
	return fmt.Errorf("daemon: http %d: %s", resp.StatusCode, string(body))
}
