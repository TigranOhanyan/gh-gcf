package tui

import (
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const noRepos = "No app repos yet."
const noLabels = "No labels yet."
const noApps = "No apps yet."
const noDevelopments = "No developments yet."
const noChecks = "No checks yet."

const pathPastFg = "#E8EEF4"
const pathPastBg = "#3C4F65"
const pathNowFg = "#FFFFFF"
const pathNowBg = "#2F6FED"
const keyFg = "#FFFFFF"
const keyBg = "#2F6FED"
const barFg = "#E8EEF4"
const barBg = "#1B2836"

type keyHint struct {
	key   string
	label string
}

var homeKeys = []keyHint{
	{key: "enter", label: "open"},
	{key: "q", label: "quit"},
}

var listKeys = []keyHint{
	{key: "a", label: "add"},
	{key: "enter", label: "open"},
	{key: "esc", label: "back"},
	{key: "q", label: "quit"},
}

var pickKeys = []keyHint{
	{key: "enter", label: "select"},
	{key: "esc", label: "back"},
	{key: "q", label: "quit"},
}

var addKeys = []keyHint{
	{key: "a", label: "add"},
	{key: "esc", label: "back"},
	{key: "q", label: "quit"},
}

var formNextKeys = []keyHint{
	{key: "tab", label: "next"},
	{key: "enter", label: "next"},
	{key: "esc", label: "back"},
}

var formSaveKeys = []keyHint{
	{key: "tab", label: "saves"},
	{key: "enter", label: "saves"},
	{key: "esc", label: "back"},
}

var termWidth int
var termHeight int

func noteSize(msg tea.Msg) (sized bool) {
	size, ok := msg.(tea.WindowSizeMsg)
	if !ok {
		return
	}
	termWidth = size.Width
	termHeight = size.Height
	sized = true
	return
}

func render(trail []string, body string, notice string, hints []keyHint, cursor *tea.Cursor) (view tea.View) {
	path := paintPath(trail)
	lines := []string{path, "", body, ""}
	lines = appendNotice(lines, notice)
	content := joinLines(lines)
	bar := paintKeys(hints)
	text := pinKeys(content, bar)
	view = tea.NewView(text)
	view.AltScreen = true
	view.WindowTitle = "setup"
	if cursor != nil {
		view.Cursor = cursor
	}
	return
}

func paintKeys(hints []keyHint) (bar string) {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(keyFg)).Background(lipgloss.Color(keyBg))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(barFg)).Background(lipgloss.Color(barBg))
	gap := labelStyle.Render("  ")
	row := ""
	for i, hint := range hints {
		chip := keyStyle.Render(hint.key)
		word := labelStyle.Render(" " + hint.label)
		pair := chip + word
		first := i == 0
		if first {
			row = pair
			continue
		}
		row = row + gap + pair
	}
	style := lipgloss.NewStyle().Background(lipgloss.Color(barBg))
	wide := termWidth > 0
	if wide {
		style = style.Width(termWidth)
	}
	bar = style.Render(row)
	return
}

func pinKeys(content string, bar string) (text string) {
	unknown := termHeight < 2
	if unknown {
		text = content + "\n" + bar
		return
	}
	barHeight := lipgloss.Height(bar)
	bodyHeight := termHeight - barHeight
	short := bodyHeight < 1
	if short {
		bodyHeight = 1
	}
	fitted := lipgloss.NewStyle().MaxHeight(bodyHeight).Height(bodyHeight).Render(content)
	text = fitted + "\n" + bar
	return
}

func paintPath(parts []string) (path string) {
	last := len(parts) - 1
	for i, part := range parts {
		chunk := part
		end := i == last
		seam := !end
		if seam {
			chunk = part + ">"
		}
		fg := pathPastFg
		bg := pathPastBg
		if end {
			fg = pathNowFg
			bg = pathNowBg
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Background(lipgloss.Color(bg))
		if end {
			style = style.Bold(true)
		}
		painted := style.Render(chunk)
		path = path + painted
	}
	return
}

func appendNotice(lines []string, notice string) (next []string) {
	next = lines
	hasNotice := notice != ""
	if !hasNotice {
		return
	}
	next = append(next, notice)
	next = append(next, "")
	return
}

func joinLines(lines []string) (text string) {
	for i, line := range lines {
		first := i == 0
		if first {
			text = line
			continue
		}
		text = text + "\n" + line
	}
	return
}

func repoTrail(owner string, name string, tail ...string) (parts []string) {
	repo := owner + "/" + name
	parts = []string{"gcf", "repos", repo}
	parts = append(parts, tail...)
	return
}
