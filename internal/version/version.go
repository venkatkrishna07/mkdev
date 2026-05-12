package version

import "fmt"

// Set via -ldflags at build time. Defaults are placeholders for go run / go test.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String formats version, commit, and date as one line.
func String() string {
	if Commit == "none" {
		return Version
	}
	return fmt.Sprintf("%s (%s @ %s)", Version, Commit, Date)
}
