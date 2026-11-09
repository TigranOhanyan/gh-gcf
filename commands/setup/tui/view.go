package tui

import (
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const homeKeys = "enter open   q quit"
const listKeys = "a add   enter open   esc back   q quit"
const pickKeys = "enter select   esc back   q quit"
const formNextKeys = "tab next   enter next   esc back"
const formSaveKeys = "tab saves   enter saves   esc back"
const noRepos = "No app repos yet."
const noLabels = "No labels yet."
const noApps = "No apps yet."

const pathPastFg = "#E8EEF4"
const pathPastBg = "#3C4F65"
const pathNowFg = "#FFFFFF"
const pathNowBg = "#2F6FED"

func render(trail []string, body string, notice string, keys string, cursor *tea.Cursor) (view tea.View) {
	path := paintPath(trail)
	lines := []string{path, "", body, ""}
	lines = appendNotice(lines, notice)
	lines = append(lines, keys)
	text := joinLines(lines)
	view = tea.NewView(text)
	view.AltScreen = true
	view.WindowTitle = "setup"
	if cursor != nil {
		view.Cursor = cursor
	}
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
