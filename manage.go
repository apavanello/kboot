package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// --- Unified TUI Manager (v2.3.0) ---

var (
	docStyle     = lipgloss.NewStyle().Margin(1, 2)
	titleStyle   = lipgloss.NewStyle().Margin(1, 0, 0, 2).Foreground(lipgloss.Color("205")).Bold(true)
	statusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#bd534b"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#42bd53"))
)

// View states
const (
	viewMainMenu = iota
	viewClusterList
	viewClusterAddForm
	viewClusterEditForm
	viewClusterDeleteConfirm
	viewStaticCredsList
	viewStaticAddForm
	viewStaticEditForm
	viewStaticDeleteConfirm
	viewSSOProfilesList
	viewSSOAddForm
	viewSSOEditForm
	viewSSODeleteConfirm
)

// Menu items
type menuItem struct {
	title string
	desc  string
	value int
}

func (m menuItem) Title() string       { return m.title }
func (m menuItem) Description() string { return m.desc }
func (m menuItem) FilterValue() string { return m.title }

// Cluster list items
type clusterItem struct {
	Cluster
}

func (i clusterItem) Title() string { return i.Alias }
func (i clusterItem) Description() string {
	return fmt.Sprintf("%s (%s @ %s)", i.Name, i.Profile, i.Region)
}
func (i clusterItem) FilterValue() string { return i.Alias }

// Credential item for display
type credentialItem struct {
	ProfileName string
	AccessKey   string // Masked
	SecretKey   string // Masked
	Token       string // Masked
	IsSSO       bool
	// SSO specific
	SSOSession string
	AccountID  string
	RoleName   string
	Region     string
	StartURL   string // SSO Start URL from sso-session block
}

func (c credentialItem) Title() string { return c.ProfileName }
func (c credentialItem) Description() string {
	if c.IsSSO {
		return fmt.Sprintf("SSO: %s | Account: %s | Role: %s", c.SSOSession, c.AccountID, c.RoleName)
	}
	return fmt.Sprintf("Key: %s", c.AccessKey)
}
func (c credentialItem) FilterValue() string { return c.ProfileName }

// Auth form data
type AuthFormData struct {
	Profile     string
	AccessKey   string
	SecretKey   string
	Token       string
	SessionName string
	StartURL    string
	Region      string
	AccountID   string
	RoleName    string
}

type managerModel struct {
	view     int
	mainMenu list.Model
	list     list.Model // Used for clusters
	credList list.Model // Used for credentials
	form     *huh.Form

	// Cluster data
	clusterFormData *ClusterConfig
	editIdx         int
	config          *Config
	path            string

	// Auth data
	authFormData *AuthFormData
	staticCreds  []credentialItem
	ssoProfiles  []credentialItem
	authEditIdx  int

	// Delete confirmation
	pendingDeleteIdx  int
	pendingDeleteName string
	confirmDelete     *bool // Pointer to survive value copy in Update

	err      error
	status   string
	quitting bool
}

const (
	headerHeight = 2
	footerHeight = 2
)

// Key bindings
var (
	keyAdd = key.NewBinding(
		key.WithKeys("a", "A"),
		key.WithHelp("a", "adicionar"),
	)
	keyDelete = key.NewBinding(
		key.WithKeys("d", "D"),
		key.WithHelp("d", "deletar"),
	)
	keyEdit = key.NewBinding(
		key.WithKeys("e", "E", "enter"),
		key.WithHelp("e/enter", "editar"),
	)
	keyDuplicate = key.NewBinding(
		key.WithKeys("c", "C"),
		key.WithHelp("c", "duplicar"),
	)
	keyBack = key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc", "voltar"),
	)
)

// Mask a string showing only first 4 chars
func maskString(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-4)
}

