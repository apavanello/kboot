package ui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"kboot/internal/app"
	"kboot/internal/config"
)

// Styles
var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))             // Green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))            // Red
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))            // Gray
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")) // Pink
)

type itemState int

const (
	statePending itemState = iota
	stateRunning
	stateSuccess
	stateError
)

type clusterItem struct {
	alias   string
	state   itemState
	message string
}

type LoadingModel struct {
	clusters map[string]*clusterItem
	keys     []string // For ordered rendering

	sub      chan app.Event
	spinner  spinner.Model
	quitting bool
}

func NewLoadingModel(cfg *config.Config, sub chan app.Event) LoadingModel {
	m := LoadingModel{
		clusters: make(map[string]*clusterItem),
		sub:      sub,
		spinner:  spinner.New(),
	}
	m.spinner.Spinner = spinner.Dot
	m.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	for _, c := range cfg.Clusters {
		m.clusters[c.Alias] = &clusterItem{
			alias:   c.Alias,
			state:   statePending,
			message: "Waiting...",
		}
		m.keys = append(m.keys, c.Alias)
	}
	sort.Strings(m.keys) // Consistent order
	return m
}

func (m LoadingModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		waitForEvent(m.sub), // Listen to channel
	)
}

// waitForEvent is a command that waits for a channel msg
func waitForEvent(sub <-chan app.Event) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-sub
		if !ok {
			return nil // Channel closed
		}
		return evt
	}
}

func (m LoadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case app.Event:
		// Update state based on event
		item, exists := m.clusters[msg.ClusterAlias]
		if exists {
			switch msg.Type {
			case app.EventStart:
				item.state = stateRunning
				item.message = msg.Message
			case app.EventSuccess:
				item.state = stateSuccess
				item.message = msg.Message
			case app.EventError:
				item.state = stateError
				item.message = msg.Message
			}
		}
		// Continue listening
		return m, waitForEvent(m.sub)

	case bool: // Done signal (simple usage)
		if msg {
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m LoadingModel) View() string {
	if m.quitting {
		return ""
	}

	s := titleStyle.Render(fmt.Sprintf("Booting %d clusters...", len(m.keys))) + "\n\n"

	for _, k := range m.keys {
		item := m.clusters[k]

		var icon string
		var msgStyle lipgloss.Style

		switch item.state {
		case statePending:
			icon = "•"
			msgStyle = pendingStyle
		case stateRunning:
			icon = m.spinner.View()
			msgStyle = pendingStyle
		case stateSuccess:
			icon = "✓"
			msgStyle = successStyle
		case stateError:
			icon = "x"
			msgStyle = errorStyle
		}

		s += fmt.Sprintf(" %s %s: %s\n", icon, item.alias, msgStyle.Render(item.message))
	}

	s += "\n" + pendingStyle.Render("Press 'q' to cancel")
	return s
}
