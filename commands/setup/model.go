package setup

import (
	"context"
	"strings"
	"unicode/utf8"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

const reposHelp = "a add   enter open   q quit"
const labelsHelp = "a add   esc repos   q quit"
const missingHelp = "esc repos   q quit"
const formContinueHelp = "enter continues   esc back"
const formSaveHelp = "enter saves   esc back"
const missingConfig = "config.db is missing"
const noRepos = "No app repos yet."

var titleStyle = lipgloss.NewStyle().Bold(true)

type screen int

const (
	screenRepos screen = iota
	screenAddRepo
	screenLabels
	screenAddLabel
)

type model struct {
	ctx         context.Context
	factoryDir  string
	screen      screen
	repos       []appRepo
	repoCursor  int
	current     appRepo
	db          *configdb.DB
	labels      []configrecords.Label
	labelCursor int
	field       int
	ownerInput  string
	nameInput   string
	labelInput  string
	colorInput  string
	notice      string
}

func newModel(ctx context.Context, factoryDir string) (m model) {
	m.ctx = ctx
	m.factoryDir = factoryDir
	m.screen = screenRepos
	var err error
	m, err = m.reloadRepos()
	if err != nil {
		m.notice = err.Error()
	}
	return
}

func (m model) Init() (cmd tea.Cmd) {
	return
}

func (m model) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
	press, isPress := msg.(tea.KeyPressMsg)
	if !isPress {
		updated = m
		return
	}
	m, cmd = m.onKey(press)
	updated = m
	return
}

func (m model) onKey(key tea.KeyPressMsg) (updated model, cmd tea.Cmd) {
	stroke := key.String()
	quitNow := stroke == "ctrl+c"
	if quitNow {
		updated, cmd = m.quit()
		return
	}

	onForm := m.screen == screenAddRepo || m.screen == screenAddLabel
	if onForm {
		updated = m.onFormKey(key)
		return
	}
	updated, cmd = m.onListKey(key)
	return
}

func (m model) onListKey(key tea.KeyPressMsg) (updated model, cmd tea.Cmd) {
	stroke := key.String()
	quit := stroke == "q"
	if quit {
		updated, cmd = m.quit()
		return
	}
	add := stroke == "a"
	if add {
		updated = m.startAdd()
		return
	}
	open := stroke == "enter"
	if open {
		updated = m.openSelected()
		return
	}
	back := stroke == "esc"
	if back {
		updated = m.back()
		return
	}
	up := stroke == "up" || stroke == "k"
	if up {
		updated = m.move(-1)
		return
	}
	down := stroke == "down" || stroke == "j"
	if down {
		updated = m.move(1)
		return
	}
	updated = m
	return
}

func (m model) onFormKey(key tea.KeyPressMsg) (updated model) {
	stroke := key.String()
	escape := stroke == "esc"
	if escape {
		updated = m.cancelForm()
		return
	}
	enter := stroke == "enter"
	if enter {
		updated = m.advanceForm()
		return
	}
	back := stroke == "backspace"
	if back {
		updated = m.backspace()
		return
	}

	text := key.Text
	hasText := text != ""
	if hasText {
		updated = m.appendText(text)
		return
	}
	updated = m
	return
}

func (m model) View() (view tea.View) {
	content := m.render()
	view = tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "setup"
	onForm := m.screen == screenAddRepo || m.screen == screenAddLabel
	if onForm {
		x, y := m.cursorSpot()
		cursor := tea.NewCursor(x, y)
		view.Cursor = cursor
	}
	return
}

func (m model) quit() (updated model, cmd tea.Cmd) {
	updated = m.closeDB()
	cmd = tea.Quit
	return
}

func (m model) startAdd() (updated model) {
	updated = m
	onRepos := updated.screen == screenRepos
	if onRepos {
		updated.screen = screenAddRepo
		updated.field = 0
		updated.ownerInput = ""
		updated.nameInput = ""
		updated.notice = ""
		return
	}

	missing := updated.db == nil
	if missing {
		updated.notice = missingConfig
		return
	}
	updated.screen = screenAddLabel
	updated.field = 0
	updated.labelInput = ""
	updated.colorInput = ""
	updated.notice = ""
	return
}

func (m model) openSelected() (updated model) {
	onRepos := m.screen == screenRepos
	if !onRepos {
		updated = m
		return
	}
	empty := len(m.repos) == 0
	if empty {
		updated = m
		return
	}
	selected := m.repos[m.repoCursor]
	updated = m.openRepo(selected)
	return
}

func (m model) openRepo(repo appRepo) (updated model) {
	updated = m.closeDB()
	updated.current = repo
	updated.labels = nil
	updated.labelCursor = 0
	updated.screen = screenLabels
	updated.notice = ""

	path := configPath(updated.factoryDir, repo.owner, repo.name)
	db, missing, err := openConfig(updated.ctx, path)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	if missing {
		updated.notice = missingConfig
		return
	}

	updated.db = db
	var listErr error
	updated, listErr = updated.reloadLabels()
	if listErr != nil {
		updated.notice = listErr.Error()
	}
	return
}