// Parse static credentials from ~/.aws/credentials
func loadStaticCredentials() []credentialItem {
	home, err := getHomeDir()
	if err != nil {
		return nil
	}

	credPath := filepath.Join(home, ".aws", "credentials")
	f, err := os.Open(credPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var creds []credentialItem
	var current *credentialItem
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if current != nil {
				creds = append(creds, *current)
			}
			profileName := line[1 : len(line)-1]
			current = &credentialItem{ProfileName: profileName, IsSSO: false}
		} else if current != nil && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "aws_access_key_id":
				current.AccessKey = maskString(value)
			case "aws_secret_access_key":
				current.SecretKey = maskString(value)
			case "aws_session_token":
				current.Token = maskString(value)
			}
		}
	}

	if current != nil {
		creds = append(creds, *current)
	}

	return creds
}

// Parse SSO profiles from ~/.aws/config
func loadSSOProfiles() []credentialItem {
	home, err := getHomeDir()
	if err != nil {
		return nil
	}

	configPath := filepath.Join(home, ".aws", "config")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")

	// First pass: Parse sso-session blocks to get start URLs
	ssoSessions := make(map[string]string) // sessionName -> startURL
	var currentSession string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[sso-session ") && strings.HasSuffix(line, "]") {
			currentSession = line[13 : len(line)-1] // Extract session name
		} else if currentSession != "" && strings.HasPrefix(line, "sso_start_url") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				ssoSessions[currentSession] = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSession = "" // End of session block
		}
	}

	// Second pass: Parse profiles
	var profiles []credentialItem
	var current *credentialItem

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			// Save previous if it was SSO
			if current != nil && current.IsSSO {
				// Associate start URL from session
				if url, ok := ssoSessions[current.SSOSession]; ok {
					current.StartURL = url
				}
				profiles = append(profiles, *current)
			}

			content := line[1 : len(line)-1]
			// Skip sso-session blocks
			if strings.HasPrefix(content, "sso-session ") {
				current = nil
				continue
			}

			profileName := strings.TrimPrefix(content, "profile ")
			profileName = strings.TrimSpace(profileName)
			current = &credentialItem{ProfileName: profileName, IsSSO: false}
		} else if current != nil && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "sso_session":
				current.SSOSession = value
				current.IsSSO = true
			case "sso_account_id":
				current.AccountID = value
				current.IsSSO = true
			case "sso_role_name":
				current.RoleName = value
				current.IsSSO = true
			case "region":
				current.Region = value
			}
		}
	}

	if current != nil && current.IsSSO {
		if url, ok := ssoSessions[current.SSOSession]; ok {
			current.StartURL = url
		}
		profiles = append(profiles, *current)
	}

	return profiles
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

	// Main Menu
	mainMenuItems := []list.Item{
		menuItem{"Gerenciar Clusters", "Adicionar, editar ou remover clusters EKS", viewClusterList},
		menuItem{"Credenciais Estáticas", "Gerenciar access key / secret key", viewStaticCredsList},
		menuItem{"Perfis SSO", "Gerenciar perfis AWS SSO", viewSSOProfilesList},
	}
	mainMenu := list.New(mainMenuItems, list.NewDefaultDelegate(), 0, 0)
	mainMenu.Title = "Kboot - Menu Principal"
	mainMenu.SetShowTitle(false)
	mainMenu.SetShowHelp(true)
	mainMenu.SetFilteringEnabled(false)

	// Cluster List
	clusterItems := make([]list.Item, len(cfg.Clusters))
	for i, c := range cfg.Clusters {
		clusterItems[i] = clusterItem{c}
	}
	clusterList := list.New(clusterItems, list.NewDefaultDelegate(), 0, 0)
	clusterList.Title = "Clusters"
	clusterList.SetShowTitle(false)
	clusterList.SetShowHelp(true)
	clusterList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keyAdd, keyEdit, keyDuplicate, keyDelete, keyBack}
	}

	// Load credentials
	staticCreds := loadStaticCredentials()
	ssoProfiles := loadSSOProfiles()

	// Credential list (will be populated based on view)
	credList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	credList.SetShowTitle(false)
	credList.SetShowHelp(true)
	credList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keyAdd, keyEdit, keyDuplicate, keyDelete, keyBack}
	}

	return &managerModel{
		view:        viewMainMenu,
		mainMenu:    mainMenu,
		list:        clusterList,
		credList:    credList,
		config:      cfg,
		path:        path,
		editIdx:     -1,
		authEditIdx: -1,
		staticCreds: staticCreds,
		ssoProfiles: ssoProfiles,
	}, nil
}

