package tabs

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/cert"
	"github.com/venkatkrishna07/mkdev/internal/cert/trust"
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/mdns"
	"github.com/venkatkrishna07/mkdev/internal/store"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// CheckStatus is the outcome severity of a single Doctor check.
type CheckStatus int

const (
	// CheckOK means the check passed cleanly.
	CheckOK CheckStatus = iota
	// CheckWarn means the check passed with a caveat.
	CheckWarn
	// CheckFail means the check failed and likely needs operator action.
	CheckFail
)

// CheckResult is one row in the Doctor output.
type CheckResult struct {
	Name   string
	Status CheckStatus
	Detail string
}

// Doctor is the Doctor tab.
type Doctor struct {
	th      styles.Theme
	home    string
	store   *store.Store
	results []CheckResult
}

// NewDoctor constructs a Doctor tab against home (e.g. ~/.mkdev) and runs
// the initial battery of checks.
func NewDoctor(th styles.Theme, home string, st *store.Store) Doctor {
	d := Doctor{th: th, home: home, store: st}
	d.runChecks()
	return d
}

// Title implements tabs.Tab.
func (d Doctor) Title() string { return "Doctor" }

// Init implements tea.Model. Doctor has no async startup work.
func (d Doctor) Init() tea.Cmd { return nil }

// DoctorRunMsg is sent when the user presses r to re-run.
type DoctorRunMsg struct{}

// Update handles the 'r' rerun shortcut. All other input is ignored.
func (d Doctor) Update(msg tea.Msg) (Doctor, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "r" {
		d.runChecks()
	}
	return d, nil
}

func (d *Doctor) runChecks() {
	d.results = nil
	d.results = append(d.results, d.checkStateDir())
	d.results = append(d.results, d.checkConfig())
	caRes, ca := d.checkCAFiles()
	d.results = append(d.results, caRes)
	if ca != nil {
		d.results = append(d.results, d.checkCATrusted(ca))
		d.results = append(d.results, d.checkStaleCAs())
	}
	d.results = append(d.results, d.checkProxyPort())
	d.results = append(d.results, d.checkHosts())
	d.results = append(d.results, d.checkLANIP())
	d.results = append(d.results, d.checkSharedRoutes())
}

func (d Doctor) checkStateDir() CheckResult {
	if _, err := os.Stat(d.home); err != nil {
		return CheckResult{"state directory", CheckFail, err.Error()}
	}
	return CheckResult{"state directory", CheckOK, d.home}
}

func (d Doctor) checkConfig() CheckResult {
	p := filepath.Join(d.home, "config.toml")
	if _, err := os.Stat(p); err != nil {
		return CheckResult{"config.toml", CheckFail, err.Error()}
	}
	return CheckResult{"config.toml", CheckOK, p}
}

func (d Doctor) checkCAFiles() (CheckResult, *cert.CA) {
	caDir := filepath.Join(d.home, "ca")
	ca, err := cert.LoadCA(caDir)
	if err != nil {
		return CheckResult{"CA on disk", CheckFail, err.Error()}, nil
	}
	return CheckResult{"CA on disk", CheckOK, caDir}, ca
}

func (d Doctor) checkCATrusted(ca *cert.CA) CheckResult {
	ok, err := trust.IsInstalled(ca.Cert)
	if err != nil {
		return CheckResult{"CA in keychain", CheckFail, err.Error()}
	}
	if !ok {
		return CheckResult{"CA in keychain", CheckFail, "not trusted — run `mkdev install`"}
	}
	return CheckResult{"CA in keychain", CheckOK, "trusted"}
}

func (d Doctor) checkStaleCAs() CheckResult {
	fps, err := trust.ListMkdevCerts()
	if err != nil {
		return CheckResult{"stale CAs", CheckWarn, err.Error()}
	}
	if len(fps) > 1 {
		detail := fmt.Sprintf("%d mkdev CAs in keychain — older entries may need manual cleanup", len(fps))
		return CheckResult{"stale CAs", CheckWarn, detail}
	}
	return CheckResult{"stale CAs", CheckOK, "single CA"}
}

func (d Doctor) checkProxyPort() CheckResult {
	cfg, _ := config.Load(filepath.Join(d.home, "config.toml"))
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.ProxyPort))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return CheckResult{"proxy port", CheckWarn, "in use: " + err.Error()}
	}
	_ = ln.Close() // probe-only listener; close error is irrelevant
	return CheckResult{"proxy port", CheckOK, addr + " available"}
}

func (d Doctor) checkHosts() CheckResult {
	if _, err := os.ReadFile("/etc/hosts"); err != nil {
		return CheckResult{"/etc/hosts", CheckFail, err.Error()}
	}
	return CheckResult{"/etc/hosts", CheckOK, "readable"}
}

func (d Doctor) checkLANIP() CheckResult {
	ip, err := mdns.PrimaryLANIPv4()
	if err != nil {
		return CheckResult{"LAN IP", CheckFail, err.Error()}
	}
	return CheckResult{"LAN IP", CheckOK, "advertising routes to " + ip.String()}
}

func (d Doctor) checkSharedRoutes() CheckResult {
	if d.store == nil {
		return CheckResult{"shared routes", CheckWarn, "store handle unavailable"}
	}
	routes, err := d.store.ListRoutes()
	if err != nil {
		return CheckResult{"shared routes", CheckWarn, err.Error()}
	}
	n := 0
	for _, r := range routes {
		if r.Enabled && r.Shared {
			n++
		}
	}
	if n == 0 {
		return CheckResult{"shared routes", CheckOK, "0 — LAN access denied"}
	}
	ip, ipErr := mdns.PrimaryLANIPv4()
	detail := fmt.Sprintf("%d enabled", n)
	if ipErr == nil {
		detail += " — advertising on " + ip.String()
	}
	return CheckResult{"shared routes", CheckOK, detail}
}

// View renders the check list with ✓/!/✗ glyphs and a re-run hint.
func (d Doctor) View() string {
	if len(d.results) == 0 {
		return d.th.Dim.Render("no checks run yet — press r")
	}
	var out string
	for _, r := range d.results {
		glyph := "✓"
		style := d.th.PillUp
		switch r.Status {
		case CheckWarn:
			glyph = "!"
			style = d.th.PillDown
		case CheckFail:
			glyph = "✗"
			style = d.th.PillDown
		}
		out += style.Render(glyph) + " " + d.th.Title.Render(r.Name) + "  " + d.th.Dim.Render(r.Detail) + "\n"
	}
	out += "\n" + d.th.Dim.Render("press ") + d.th.FooterKey.Render("r") + d.th.Dim.Render(" to re-run")
	return out
}
