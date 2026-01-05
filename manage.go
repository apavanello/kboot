package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
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

const (
	viewList = iota
	viewAddForm
)

type managerModel struct {
	view     int
	list     list.Model
	form     *huh.Form
	formData *ClusterConfig
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
		view:   viewList,
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

	switch m.view {
	case viewList:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if m.list.FilterState() == list.Filtering {
				break
			}

			if key.Matches(msg, keyAdd) {
				// Switch to Form View
				m.formData = &ClusterConfig{}
				m.form = newClusterForm(m.formData)
				m.view = viewAddForm
				// Initializing the form sends a message to focus it
				return m, m.form.Init()
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

					// Save immediately using shared helper
					if err := saveConfigToFile(m.path, m.config); err != nil {
						m.err = err
					}
					// Update list items
					items := make([]list.Item, len(m.config.Clusters))
					for i, c := range m.config.Clusters {
						items[i] = clusterItem{c}
					}
					m.list.SetItems(items)
					return m, nil
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
		}

		m.list, cmd = m.list.Update(msg)
		return m, cmd

	case viewAddForm:
		// Explicitly handle ESC to ensure user can back out
		if msg, ok := msg.(tea.KeyMsg); ok {
			if msg.Type == tea.KeyEsc {
				m.view = viewList
				m.status = "Cancelled"
				return m, nil
			}
		}

		// Update Huh Form
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateCompleted {
			// Save Data
			newCluster := Cluster{
				Alias:   strings.TrimSpace(m.formData.Alias),
				Name:    strings.TrimSpace(m.formData.ClusterName),
				Region:  strings.TrimSpace(m.formData.Region),
				Profile: strings.TrimSpace(m.formData.Profile),
			}
			if newCluster.Region == "" {
				newCluster.Region = "us-east-1"
			}

			m.config.Clusters = append(m.config.Clusters, newCluster)
			if err := saveConfigToFile(m.path, m.config); err != nil {
				m.err = err
			}

			// Reload List & Switch back
			items := make([]list.Item, len(m.config.Clusters))
			for i, c := range m.config.Clusters {
				items[i] = clusterItem{c}
			}
			m.list.SetItems(items)
			m.status = fmt.Sprintf("Added '%s'", newCluster.Alias)
			m.view = viewList
			return m, nil
		}

		if m.form.State == huh.StateAborted {
			m.view = viewList
			m.status = "Cancelled"
			return m, nil
		}

		return m, cmd
	}

	return m, nil
}

func (m managerModel) View() string {
	if m.quitting {
		return ""
	}

	if m.view == viewAddForm {
		return docStyle.Render(m.form.View())
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
