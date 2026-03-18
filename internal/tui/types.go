package tui

import (
	"context"
	"sync"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"minioc/internal/agent"
	"minioc/internal/llm"
	"minioc/internal/safety"
	"minioc/internal/session"
)

type Config struct {
	RepoRoot    string
	Workdir     string
	Model       string
	Prompt      string
	Loop        agent.Loop
	Session     *session.Session
	AutoApprove bool
}

type sceneKind int

const (
	sceneLanding sceneKind = iota
	sceneTools
	sceneAssistant
)

const defaultPrompt = "audit and improve test coverage"

var spinnerFrames = []string{"-", "\\", "|", "/"}

type tickMsg time.Time

type deltaEvent struct {
	Text string
	Done bool
}

type assistantDeltaMsg struct {
	Text string
}

type assistantDoneMsg struct{}

type toolCallMsg struct {
	Call llm.ToolCall
}

type toolResultMsg struct {
	Call   llm.ToolCall
	Status string
	Output string
}

type runFinishedMsg struct {
	Answer string
	Err    error
}

type permissionRequestMsg struct {
	Kind    string
	Summary string
	Reply   chan error
}

type chatEntry struct {
	Role session.Role
	Text string
}

type turnEventKind int

const (
	turnEventAssistant turnEventKind = iota
	turnEventTool
)

type turnEvent struct {
	Kind turnEventKind
	Text string
	Tool toolEntry
}

type turnEntry struct {
	User   string
	Events []turnEvent
}

type toolEntry struct {
	ID     string
	Name   string
	Target string
	Status string
	Output string
}

type permissionPrompt struct {
	Kind    string
	Summary string
	Reply   chan error
}

type model struct {
	config            Config
	styles            styles
	viewport          viewport.Model
	width             int
	height            int
	scene             sceneKind
	frame             int
	compact           bool
	compactW          int
	compactH          int
	inputBox          textarea.Model
	cursorBlink       bool
	loop              agent.Loop
	sess              *session.Session
	permissions       *safety.PermissionManager
	externalEvents    chan tea.Msg
	deltaEvents       chan deltaEvent
	shutdown          chan struct{}
	shutdownOnce      sync.Once
	initialPrompt     string
	startedInitialRun bool
	running           bool
	runCancel         context.CancelFunc
	statusText        string
	lastError         string
	turns             []turnEntry
	transcript        []chatEntry
	tools             []toolEntry
	assistantDraft    string
	pendingPermission *permissionPrompt
	showHistory       bool
	showLatestDetails bool
}

type styles struct {
	screen           lipgloss.Style
	screenFill       lipgloss.Style
	logo             lipgloss.Style
	title            lipgloss.Style
	subtitle         lipgloss.Style
	meta             lipgloss.Style
	rule             lipgloss.Style
	footer           lipgloss.Style
	footerAccent     lipgloss.Style
	footerMuted      lipgloss.Style
	promptFill       lipgloss.Style
	promptPrefix     lipgloss.Style
	promptLine       lipgloss.Style
	assistantMarker  lipgloss.Style
	assistantLabel   lipgloss.Style
	assistant        lipgloss.Style
	toolLabel        lipgloss.Style
	toolPath         lipgloss.Style
	toolRead         lipgloss.Style
	toolBash         lipgloss.Style
	toolStatusRun    lipgloss.Style
	toolStatusDone   lipgloss.Style
	toolStatusFail   lipgloss.Style
	toolStatusMuted  lipgloss.Style
	toolMeta         lipgloss.Style
	running          lipgloss.Style
	dim              lipgloss.Style
	hint             lipgloss.Style
	errorText        lipgloss.Style
	warningText      lipgloss.Style
	composerFill     lipgloss.Style
	inputPrompt      lipgloss.Style
	inputText        lipgloss.Style
	inputPlaceholder lipgloss.Style
	inputCursor      lipgloss.Style
	compactTitle     lipgloss.Style
	compactBody      lipgloss.Style
}