func (m managerModel) Init() tea.Cmd {
	return nil
}

func (m managerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle window resize globally
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		h, v := docStyle.GetFrameSize()
		availableH := msg.Height - headerHeight - footerHeight - v
		if availableH < 1 {
			availableH = 1
		}
		m.mainMenu.SetSize(msg.Width-h, availableH)
		m.list.SetSize(msg.Width-h, availableH)
		m.credList.SetSize(msg.Width-h, availableH)
	}

	switch m.view {
	case viewMainMenu:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "q" {
				m.quitting = true
				return m, tea.Quit
			}
			if msg.String() == "enter" {
				if item, ok := m.mainMenu.SelectedItem().(menuItem); ok {
					m.view = item.value
					m.status = ""
					// Refresh credential lists when entering
					if item.value == viewStaticCredsList {
						m.staticCreds = loadStaticCredentials()
						items := make([]list.Item, len(m.staticCreds))
						for i, c := range m.staticCreds {
							items[i] = c
						}
						m.credList.SetItems(items)
					} else if item.value == viewSSOProfilesList {
						m.ssoProfiles = loadSSOProfiles()
						items := make([]list.Item, len(m.ssoProfiles))
						for i, c := range m.ssoProfiles {
							items[i] = c
						}
						m.credList.SetItems(items)
					}
					return m, nil
				}
			}
		}
		m.mainMenu, cmd = m.mainMenu.Update(msg)
		return m, cmd

	case viewClusterList:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if m.list.FilterState() == list.Filtering {
				m.list, cmd = m.list.Update(msg)
				return m, cmd
			}

			if msg.Type == tea.KeyEsc || msg.String() == "backspace" {
				m.view = viewMainMenu
				m.status = ""
				return m, nil
			}

			if key.Matches(msg, keyAdd) {
				m.clusterFormData = &ClusterConfig{}
				m.form = newClusterForm(m.clusterFormData)
				m.editIdx = -1
				m.view = viewClusterAddForm
				return m, m.form.Init()
			}

			if key.Matches(msg, keyDelete) {
				if len(m.config.Clusters) == 0 {
					m.status = "Nenhum cluster para deletar"
					return m, nil
				}
				idx := m.list.Index()
				if idx >= 0 && idx < len(m.config.Clusters) {
					m.pendingDeleteIdx = idx
					m.pendingDeleteName = m.config.Clusters[idx].Alias
					val := false
					m.confirmDelete = &val
					m.form = huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().
								Title(fmt.Sprintf("Deletar cluster '%s'?", m.pendingDeleteName)).
								Affirmative("Sim, deletar").
								Negative("Cancelar").
								Value(m.confirmDelete),
						),
					)
					m.view = viewClusterDeleteConfirm
					return m, m.form.Init()
				}
			}

			if key.Matches(msg, keyEdit) {
				if len(m.config.Clusters) == 0 {
					m.status = "Nenhum cluster para editar"
					return m, nil
				}
				idx := m.list.Index()
				if idx >= 0 && idx < len(m.config.Clusters) {
					cluster := m.config.Clusters[idx]
					m.clusterFormData = &ClusterConfig{
						Alias:       cluster.Alias,
						ClusterName: cluster.Name,
						Region:      cluster.Region,
						Profile:     cluster.Profile,
					}
					m.form = newClusterForm(m.clusterFormData)
					m.editIdx = idx
					m.view = viewClusterEditForm
					return m, m.form.Init()
				}
			}

			// Duplicate - copy all data but add suffix to name
			if key.Matches(msg, keyDuplicate) {
				if len(m.config.Clusters) == 0 {
					m.status = "Nenhum cluster para duplicar"
					return m, nil
				}
				idx := m.list.Index()
				if idx >= 0 && idx < len(m.config.Clusters) {
					cluster := m.config.Clusters[idx]
					m.clusterFormData = &ClusterConfig{
						Alias:       cluster.Alias + "-copy",
						ClusterName: cluster.Name,
						Region:      cluster.Region,
						Profile:     cluster.Profile,
					}
					m.form = newClusterForm(m.clusterFormData)
					m.editIdx = -1 // It's a new item, not edit
					m.view = viewClusterAddForm
					m.status = "Duplicando cluster..."
					return m, m.form.Init()
				}
			}
		}
		m.list, cmd = m.list.Update(msg)
		return m, cmd

	case viewClusterAddForm, viewClusterEditForm:
		if msg, ok := msg.(tea.KeyMsg); ok {
			if msg.Type == tea.KeyEsc {
				m.view = viewClusterList
				m.status = "Cancelado"
				return m, nil
			}
		}

		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateCompleted {
			newCluster := Cluster{
				Alias:   strings.TrimSpace(m.clusterFormData.Alias),
				Name:    strings.TrimSpace(m.clusterFormData.ClusterName),
				Region:  strings.TrimSpace(m.clusterFormData.Region),
				Profile: strings.TrimSpace(m.clusterFormData.Profile),
			}
			if newCluster.Region == "" {
				newCluster.Region = "us-east-1"
			}

			if m.view == viewClusterEditForm && m.editIdx >= 0 {
				m.config.Clusters[m.editIdx] = newCluster
				m.status = fmt.Sprintf("Atualizado '%s'", newCluster.Alias)
			} else {
				m.config.Clusters = append(m.config.Clusters, newCluster)
				m.status = fmt.Sprintf("Adicionado '%s'", newCluster.Alias)
			}
			saveConfigToFile(m.path, m.config)

			items := make([]list.Item, len(m.config.Clusters))
			for i, c := range m.config.Clusters {
				items[i] = clusterItem{c}
			}
			m.list.SetItems(items)
			m.view = viewClusterList
			return m, nil
		}

		if m.form.State == huh.StateAborted {
			m.view = viewClusterList
			m.status = "Cancelado"
			return m, nil
		}

		return m, cmd

	case viewClusterDeleteConfirm:
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateCompleted {
			if *m.confirmDelete {
				// Actually delete
				m.config.Clusters = append(m.config.Clusters[:m.pendingDeleteIdx], m.config.Clusters[m.pendingDeleteIdx+1:]...)
				m.status = fmt.Sprintf("Deletado '%s'", m.pendingDeleteName)
				saveConfigToFile(m.path, m.config)
				items := make([]list.Item, len(m.config.Clusters))
				for i, c := range m.config.Clusters {
					items[i] = clusterItem{c}
				}
				m.list.SetItems(items)
			} else {
				m.status = "Cancelado"
			}
			m.view = viewClusterList
			return m, nil
		}

		if m.form.State == huh.StateAborted {
			m.view = viewClusterList
			m.status = "Cancelado"
			return m, nil
		}

		return m, cmd

	case viewStaticCredsList:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEsc || msg.String() == "backspace" {
				m.view = viewMainMenu
				m.status = ""
				return m, nil
			}

			if key.Matches(msg, keyAdd) {
				m.authFormData = &AuthFormData{}
				m.form = newStaticCredentialForm(m.authFormData)
				m.authEditIdx = -1
				m.view = viewStaticAddForm
				return m, m.form.Init()
			}

			if key.Matches(msg, keyDelete) {
				if len(m.staticCreds) == 0 {
					m.status = "Nenhuma credencial para deletar"
					return m, nil
				}
				idx := m.credList.Index()
				if idx >= 0 && idx < len(m.staticCreds) {
					m.pendingDeleteIdx = idx
					m.pendingDeleteName = m.staticCreds[idx].ProfileName
					val := false
					m.confirmDelete = &val
					m.form = huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().
								Title(fmt.Sprintf("Deletar credencial '%s'?", m.pendingDeleteName)).
								Affirmative("Sim, deletar").
								Negative("Cancelar").
								Value(m.confirmDelete),
						),
					)
					m.view = viewStaticDeleteConfirm
					return m, m.form.Init()
				}
			}

			// Edit - keep profile name, clear sensitive data
			if key.Matches(msg, keyEdit) {
				if len(m.staticCreds) == 0 {
					m.status = "Nenhuma credencial para editar"
					return m, nil
				}
				idx := m.credList.Index()
				if idx >= 0 && idx < len(m.staticCreds) {
					cred := m.staticCreds[idx]
					// Pre-fill profile name, but clear sensitive fields (user must re-enter)
					m.authFormData = &AuthFormData{
						Profile: cred.ProfileName,
						// AccessKey, SecretKey, Token left empty - user must re-enter
					}
					m.form = newStaticCredentialForm(m.authFormData)
					m.authEditIdx = idx
					m.view = viewStaticEditForm
					return m, m.form.Init()
				}
			}

			// Duplicate - copy profile name with suffix
			if key.Matches(msg, keyDuplicate) {
				if len(m.staticCreds) == 0 {
					m.status = "Nenhuma credencial para duplicar"
					return m, nil
				}
				idx := m.credList.Index()
				if idx >= 0 && idx < len(m.staticCreds) {
					cred := m.staticCreds[idx]
					m.authFormData = &AuthFormData{
						Profile: cred.ProfileName + "-copy",
						// Sensitive data must be entered again
					}
					m.form = newStaticCredentialForm(m.authFormData)
					m.authEditIdx = -1
					m.view = viewStaticAddForm
					m.status = "Duplicando credencial..."
					return m, m.form.Init()
				}
			}
		}
		m.credList, cmd = m.credList.Update(msg)
		return m, cmd

	case viewStaticAddForm, viewStaticEditForm:
		if msg, ok := msg.(tea.KeyMsg); ok {
			if msg.Type == tea.KeyEsc {
				m.view = viewStaticCredsList
				m.status = "Cancelado"
				return m, nil
			}
		}

		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateCompleted {
			profile := strings.TrimSpace(m.authFormData.Profile)
			accessKey := strings.TrimSpace(m.authFormData.AccessKey)
			secretKey := strings.TrimSpace(m.authFormData.SecretKey)
			token := strings.TrimSpace(m.authFormData.Token)

			// If editing, delete the old profile first
			if m.view == viewStaticEditForm && m.authEditIdx >= 0 {
				oldProfileName := m.staticCreds[m.authEditIdx].ProfileName
				deleteStaticCredential(oldProfileName)
			}

			content := fmt.Sprintf("\n[%s]\naws_access_key_id = %s\naws_secret_access_key = %s\n", profile, accessKey, secretKey)
			if token != "" {
				content += fmt.Sprintf("aws_session_token = %s\n", token)
			}

			home, _ := getHomeDir()
			credPath := filepath.Join(home, ".aws", "credentials")
			appendToFile(credPath, content)

			if m.view == viewStaticEditForm {
				m.status = fmt.Sprintf("Atualizado '%s'", profile)
			} else {
				m.status = fmt.Sprintf("Adicionado '%s'", profile)
			}

			// Refresh list
			m.staticCreds = loadStaticCredentials()
			items := make([]list.Item, len(m.staticCreds))
			for i, c := range m.staticCreds {
				items[i] = c
			}
			m.credList.SetItems(items)
			m.view = viewStaticCredsList
			return m, nil
		}

		if m.form.State == huh.StateAborted {
			m.view = viewStaticCredsList
			m.status = "Cancelado"
			return m, nil
		}

		return m, cmd

	case viewStaticDeleteConfirm:
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateCompleted {
			if *m.confirmDelete {
				profileName := m.staticCreds[m.pendingDeleteIdx].ProfileName
				if deleteStaticCredential(profileName) {
					m.status = fmt.Sprintf("Deletado '%s'", profileName)
					m.staticCreds = loadStaticCredentials()
					items := make([]list.Item, len(m.staticCreds))
					for i, c := range m.staticCreds {
						items[i] = c
					}
					m.credList.SetItems(items)
				} else {
					m.status = "Erro ao deletar credencial"
				}
			} else {
				m.status = "Cancelado"
			}
			m.view = viewStaticCredsList
			return m, nil
		}

		if m.form.State == huh.StateAborted {
			m.view = viewStaticCredsList
			m.status = "Cancelado"
			return m, nil
		}

		return m, cmd

	case viewSSOProfilesList:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEsc || msg.String() == "backspace" {
				m.view = viewMainMenu
				m.status = ""
				return m, nil
			}

			if key.Matches(msg, keyAdd) {
				m.authFormData = &AuthFormData{SessionName: "my-sso"}
				m.form = newSSOProfileForm(m.authFormData)
				m.authEditIdx = -1
				m.view = viewSSOAddForm
				return m, m.form.Init()
			}

			if key.Matches(msg, keyDelete) {
				if len(m.ssoProfiles) == 0 {
					m.status = "Nenhum perfil para deletar"
					return m, nil
				}
				idx := m.credList.Index()
				if idx >= 0 && idx < len(m.ssoProfiles) {
					m.pendingDeleteIdx = idx
					m.pendingDeleteName = m.ssoProfiles[idx].ProfileName
					val := false
					m.confirmDelete = &val
					m.form = huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().
								Title(fmt.Sprintf("Deletar perfil SSO '%s'?", m.pendingDeleteName)).
								Affirmative("Sim, deletar").
								Negative("Cancelar").
								Value(m.confirmDelete),
						),
					)
					m.view = viewSSODeleteConfirm
					return m, m.form.Init()
				}
			}

			// Edit - keep profile name and session, user re-enters SSO details
			if key.Matches(msg, keyEdit) {
				if len(m.ssoProfiles) == 0 {
					m.status = "Nenhum perfil para editar"
					return m, nil
				}
				idx := m.credList.Index()
				if idx >= 0 && idx < len(m.ssoProfiles) {
					cred := m.ssoProfiles[idx]
					m.authFormData = &AuthFormData{
						Profile:     cred.ProfileName,
						SessionName: cred.SSOSession,
						Region:      cred.Region,
						AccountID:   cred.AccountID,
						RoleName:    cred.RoleName,
						StartURL:    cred.StartURL, // Now populated from sso-session block
					}
					m.form = newSSOProfileForm(m.authFormData)
					m.authEditIdx = idx
					m.view = viewSSOEditForm
					return m, m.form.Init()
				}
			}

			// Duplicate - copy all data with suffix on profile name
			if key.Matches(msg, keyDuplicate) {
				if len(m.ssoProfiles) == 0 {
					m.status = "Nenhum perfil para duplicar"
					return m, nil
				}
				idx := m.credList.Index()
				if idx >= 0 && idx < len(m.ssoProfiles) {
					cred := m.ssoProfiles[idx]
					m.authFormData = &AuthFormData{
						Profile:     cred.ProfileName + "-copy",
						SessionName: cred.SSOSession,
						Region:      cred.Region,
						AccountID:   cred.AccountID,
						RoleName:    cred.RoleName,
						StartURL:    cred.StartURL,
					}
					m.form = newSSOProfileForm(m.authFormData)
					m.authEditIdx = -1
					m.view = viewSSOAddForm
					m.status = "Duplicando perfil..."
					return m, m.form.Init()
				}
			}
		}
		m.credList, cmd = m.credList.Update(msg)
		return m, cmd

	case viewSSOAddForm, viewSSOEditForm:
		if msg, ok := msg.(tea.KeyMsg); ok {
			if msg.Type == tea.KeyEsc {
				m.view = viewSSOProfilesList
				m.status = "Cancelado"
				return m, nil
			}
		}

		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateCompleted {
			profile := strings.TrimSpace(m.authFormData.Profile)
			sessionName := strings.TrimSpace(m.authFormData.SessionName)
			if sessionName == "" {
				sessionName = "my-sso"
			}
			url := strings.TrimSpace(m.authFormData.StartURL)
			region := strings.TrimSpace(m.authFormData.Region)
			accID := strings.TrimSpace(m.authFormData.AccountID)
			roleName := strings.TrimSpace(m.authFormData.RoleName)

			// If editing, delete the old profile first
			if m.view == viewSSOEditForm && m.authEditIdx >= 0 {
				oldProfileName := m.ssoProfiles[m.authEditIdx].ProfileName
				deleteSSOProfile(oldProfileName)
			}

			home, _ := getHomeDir()
			configPath := filepath.Join(home, ".aws", "config")

			// Check if sso-session exists
			existingContent, _ := os.ReadFile(configPath)
			sessionHeader := fmt.Sprintf("[sso-session %s]", sessionName)
			if !strings.Contains(string(existingContent), sessionHeader) {
				sessionBlock := fmt.Sprintf("\n%s\nsso_start_url = %s\nsso_region = %s\nsso_registration_scopes = sso:account:access\n",
					sessionHeader, url, region)
				appendToFile(configPath, sessionBlock)
			}

			profileBlock := fmt.Sprintf("\n[profile %s]\nsso_session = %s\nsso_account_id = %s\nsso_role_name = %s\nregion = %s\n",
				profile, sessionName, accID, roleName, region)
			appendToFile(configPath, profileBlock)

			if m.view == viewSSOEditForm {
				m.status = fmt.Sprintf("Atualizado SSO '%s'", profile)
			} else {
				m.status = fmt.Sprintf("Adicionado SSO '%s'", profile)
			}

			// Refresh list
			m.ssoProfiles = loadSSOProfiles()
			items := make([]list.Item, len(m.ssoProfiles))
			for i, c := range m.ssoProfiles {
				items[i] = c
			}
			m.credList.SetItems(items)
			m.view = viewSSOProfilesList
			return m, nil
		}

		if m.form.State == huh.StateAborted {
			m.view = viewSSOProfilesList
			m.status = "Cancelado"
			return m, nil
		}

		return m, cmd

	case viewSSODeleteConfirm:
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateCompleted {
			if *m.confirmDelete {
				profileName := m.ssoProfiles[m.pendingDeleteIdx].ProfileName
				if deleteSSOProfile(profileName) {
					m.status = fmt.Sprintf("Deletado '%s'", profileName)
					m.ssoProfiles = loadSSOProfiles()
					items := make([]list.Item, len(m.ssoProfiles))
					for i, c := range m.ssoProfiles {
						items[i] = c
					}
					m.credList.SetItems(items)
				} else {
					m.status = "Erro ao deletar perfil"
				}
			} else {
				m.status = "Cancelado"
			}
			m.view = viewSSOProfilesList
			return m, nil
		}

		if m.form.State == huh.StateAborted {
			m.view = viewSSOProfilesList
			m.status = "Cancelado"
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

	var title string
	var content string

	switch m.view {
	case viewMainMenu:
		title = "Kboot - Menu Principal"
		content = docStyle.Render(m.mainMenu.View())
	case viewClusterList:
		title = "Clusters EKS"
		content = docStyle.Render(m.list.View())
	case viewClusterAddForm:
		title = "Adicionar Cluster"
		content = docStyle.Render(m.form.View())
	case viewClusterEditForm:
		title = "Editar Cluster"
		content = docStyle.Render(m.form.View())
	case viewStaticCredsList:
		title = "Credenciais Estáticas (~/.aws/credentials)"
		content = docStyle.Render(m.credList.View())
	case viewStaticAddForm:
		title = "Adicionar Credencial"
		content = docStyle.Render(m.form.View())
	case viewStaticEditForm:
		title = "Editar Credencial"
		content = docStyle.Render(m.form.View())
	case viewSSOProfilesList:
		title = "Perfis SSO (~/.aws/config)"
		content = docStyle.Render(m.credList.View())
	case viewSSOAddForm:
		title = "Adicionar Perfil SSO"
		content = docStyle.Render(m.form.View())
	case viewSSOEditForm:
		title = "Editar Perfil SSO"
		content = docStyle.Render(m.form.View())
	case viewClusterDeleteConfirm:
		title = "Confirmar Exclusão"
		content = docStyle.Render(m.form.View())
	case viewStaticDeleteConfirm:
		title = "Confirmar Exclusão"
		content = docStyle.Render(m.form.View())
	case viewSSODeleteConfirm:
		title = "Confirmar Exclusão"
		content = docStyle.Render(m.form.View())
	default:
		title = "Kboot"
		content = ""
	}

	header := titleStyle.Render(title)
	s := lipgloss.JoinVertical(lipgloss.Left, header, content)

	statusVal := m.status
	if statusVal == "" {
		statusVal = " "
	}
	status := statusStyle.Render(statusVal)
	if m.err != nil {
		status = errStyle.Render(fmt.Sprintf("Erro: %v", m.err))
	}

	return lipgloss.JoinVertical(lipgloss.Left, s, status)
}

