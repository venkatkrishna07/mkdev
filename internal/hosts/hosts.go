package hosts

import (
	"bufio"
	"io"
	"regexp"
	"slices"
	"strings"
)

// Entry is a parsed host file line.
type Entry struct {
	IP   string
	Host string
}

var hostnameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

// ValidHostname returns true if h is a syntactically valid lower-case DNS name.
// Used to guard /etc/hosts writes against injection.
func ValidHostname(h string) bool {
	if len(h) == 0 || len(h) > 253 {
		return false
	}
	return hostnameRE.MatchString(h)
}

// Parse extracts host entries from an /etc/hosts-formatted reader, skipping
// comments and blank lines. Multi-host lines yield one Entry per host.
func Parse(r io.Reader) []Entry {
	out := []Entry{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		for _, h := range fields[1:] {
			out = append(out, Entry{IP: ip, Host: h})
		}
	}
	return out
}

// AddEntry appends "ip\thost\t# comment" to body if no line already maps host.
// Returns the new body and whether a change was made.
func AddEntry(body, ip, host, comment string) (string, bool) {
	for _, e := range Parse(strings.NewReader(body)) {
		if e.Host == host {
			return body, false
		}
	}
	if !strings.HasSuffix(body, "\n") && body != "" {
		body += "\n"
	}
	body += ip + "\t" + host + "\t# " + comment + "\n"
	return body, true
}

// RemoveEntry deletes any line that maps host. Returns the new body and
// whether a change was made.
func RemoveEntry(body, host string) (string, bool) {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		stripped, _, _ := strings.Cut(line, "#")
		fields := strings.Fields(stripped)
		if len(fields) >= 2 && slices.Contains(fields[1:], host) {
			changed = true
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), changed
}
