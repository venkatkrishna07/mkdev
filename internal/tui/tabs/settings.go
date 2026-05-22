package tabs

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/venkatkrishna07/mkdev/internal/config"
	"github.com/venkatkrishna07/mkdev/internal/tui/styles"
)

const numSettingsFields = 5

// SettingsSavedMsg is emitted after a successful save so the root model can
// refresh its in-memory config copy.
type SettingsSavedMsg struct{ Cfg config.Config }

// Settings is the Settings tab. It edits ~/.mkdev/config.toml in place.
type Settings struct {
	th         styles.Theme
	home       string
	textFields [numSettingsFields]textinput.Model
	textLabels [numSettingsFields]string
	focus      int
	status     string
}

// NewSettings constructs a Settings tab seeded from the on-disk config.
func NewSettings(th styles.Theme, home string) Settings {
	s := Settings{th: th, home: home}
	cfg, _ := config.Load(filepath.Join(home, "config.toml"))
	s.textLabels = [numSettingsFields]string{"tld", "proxy_port", "theme", "log_retention", "log_max_size"}
	s.textFields[0] = mkField(cfg.TLD)
	s.textFields[1] = mkField(strconv.Itoa(cfg.ProxyPort))
	s.textFields[2] = mkField(cfg.Theme)
	s.textFields[3] = mkField(cfg.LogRetention)
	s.textFields[4] = mkField(cfg.LogMaxSize)
	s.textFields[0].Focus()
	return s
}

func mkField(value string) textinput.Model {
	t := textinput.New()
	t.SetValue(value)
	return t
}

// Title implements tabs.Tab.
func (s Settings) Title() string { return "Settings" }

// Init starts the textinput cursor blink.
func (s Settings) Init() tea.Cmd { return textinput.Blink }

// focusOn moves focus to index i, updating textinput focus states accordingly.
func (s *Settings) focusOn(i int) {
	for j := range s.textFields {
		s.textFields[j].Blur()
	}
	s.textFields[i].Focus()
	s.focus = i
}

// Update handles focus cycling, save, and textinput forwarding.
func (s Settings) Update(msg tea.Msg) (Settings, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyDown:
			s.focusOn((s.focus + 1) % numSettingsFields)
			return s, textinput.Blink
		case tea.KeyUp:
			s.focusOn((s.focus - 1 + numSettingsFields) % numSettingsFields)
			return s, textinput.Blink
		case tea.KeyEnter:
			next := (s.focus + 1) % numSettingsFields
			s.focusOn(next)
			return s, textinput.Blink
		}
		switch k.String() {
		case "ctrl+s":
			return s.save()
		case "ctrl+r":
			return NewSettings(s.th, s.home), textinput.Blink
		}
	}
	var cmd tea.Cmd
	s.textFields[s.focus], cmd = s.textFields[s.focus].Update(msg)
	return s, cmd
}

func (s Settings) save() (Settings, tea.Cmd) {
	port, err := strconv.Atoi(strings.TrimSpace(s.textFields[1].Value()))
	if err != nil || port < 1 || port > 65535 {
		s.status = "x proxy_port must be 1-65535"
		return s, nil
	}
	cfg := config.Config{
		TLD:          strings.TrimSpace(s.textFields[0].Value()),
		ProxyPort:    port,
		Theme:        strings.TrimSpace(s.textFields[2].Value()),
		LogRetention: strings.TrimSpace(s.textFields[3].Value()),
		LogMaxSize:   strings.TrimSpace(s.textFields[4].Value()),
	}
	if err := config.Save(filepath.Join(s.home, "config.toml"), cfg); err != nil {
		s.status = "x save failed: " + err.Error()
		return s, nil
	}
	s.status = "saved"
	return s, func() tea.Msg { return SettingsSavedMsg{Cfg: cfg} }
}

// View renders the field stack with a focus arrow and a footer hint line.
func (s Settings) View() string {
	var out strings.Builder
	for i, f := range s.textFields {
		label := s.th.Dim.Render(s.textLabels[i] + ":")
		if i == s.focus {
			label = s.th.Title.Render("> " + s.textLabels[i] + ":")
		}
		out.WriteString(label + " " + f.View() + "\n")
	}
	out.WriteString("\n")
	if s.status != "" {
		out.WriteString(s.th.Dim.Render(s.status) + "\n")
	}
	out.WriteString(s.th.Dim.Render("↑↓ field · ↵ next · ⌃s save · ⌃r reset"))
	return out.String()
}
