package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"minioc/internal/safety"
	"minioc/internal/session"
)

func Run(cfg Config) error {
	m := newModel(cfg)
	p := tea.NewProgram(m)
	_, err := p.Run()
	m.stop()
	return err
}

func newModel(cfg Config) *model {
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = "gpt-5.4"
	}
	if cfg.Session == nil {
		cfg.Session = session.New(cfg.RepoRoot, cfg.Workdir, cfg.Model)
	}

	vp := viewport.New()
	vp.SoftWrap = true
	vp.FillHeight = false
	vp.MouseWheelEnabled = true
	vp.Style = lipgloss.NewStyle().Background(lipgloss.Color("#052B33")).Foreground(lipgloss.Color("#A8B7B8"))
	inputBox := textarea.New()
	inputBox.Prompt = "> "
	inputBox.Placeholder = "Type a prompt and press enter..."
	inputBox.ShowLineNumbers = false
	inputBox.CharLimit = 12000
	inputBox.SetHeight(1)
	inputBox.SetWidth(40)
	inputBox.Focus()

	m := &model{
		config:         cfg,
		styles:         newStyles(),
		viewport:       vp,
		inputBox:       inputBox,
		scene:          sceneAssistant,
		cursorBlink:    true,
		loop:           cfg.Loop,
		sess:           cfg.Session,
		externalEvents: make(chan tea.Msg, 256),
		deltaEvents:    make(chan deltaEvent, 256),
		shutdown:       make(chan struct{}),
		initialPrompt:  strings.TrimSpace(cfg.Prompt),
		statusText:     "Ready",
	}
	inputStyles := m.inputBox.Styles()
	inputStyles.Focused.Base = lipgloss.NewStyle().Background(lipgloss.Color("#052B33")).Foreground(lipgloss.Color("#C6D1D2"))
	inputStyles.Focused.Text = lipgloss.NewStyle().Background(lipgloss.Color("#052B33")).Foreground(lipgloss.Color("#C6D1D2"))
	inputStyles.Focused.Prompt = lipgloss.NewStyle().Background(lipgloss.Color("#052B33")).Foreground(lipgloss.Color("#5E8F95")).Bold(true)
	inputStyles.Focused.Placeholder = lipgloss.NewStyle().Background(lipgloss.Color("#052B33")).Foreground(lipgloss.Color("#6D868A"))
	inputStyles.Focused.EndOfBuffer = lipgloss.NewStyle().Background(lipgloss.Color("#052B33")).Foreground(lipgloss.Color("#35565B"))
	inputStyles.Focused.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color("#052B33"))
	inputStyles.Focused.CursorLineNumber = lipgloss.NewStyle().Background(lipgloss.Color("#052B33"))
	inputStyles.Blurred = inputStyles.Focused
	inputStyles.Cursor.Color = lipgloss.Color("#C8D2D3")
	inputStyles.Cursor.Blink = true
	m.inputBox.SetStyles(inputStyles)
	m.permissions = safety.NewCallbackPermissionManager(cfg.AutoApprove, m.requestPermission)
	m.hydrateFromSession()
	go m.deltaPump()
	return m
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), waitExternalCmd(m.externalEvents), m.inputBox.Focus())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncLayout()
		if !m.startedInitialRun && m.initialPrompt != "" {
			m.startedInitialRun = true
			prompt := m.initialPrompt
			m.initialPrompt = ""
			m.startRun(prompt)
		}
		return m, nil

	case tickMsg:
		m.frame = (m.frame + 1) % len(spinnerFrames)
		m.cursorBlink = !m.cursorBlink
		cmds = append(cmds, tickCmd())

	case assistantDeltaMsg:
		m.appendAssistantDelta(msg.Text)
		cmds = append(cmds, waitExternalCmd(m.externalEvents))

	case assistantDoneMsg:
		m.commitAssistantDraft()
		cmds = append(cmds, waitExternalCmd(m.externalEvents))

	case toolCallMsg:
		m.recordToolCall(msg.Call)
		cmds = append(cmds, waitExternalCmd(m.externalEvents))

	case toolResultMsg:
		m.recordToolResult(msg.Call, msg.Status, msg.Output)
		cmds = append(cmds, waitExternalCmd(m.externalEvents))

	case permissionRequestMsg:
		m.pendingPermission = &permissionPrompt{Kind: msg.Kind, Summary: msg.Summary, Reply: msg.Reply}
		m.statusText = "Waiting for permission approval"
		cmds = append(cmds, waitExternalCmd(m.externalEvents))

	case runFinishedMsg:
		m.finishRun(msg)
		cmds = append(cmds, waitExternalCmd(m.externalEvents))

	case tea.KeyPressMsg:
		if done, cmd := m.handleKey(msg); done {
			m.syncLayout()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(mouseMsg)
		cmds = append(cmds, cmd)
	}

	if m.pendingPermission == nil {
		var cmd tea.Cmd
		m.inputBox, cmd = m.inputBox.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.syncLayout()
	return m, tea.Batch(cmds...)
}

func (m *model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		v := tea.NewView("loading tui...")
		v.AltScreen = true
		v.WindowTitle = "minioc TUI"
		return v
	}

	if m.compact {
		return m.renderCompactView()
	}

	innerWidth := max(32, m.width-4)
	header := m.renderHeader(innerWidth)
	rule := m.renderRule(innerWidth)
	content := m.viewport.View()
	composer := m.renderComposer(innerWidth)
	footer := m.renderFooter(innerWidth)
	body := lipgloss.JoinVertical(lipgloss.Left, header, rule, content, rule, composer, rule, footer)
	body = lipgloss.NewStyle().Padding(1, 2).Background(lipgloss.Color("#052B33")).Render(body)
	screen := lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, body, lipgloss.WithWhitespaceStyle(m.styles.screen))

	v := tea.NewView(screen)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "minioc TUI"
	return v
}

func (m *model) syncLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	m.compact = m.width < 72 || m.height < 20
	if m.compact {
		m.compactW = m.width
		m.compactH = m.height
		return
	}

	innerWidth := max(32, m.width-4)
	m.updateInputBoxLayout(innerWidth)
	header := m.renderHeader(innerWidth)
	composer := m.renderComposer(innerWidth)
	footer := m.renderFooter(innerWidth)
	rulesHeight := 3
	paddingHeight := 2
	available := m.height - lipgloss.Height(header) - lipgloss.Height(composer) - lipgloss.Height(footer) - rulesHeight - paddingHeight
	if available < 6 {
		available = 6
	}

	stickBottom := m.viewport.AtBottom()
	m.viewport.SetWidth(innerWidth)
	m.viewport.SetHeight(available)
	m.viewport.SetContent(m.renderSceneContent(innerWidth, available))
	if stickBottom {
		m.viewport.GotoBottom()
	}
}

func (m *model) sceneLabel() string {
	return "session"
}

func (m *model) displayPath() string {
	if strings.TrimSpace(m.sess.Workdir) != "" {
		return m.sess.Workdir
	}
	if strings.TrimSpace(m.sess.RepoRoot) != "" {
		return m.sess.RepoRoot
	}
	if strings.TrimSpace(m.config.Workdir) != "" {
		return m.config.Workdir
	}
	if strings.TrimSpace(m.config.RepoRoot) != "" {
		return m.config.RepoRoot
	}
	return "."
}

func waitExternalCmd(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(140*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