func (m model) back() (updated model) {
	onLabels := m.screen == screenLabels
	if !onLabels {
		updated = m
		return
	}
	updated = m.closeDB()
	updated.screen = screenRepos
	updated.labels = nil
	updated.notice = ""
	return
}

func (m model) cancelForm() (updated model) {
	updated = m
	addingRepo := updated.screen == screenAddRepo
	if addingRepo {
		updated.screen = screenRepos
		updated.ownerInput = ""
		updated.nameInput = ""
		updated.field = 0
		updated.notice = ""
		return
	}
	updated.screen = screenLabels
	updated.labelInput = ""
	updated.colorInput = ""
	updated.field = 0
	updated.notice = ""
	return
}

func (m model) advanceForm() (updated model) {
	first := m.field == 0
	if first {
		updated = m
		updated.field = 1
		updated.notice = ""
		return
	}
	addingRepo := m.screen == screenAddRepo
	if addingRepo {
		updated = m.submitRepo()
		return
	}
	updated = m.submitLabel()
	return
}

func (m model) submitRepo() (updated model) {
	updated = m
	owner, err := normalizeSegment(updated.ownerInput)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	name, err := normalizeSegment(updated.nameInput)
	if err != nil {
		updated.notice = err.Error()
		return
	}

	db, err := createRepo(updated.ctx, updated.factoryDir, owner, name)
	if err != nil {
		updated.notice = err.Error()
		return
	}

	updated = updated.closeDB()
	updated.db = db
	updated.current = appRepo{owner: owner, name: name, hasConfig: true}
	updated.screen = screenLabels
	updated.ownerInput = ""
	updated.nameInput = ""
	updated.field = 0
	updated.notice = ""
	updated.labels = nil
	updated.labelCursor = 0

	var listErr error
	updated, listErr = updated.reloadLabels()
	if listErr != nil {
		updated.notice = listErr.Error()
	}
	var reloadErr error
	updated, reloadErr = updated.reloadRepos()
	if reloadErr != nil {
		updated.notice = reloadErr.Error()
		return
	}
	index, found := repoIndex(updated.repos, owner, name)
	if found {
		updated.repoCursor = index
	}
	return
}

func (m model) submitLabel() (updated model) {
	updated = m
	missing := updated.db == nil
	if missing {
		updated.notice = missingConfig
		return
	}

	name, err := addLabel(updated.ctx, updated.db, updated.labelInput, updated.colorInput)
	if err != nil {
		updated.notice = err.Error()
		return
	}

	updated.labelInput = ""
	updated.colorInput = ""
	updated.field = 0
	updated.notice = ""
	updated.screen = screenLabels
	var listErr error
	updated, listErr = updated.reloadLabels()
	if listErr != nil {
		updated.notice = listErr.Error()
		return
	}
	index, found := labelIndex(updated.labels, name)
	if found {
		updated.labelCursor = index
	}
	return
}

func (m model) move(delta int) (updated model) {
	updated = m
	var length int
	var cursor int
	onRepos := updated.screen == screenRepos
	if onRepos {
		length = len(updated.repos)
		cursor = updated.repoCursor
	}
	onLabels := updated.screen == screenLabels
	if onLabels {
		length = len(updated.labels)
		cursor = updated.labelCursor
	}
	empty := length == 0
	if empty {
		return
	}

	cursor = cursor + delta
	below := cursor < 0
	if below {
		cursor = 0
	}
	last := length - 1
	above := cursor > last
	if above {
		cursor = last
	}
	if onRepos {
		updated.repoCursor = cursor
		return
	}
	if onLabels {
		updated.labelCursor = cursor
	}
	return
}

func (m model) appendText(text string) (updated model) {
	current := m.activeText()
	next := current + text
	updated = m.setActiveText(next)
	updated.notice = ""
	return
}

func (m model) backspace() (updated model) {
	current := m.activeText()
	next := dropLast(current)
	updated = m.setActiveText(next)
	updated.notice = ""
	return
}

func (m model) activeText() (text string) {
	addingRepo := m.screen == screenAddRepo
	first := m.field == 0
	if addingRepo {
		if first {
			text = m.ownerInput
			return
		}
		text = m.nameInput
		return
	}
	if first {
		text = m.labelInput
		return
	}
	text = m.colorInput
	return
}

func (m model) setActiveText(text string) (updated model) {
	updated = m
	addingRepo := updated.screen == screenAddRepo
	first := updated.field == 0
	if addingRepo {
		if first {
			updated.ownerInput = text
			return
		}
		updated.nameInput = text
		return
	}
	if first {
		updated.labelInput = text
		return
	}
	updated.colorInput = text
	return
}

func (m model) closeDB() (updated model) {
	updated = m
	if updated.db == nil {
		return
	}
	err := updated.db.Close()
	if err != nil {
		log.Error("close config", "path", updated.db.FileFullPath, "err", err)
	}
	updated.db = nil
	return
}

