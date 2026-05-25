package daemon

import (
	"net"
	"net/url"
	"regexp"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ValidateName checks that name is a valid DNS label fragment for a route.
// Rules: lowercase a-z 0-9 and '-', must start alphanumeric, 1-63 chars.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return api.Error{
			Code:    api.CodeRouteInvalidName,
			Message: "name must match ^[a-z0-9][a-z0-9-]{0,62}$",
		}
	}
	return nil
}

// ValidateTarget checks that target parses as host:port or as a full URL
// with http/https scheme and a host component.
func ValidateTarget(target string) error {
	if target == "" {
		return targetErr("target empty")
	}
	if _, _, err := net.SplitHostPort(target); err == nil {
		return nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return targetErr("invalid url: " + err.Error())
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return targetErr("target must be host:port or http(s)://host[:port]")
	}
	if u.Host == "" {
		return targetErr("url missing host")
	}
	return nil
}

func targetErr(msg string) error {
	return api.Error{Code: api.CodeRouteInvalidTarget, Message: msg}
}
