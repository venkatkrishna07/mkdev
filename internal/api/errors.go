package api

import (
	"fmt"
	"net/http"
)

// ErrorCode is the machine-readable identifier returned in api.Error responses.
type ErrorCode string

// Error code values returned by the daemon.
const (
	CodeRouteDuplicate     ErrorCode = "route.duplicate"
	CodeRouteNotFound      ErrorCode = "route.not_found"
	CodeRouteInvalidName   ErrorCode = "route.invalid_name"
	CodeRouteInvalidTarget ErrorCode = "route.invalid_target"
	CodeStoreLocked        ErrorCode = "store.locked"
	CodeStoreWriteFailed   ErrorCode = "store.write_failed"
	CodeCertNotInstalled   ErrorCode = "cert.not_installed"
	CodeShareMDNSFailed    ErrorCode = "share.mdns_failed"
	CodeDaemonShuttingDown ErrorCode = "daemon.shutting_down"
)

// Error is the JSON body returned for all non-2xx daemon responses.
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// HTTPStatus maps an error code to the HTTP status the daemon returns.
func HTTPStatus(code ErrorCode) int {
	switch code {
	case CodeRouteDuplicate:
		return http.StatusConflict
	case CodeRouteNotFound:
		return http.StatusNotFound
	case CodeRouteInvalidName, CodeRouteInvalidTarget:
		return http.StatusBadRequest
	case CodeStoreLocked, CodeDaemonShuttingDown:
		return http.StatusServiceUnavailable
	case CodeStoreWriteFailed:
		return http.StatusInternalServerError
	case CodeCertNotInstalled:
		return http.StatusPreconditionFailed
	case CodeShareMDNSFailed:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
