package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *model) handleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.pendingPermission != nil {
		switch msg.String() {
		case "y", "enter":
			m.resolvePermission(nil)
			return true, nil
		case "n", "esc":
			m.resolvePermission(fmt.Errorf("permission denied"))
			return true, nil
		case "ctrl+c":
			m.resolvePermission(fmt.Errorf("permission denied"))
			m.stop()
			return true, tea.Quit
		default:
			return true, nil
		}
	}

	switch msg.String() {
	case "ctrl+c":
		m.stop()
		return true, tea.Quit
	case "ctrl+o":
		m.showHistory = !m.showHistory
		return true, nil
	case "ctrl+t":
		m.showLatestDetails = !m.showLatestDetails
		return true, nil
	case "esc":
		if m.running && m.runCancel != nil {
			m.runCancel()
			m.statusText = "Interrupt requested"
			return true, nil
		}
		return true, nil
	case "pgup":
		m.viewport.PageUp()
		return true, nil
	case "pgdown":
		m.viewport.PageDown()
		return true, nil
	case "home":
		m.viewport.GotoTop()
		return true, nil
	case "end":
		m.viewport.GotoBottom()
		return true, nil
	case "ctrl+l":
		m.resetInputBox()
		return true, nil
	case "ctrl+j", "shift+enter", "alt+enter":
		m.inputBox.InsertString("\n")
		return true, nil
	case "enter":
		if m.running {
			m.statusText = "Agent is already running"
			return true, nil
		}
		prompt := strings.TrimSpace(m.inputBox.Value())
		if prompt == "" {
			return true, nil
		}
		m.resetInputBox()
		m.startRun(prompt)
		return true, nil
	}

	return false, nil
}

func (m *model) resetInputBox() {
	m.inputBox.Reset()
	m.updateInputBoxLayout(max(10, m.viewport.Width()+2))
	m.inputBox.Focus()
}

func (m *model) updateInputBoxLayout(innerWidth int) {
	contentWidth := max(8, innerWidth-2)
	m.inputBox.SetWidth(contentWidth)

	lineCount := measureInputHeight(m.inputBox.Value(), contentWidth)
	maxHeight := max(1, min(10, max(1, m.height/3)))
	if lineCount > maxHeight {
		lineCount = maxHeight
	}
	m.inputBox.SetHeight(lineCount)
}