// Form builders
func newStaticCredentialForm(data *AuthFormData) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Nome do Perfil").
				Value(&data.Profile).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("nome não pode ser vazio")
					}
					return nil
				}),

			huh.NewInput().
				Title("AWS Access Key ID").
				Value(&data.AccessKey).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("access key não pode ser vazia")
					}
					return nil
				}),

			huh.NewInput().
				Title("AWS Secret Access Key").
				Password(true).
				Value(&data.SecretKey).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("secret key não pode ser vazia")
					}
					return nil
				}),

			huh.NewInput().
				Title("Session Token (opcional)").
				Value(&data.Token),

			huh.NewNote().
				Title("Navegação").
				Description("Tab → próximo | Shift+Tab → anterior | Enter → confirmar | Esc → cancelar"),
		),
	)
}

func newSSOProfileForm(data *AuthFormData) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Nome do Perfil").
				Value(&data.Profile).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("nome não pode ser vazio")
					}
					return nil
				}),

			huh.NewInput().
				Title("SSO Session Name").
				Description("Default: my-sso").
				Value(&data.SessionName),

			huh.NewInput().
				Title("SSO Start URL").
				Value(&data.StartURL).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("URL não pode ser vazia")
					}
					return nil
				}),

			huh.NewInput().
				Title("Região").
				Value(&data.Region).
				Suggestions([]string{"us-east-1", "us-west-2", "eu-west-1", "sa-east-1"}).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("região não pode ser vazia")
					}
					return nil
				}),

			huh.NewInput().
				Title("Account ID").
				Value(&data.AccountID).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("account ID não pode ser vazio")
					}
					return nil
				}),

			huh.NewInput().
				Title("Role Name").
				Value(&data.RoleName).
				Validate(func(str string) error {
					if strings.TrimSpace(str) == "" {
						return fmt.Errorf("role name não pode ser vazio")
					}
					return nil
				}),

			huh.NewNote().
				Title("Navegação").
				Description("Tab → próximo | Shift+Tab → anterior | Enter → confirmar | Esc → cancelar"),
		),
	)
}

