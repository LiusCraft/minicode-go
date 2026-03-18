package tui

import "charm.land/lipgloss/v2"

func newStyles() styles {
	bg := lipgloss.Color("#052B33")
	fg := lipgloss.Color("#A8B7B8")
	soft := lipgloss.Color("#708A8E")
	accent := lipgloss.Color("#EB7A50")
	coral := lipgloss.Color("#FF6E93")
	green := lipgloss.Color("#2BC97B")
	line := lipgloss.Color("#5E8F95")
	promptBG := lipgloss.Color("#3E484A")

	return styles{
		screen:           lipgloss.NewStyle().Background(bg).Foreground(fg),
		screenFill:       lipgloss.NewStyle().Background(bg),
		logo:             lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true),
		title:            lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#D4DDDD")).Bold(true),
		subtitle:         lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#88A0A4")),
		meta:             lipgloss.NewStyle().Background(bg).Foreground(soft),
		rule:             lipgloss.NewStyle().Background(bg).Foreground(line),
		footer:           lipgloss.NewStyle().Background(bg).Foreground(fg),
		footerAccent:     lipgloss.NewStyle().Background(bg).Foreground(accent),
		footerMuted:      lipgloss.NewStyle().Background(bg).Foreground(soft),
		promptFill:       lipgloss.NewStyle().Background(promptBG),
		promptPrefix:     lipgloss.NewStyle().Background(promptBG).Foreground(lipgloss.Color("#A8BEC1")).Bold(true),
		promptLine:       lipgloss.NewStyle().Background(promptBG).Foreground(lipgloss.Color("#D7E0E0")).Bold(true),
		assistantMarker:  lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#E3EAE9")).Bold(true),
		assistantLabel:   lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#8FA9AD")),
		assistant:        lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#C4D1D1")),
		toolLabel:        lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#D2DCDC")).Bold(true),
		toolPath:         lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#8CA4A8")),
		toolRead:         lipgloss.NewStyle().Background(bg).Foreground(green).Bold(true),
		toolBash:         lipgloss.NewStyle().Background(bg).Foreground(coral).Bold(true),
		toolStatusRun:    lipgloss.NewStyle().Background(lipgloss.Color("#B85D3A")).Foreground(lipgloss.Color("#FBE9DE")).Bold(true),
		toolStatusDone:   lipgloss.NewStyle().Background(lipgloss.Color("#1E6B47")).Foreground(lipgloss.Color("#E5F7EF")).Bold(true),
		toolStatusFail:   lipgloss.NewStyle().Background(lipgloss.Color("#8A2F4F")).Foreground(lipgloss.Color("#FFE9F0")).Bold(true),
		toolStatusMuted:  lipgloss.NewStyle().Background(lipgloss.Color("#3A5156")).Foreground(lipgloss.Color("#D3DEDF")).Bold(true),
		toolMeta:         lipgloss.NewStyle().Background(bg).Foreground(soft),
		running:          lipgloss.NewStyle().Background(bg).Foreground(accent),
		dim:              lipgloss.NewStyle().Background(bg).Foreground(soft),
		hint:             lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#8AA2A6")),
		errorText:        lipgloss.NewStyle().Background(bg).Foreground(coral).Bold(true),
		warningText:      lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#F4C96B")).Bold(true),
		composerFill:     lipgloss.NewStyle().Background(bg),
		inputPrompt:      lipgloss.NewStyle().Background(bg).Foreground(line).Bold(true),
		inputText:        lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#C6D1D2")),
		inputPlaceholder: lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#6D868A")),
		inputCursor:      lipgloss.NewStyle().Background(lipgloss.Color("#C8D2D3")).Foreground(bg).Bold(true),
		compactTitle:     lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#D4DDDD")).Bold(true),
		compactBody:      lipgloss.NewStyle().Background(bg).Foreground(fg),
	}
}
