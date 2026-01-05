package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// --- TUI Manager (v2.1.0) ---

var (
	docStyle     = lipgloss.NewStyle().Margin(1, 2)
	statusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#bd534b"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#42bd53"))
)

type clusterItem struct {
	Cluster
}

func (i clusterItem) Title() string { return i.Alias }
func (i clusterItem) Description() string {
	return fmt.Sprintf("%s (%s @ %s)", i.Name, i.Profile, i.Region)
}
func (i clusterItem) FilterValue() string { return i.Alias }

type managerModel struct {
	list     list.Model
	config   *Config
	path     string
	err      error
	status   string
	quitting bool
}

func initialManagerModel() (*managerModel, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	cfg, err := loadConfig(path)
	if err != nil {
		return nil, err
	}

	items := make([]list.Item, len(cfg.Clusters))
	for i, c := range cfg.Clusters {
		items[i] = clusterItem{c}
	}

	// Setup List
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Kboot Clusters"
	l.SetShowHelp(true)
	// Bindings: Enter (Edit - Todo), a (Add), d (Delete)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			keyAdd,
			keyDelete,
		}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			keyAdd,
			keyDelete,
		}
	}

	return &managerModel{
		list:   l,
		config: cfg,
		path:   path,
	}, nil
}

// Key bindings
var (
	keyAdd = key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add cluster"),
	)
	keyDelete = key.NewBinding(
		key.WithKeys("d", "delete"),
		key.WithHelp("d", "delete"),
	)
	// Enter is built-in to list for selecting, but we intercept it.
)

func (m managerModel) Init() tea.Cmd {
	return nil
}

func (m managerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		if key.Matches(msg, keyAdd) {
			// Trigger Add Process
			return m, m.cmdAddCluster
		}

		if key.Matches(msg, keyDelete) {
			if len(m.config.Clusters) == 0 {
				m.status = "No clusters to delete"
				return m, nil
			}
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.config.Clusters) {
				// Remove
				deleted := m.config.Clusters[idx].Alias
				m.config.Clusters = append(m.config.Clusters[:idx], m.config.Clusters[idx+1:]...)
				m.status = fmt.Sprintf("Deleted '%s'", deleted)
				return m, m.cmdSaveConfig
			}
		}

		if msg.String() == "enter" {
			// Edit not implemented yet, just show status
			m.status = "Edit feature coming in v2.2.0"
			return m, nil
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case cmdRunAdd:
		// Launch kboot cluster add as subprocess
		exe, err := os.Executable()
		if err != nil {
			m.err = err
			return m, nil
		}
		c := exec.Command(exe, "cluster", "add")
		// tea.ExecProcess handles stdin/stdout/stderr for us
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				return fmt.Errorf("process finished with error: %v", err)
			}
			return m.cmdReloadFromDisk()
		})

	case cmdReload:
		m.list.SetItems(msg.items)
		m.status = "Configuration reloaded"
		return m, nil

	case error:
		m.err = msg
		return m, nil
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *managerModel) cmdReloadFromDisk() tea.Msg {
	cfg, err := loadConfig(m.path)
	if err != nil {
		return err
	}
	m.config = cfg // Update internal state

	items := make([]list.Item, len(cfg.Clusters))
	for i, c := range cfg.Clusters {
		items[i] = clusterItem{c}
	}
	return cmdReload{items}
}

func (m managerModel) View() string {
	if m.quitting {
		return ""
	}

	s := docStyle.Render(m.list.View())

	if m.status != "" {
		s += "\n" + statusStyle.Render(m.status)
	}
	if m.err != nil {
		s += "\n" + errStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	return s
}

// Commands
func (m *managerModel) cmdSaveConfig() tea.Msg {
	if err := saveConfig(m.path, m.config); err != nil {
		m.err = err
		return nil
	}
	// Reload items
	items := make([]list.Item, len(m.config.Clusters))
	for i, c := range m.config.Clusters {
		items[i] = clusterItem{c}
	}
	return cmdReload{items}
}

func (m *managerModel) cmdAddCluster() tea.Msg {
	// Execute the interactive Add function (blocking)
	// We need to suspend the Tea program to let clusterAdd print to stdout,
	// technically Tea has Exec methods but clusterAdd is our own function content.
	// Actually, best way in simple tool:
	return cmdRunAdd{}
}

// Custom Messages
type cmdReload struct {
	items []list.Item
}
type cmdRunAdd struct{}

// Helper to save (extracted from clusterAdd)
func saveConfig(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(2)
	return encoder.Encode(cfg)
}

// Entry point
func runManager() {
	m, err := initialManagerModel()
	if err != nil {
		fatal("Could not remove dashboard: %v", err)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fatal("Error running dashboard: %v", err)
	}
}