func (m model) reloadRepos() (updated model, err error) {
	updated = m
	repos, err := listRepos(updated.factoryDir)
	if err != nil {
		return
	}
	updated.repos = repos
	updated.repoCursor = clamp(updated.repoCursor, len(repos))
	return
}

func (m model) reloadLabels() (updated model, err error) {
	updated = m
	if updated.db == nil {
		return
	}
	labels, err := updated.db.Queries.ListLabels(updated.ctx)
	if err != nil {
		log.Error("list labels", "err", err)
		return
	}
	updated.labels = labels
	updated.labelCursor = clamp(updated.labelCursor, len(labels))
	return
}

func (m model) render() (text string) {
	onRepos := m.screen == screenRepos
	if onRepos {
		text = m.renderRepos()
		return
	}
	onAddRepo := m.screen == screenAddRepo
	if onAddRepo {
		text = m.renderForm("Add a repo", "owner", "name", m.ownerInput, m.nameInput)
		return
	}
	onLabels := m.screen == screenLabels
	if onLabels {
		text = m.renderLabels()
		return
	}
	text = m.renderForm("Add a label", "name", "color", m.labelInput, m.colorInput)
	return
}

func (m model) renderRepos() (text string) {
	title := titleStyle.Render("Repos")
	lines := []string{title, ""}
	empty := len(m.repos) == 0
	if empty {
		lines = append(lines, noRepos)
	}
	for i, repo := range m.repos {
		selected := i == m.repoCursor
		line := repoLine(repo, selected)
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = appendNotice(lines, m.notice)
	lines = append(lines, reposHelp)
	text = strings.Join(lines, "\n")
	return
}

func (m model) renderLabels() (text string) {
	heading := m.current.owner + "/" + m.current.name
	title := titleStyle.Render(heading)
	lines := []string{title, ""}
	absent := m.db == nil
	if absent {
		message := m.notice
		empty := message == ""
		if empty {
			message = missingConfig
		}
		lines = append(lines, message)
		lines = append(lines, "")
		lines = append(lines, missingHelp)
		text = strings.Join(lines, "\n")
		return
	}
	empty := len(m.labels) == 0
	if empty {
		lines = append(lines, "No labels yet.")
	}
	for i, label := range m.labels {
		selected := i == m.labelCursor
		line := labelLine(label, selected)
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = appendNotice(lines, m.notice)
	lines = append(lines, labelsHelp)
	text = strings.Join(lines, "\n")
	return
}

func (m model) renderForm(title string, firstLabel string, secondLabel string, firstValue string, secondValue string) (text string) {
	heading := titleStyle.Render(title)
	firstLine := fieldLine(firstLabel, firstValue)
	secondLine := fieldLine(secondLabel, secondValue)
	lines := []string{heading, "", firstLine, secondLine, ""}
	lines = appendNotice(lines, m.notice)
	help := m.formHelp()
	lines = append(lines, help)
	text = strings.Join(lines, "\n")
	return
}

func (m model) formHelp() (help string) {
	last := m.field == 1
	if last {
		help = formSaveHelp
		return
	}
	help = formContinueHelp
	return
}

func (m model) cursorSpot() (x int, y int) {
	y = 2 + m.field
	var label string
	addingRepo := m.screen == screenAddRepo
	first := m.field == 0
	if addingRepo {
		if first {
			label = "owner"
		} else {
			label = "name"
		}
	} else if first {
		label = "name"
	} else {
		label = "color"
	}
	prefix := label + ": "
	value := m.activeText()
	x = utf8.RuneCountInString(prefix) + utf8.RuneCountInString(value)
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

func repoLine(repo appRepo, selected bool) (line string) {
	mark := "  "
	if selected {
		mark = "> "
	}
	line = mark + repo.owner + "/" + repo.name
	missing := !repo.hasConfig
	if missing {
		line = line + "  " + missingConfig
	}
	return
}

func labelLine(label configrecords.Label, selected bool) (line string) {
	mark := "  "
	if selected {
		mark = "> "
	}
	line = mark + label.Name + "  " + label.Color
	return
}

func fieldLine(label string, value string) (line string) {
	line = label + ": " + value
	return
}

func dropLast(text string) (next string) {
	empty := text == ""
	if empty {
		return
	}
	runes := []rune(text)
	last := len(runes) - 1
	next = string(runes[:last])
	return
}

func repoIndex(repos []appRepo, owner string, name string) (index int, found bool) {
	for i, repo := range repos {
		sameOwner := repo.owner == owner
		sameName := repo.name == name
		same := sameOwner && sameName
		if same {
			index = i
			found = true
			return
		}
	}
	return
}

func labelIndex(labels []configrecords.Label, name string) (index int, found bool) {
	for i, label := range labels {
		same := label.Name == name
		if same {
			index = i
			found = true
			return
		}
	}
	return
}

func clamp(cursor int, length int) (next int) {
	empty := length == 0
	if empty {
		return
	}
	next = cursor
	below := next < 0
	if below {
		next = 0
	}
	last := length - 1
	above := next > last
	if above {
		next = last
	}
	return
}