// Delete a static credential profile from ~/.aws/credentials
func deleteStaticCredential(profileName string) bool {
	home, err := getHomeDir()
	if err != nil {
		return false
	}

	credPath := filepath.Join(home, ".aws", "credentials")
	content, err := os.ReadFile(credPath)
	if err != nil {
		return false
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	skipUntilNextSection := false
	targetHeader := fmt.Sprintf("[%s]", profileName)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if trimmed == targetHeader {
				skipUntilNextSection = true
				continue
			} else {
				skipUntilNextSection = false
			}
		}
		if !skipUntilNextSection {
			result = append(result, line)
		}
	}

	return os.WriteFile(credPath, []byte(strings.Join(result, "\n")), 0600) == nil
}

// Delete an SSO profile from ~/.aws/config
func deleteSSOProfile(profileName string) bool {
	home, err := getHomeDir()
	if err != nil {
		return false
	}

	configPath := filepath.Join(home, ".aws", "config")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	skipUntilNextSection := false
	targetHeader := fmt.Sprintf("[profile %s]", profileName)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if trimmed == targetHeader {
				skipUntilNextSection = true
				continue
			} else {
				skipUntilNextSection = false
			}
		}
		if !skipUntilNextSection {
			result = append(result, line)
		}
	}

	return os.WriteFile(configPath, []byte(strings.Join(result, "\n")), 0600) == nil
}

// Entry point
func runManager() {
	m, err := initialManagerModel()
	if err != nil {
		fatal("Não foi possível iniciar o dashboard: %v", err)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fatal("Erro ao executar dashboard: %v", err)
	}
}
