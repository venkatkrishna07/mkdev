package tabs

import (
	"bufio"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

// LogsTickMsg is sent on each refresh tick.
type LogsTickMsg time.Time

// Logs is the Logs tab. It tails a daemon log file.
type Logs struct {
	th       styles.Theme
	logPath  string
	viewport viewport.Model
	width    int
	height   int
	paused   bool
}

// NewLogs constructs a Logs tab tailing path.
func NewLogs(th styles.Theme, logPath string) Logs {
	vp := viewport.New(100, 10)
	vp.SetContent("(no log entries yet)")
	return Logs{th: th, logPath: logPath, viewport: vp}
}

// Title implements tabs.Tab.
func (l Logs) Title() string { return "Logs" }

// Init starts the tail tick.
func (l Logs) Init() tea.Cmd { return logsTickCmd() }

// Update handles ticks, viewport scrolling, and tab-local keys.
func (l Logs) Update(msg tea.Msg) (Logs, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = m.Width
		l.height = m.Height
		l.viewport.Width = m.Width - 2
		l.viewport.Height = max(m.Height-10, 5)
	case LogsTickMsg:
		if !l.paused {
			l.refresh()
		}
		return l, logsTickCmd()
	case tea.KeyMsg:
		switch m.String() {
		case " ":
			l.paused = !l.paused
			return l, nil
		case "c":
			l.viewport.SetContent("")
			return l, nil
		}
	}
	var cmd tea.Cmd
	l.viewport, cmd = l.viewport.Update(msg)
	return l, cmd
}

// refresh reads the tail of the log file into the viewport. The last
// 2*viewport.Height lines are retained so a scroll-up gesture still has
// recent context without keeping the entire file in memory.
func (l *Logs) refresh() {
	f, err := os.Open(l.logPath) //nolint:gosec // logPath comes from runtime state dir
	if err != nil {
		l.viewport.SetContent(l.th.Dim.Render("log file not yet present at " + l.logPath))
		return
	}
	defer func() { _ = f.Close() }()
	const tailBytes = 64 * 1024
	if st, err := f.Stat(); err == nil && st.Size() > tailBytes {
		if _, err := f.Seek(-tailBytes, io.SeekEnd); err != nil {
			l.viewport.SetContent(l.th.Dim.Render("log seek failed: " + err.Error()))
			return
		}
	}
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) > 0 && len(lines[0]) > 0 {
		lines = lines[1:]
	}
	keep := l.viewport.Height
	if keep < 1 {
		keep = 20
	}
	if len(lines) > keep*2 {
		lines = lines[len(lines)-keep*2:]
	}
	for i, ln := range lines {
		lines[i] = l.colorize(ln)
	}
	l.viewport.SetContent(strings.Join(lines, "\n"))
	if !l.paused {
		l.viewport.GotoBottom()
	}
}

// View renders the header (path + paused pill) above the viewport body.
func (l Logs) View() string {
	hdr := l.th.Dim.Render("tailing " + l.logPath)
	if l.paused {
		hdr += "  " + l.th.PillDown.Render("PAUSED")
	}
	return hdr + "\n" + l.viewport.View()
}

func logsTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return LogsTickMsg(t) })
}

// colorize prepends a 1-cell colored stripe + tints the line based on slog
// level prefix (level=ERROR/WARN/INFO/DEBUG, case-insensitive).
func (l *Logs) colorize(line string) string {
	stripe, levelStyle := l.th.Dim, l.th.Dim
	switch detectLevel(line) {
	case levelError:
		stripe = lipgloss.NewStyle().Foreground(l.th.Bad)
		levelStyle = stripe.Bold(true)
	case levelWarn:
		stripe = lipgloss.NewStyle().Foreground(l.th.Warn)
		levelStyle = stripe.Bold(true)
	case levelInfo:
		stripe = lipgloss.NewStyle().Foreground(l.th.Info)
		levelStyle = l.th.Base
	case levelDebug:
		stripe = lipgloss.NewStyle().Foreground(l.th.Muted)
		levelStyle = l.th.Dim
	}
	return stripe.Render("▎") + " " + levelStyle.Render(line)
}

type logLevel int

const (
	levelUnknown logLevel = iota
	levelDebug
	levelInfo
	levelWarn
	levelError
)

func detectLevel(line string) logLevel {
	up := strings.ToUpper(line)
	switch {
	case strings.Contains(up, "LEVEL=ERROR"), strings.Contains(up, " ERROR "), strings.Contains(up, "[ERROR]"):
		return levelError
	case strings.Contains(up, "LEVEL=WARN"), strings.Contains(up, " WARN "), strings.Contains(up, "[WARN]"):
		return levelWarn
	case strings.Contains(up, "LEVEL=INFO"), strings.Contains(up, " INFO "), strings.Contains(up, "[INFO]"):
		return levelInfo
	case strings.Contains(up, "LEVEL=DEBUG"), strings.Contains(up, " DEBUG "), strings.Contains(up, "[DEBUG]"):
		return levelDebug
	}
	return levelUnknown
}
