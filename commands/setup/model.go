package setup

import (
	"context"
	"strconv"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

const homeHelp = "enter open   q quit"
const listHelp = "a add   enter open   esc back   q quit"
const backHelp = "esc back   q quit"
const formContinueHelp = "tab next   enter next   esc back"
const formSaveHelp = "tab saves   enter saves   esc back"
const missingConfig = MissingConfig
const noRepos = "No app repos yet."
const noLabels = "No labels yet."
const noApps = "No apps yet."
const pickHelp = "enter select   esc back   q quit"

const pathPastFg = "#E8EEF4"
const pathPastBg = "#3C4F65"
const pathNowFg = "#FFFFFF"
const pathNowBg = "#2F6FED"

type screen int

const (
	screenHome screen = iota
	screenRepos
	screenAddRepo
	screenSections
	screenLabels
	screenAddLabel
	screenApps
	screenAddApp
	screenRepoApps
	screenPickApp
)

type place struct {
	screen screen
	owner  string
	name   string
}

type row struct {
	title string
	value string
}

func (item row) Title() (title string) {
	title = item.title
	return
}

func (item row) Description() (description string) {
	return
}

func (item row) FilterValue() (value string) {
	value = item.title
	return
}

type model struct {
	ctx        context.Context
	factoryDir string
	place      place
	repos      []AppRepo
	labels     []configrecords.Label
	appIDs     []int64
	attached   []int64
	db         *configdb.DB
	list       list.Model
	field      int
	first      textinput.Model
	second     textinput.Model
	notice     string
	help       string
}

func newModel(ctx context.Context, factoryDir string) (m model) {
	m.ctx = ctx
	m.factoryDir = factoryDir
	m.place.screen = screenHome
	var err error
	m, err = m.reloadRepos()
	if err != nil {
		m.notice = err.Error()
	}
	m = m.showHome()
	return
}

func (m model) Init() (cmd tea.Cmd) {
	return
}

func (m model) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
	press, isPress := msg.(tea.KeyPressMsg)
	if !isPress {
		updated, cmd = m.passForm(msg)
		return
	}
	stroke := press.String()
	quitNow := stroke == "ctrl+c"
	if quitNow {
		updated, cmd = m.quit()
		return
	}
	if m.onForm() {
		updated, cmd = m.onFormKey(press)
		return
	}
	updated, cmd = m.onListKey(press)
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
		updated, cmd = m.startAdd()
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
	moving := stroke == "up" || stroke == "down" || stroke == "k" || stroke == "j"
	if moving {
		updated = m
		updated.list, cmd = updated.list.Update(key)
		return
	}
	updated = m
	return
}

func (m model) onFormKey(key tea.KeyPressMsg) (updated model, cmd tea.Cmd) {
	stroke := key.String()
	escape := stroke == "esc"
	if escape {
		updated = m.cancelForm()
		return
	}
	forward := movesForward(key)
	if forward {
		updated, cmd = m.advanceForm()
		return
	}
	updated = m
	field := updated.activeField()
	field, cmd = field.Update(key)
	updated = updated.setActiveField(field)
	updated.notice = ""
	return
}

func (m model) passForm(msg tea.Msg) (updated model, cmd tea.Cmd) {
	updated = m
	form := updated.onForm()
	if !form {
		return
	}
	field := updated.activeField()
	field, cmd = field.Update(msg)
	updated = updated.setActiveField(field)
	return
}

func movesForward(key tea.KeyPressMsg) (forward bool) {
	plain := key.Mod == 0
	enter := key.Code == tea.KeyEnter || key.Code == tea.KeyKpEnter
	tab := key.Code == tea.KeyTab
	forward = plain && (enter || tab)
	return
}

func (m model) View() (view tea.View) {
	content := m.render()
	view = tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "setup"
	if m.onForm() {
		field := m.activeField()
		cursor := field.Cursor()
		if cursor != nil {
			view.Cursor = cursor
		}
	}
	return
}

func (m model) render() (text string) {
	if m.onForm() {
		text = m.renderForm()
		return
	}
	path := m.nav()
	body := m.listBody()
	lines := []string{path, "", body, ""}
	lines = appendNotice(lines, m.notice)
	lines = append(lines, m.help)
	text = joinLines(lines)
	return
}

func (m model) listBody() (text string) {
	empty := len(m.list.Items()) == 0
	if empty {
		text = m.emptyList()
		return
	}
	text = m.list.View()
	return
}

func (m model) emptyList() (text string) {
	switch m.place.screen {
	case screenRepos:
		text = noRepos
	case screenLabels:
		text = noLabels
	case screenApps, screenRepoApps, screenPickApp:
		text = noApps
	default:
		text = ""
	}
	return
}

func (m model) renderForm() (text string) {
	path := m.nav()
	lines := []string{path, "", m.first.View()}
	two := m.twoFields()
	if two {
		lines = append(lines, m.second.View())
	}
	lines = append(lines, "")
	lines = appendNotice(lines, m.notice)
	lines = append(lines, m.formHelp())
	text = joinLines(lines)
	return
}

func (m model) nav() (path string) {
	parts := m.navParts()
	path = paintPath(parts)
	return
}

func (m model) navParts() (parts []string) {
	parts = []string{"gcf"}
	switch m.place.screen {
	case screenHome:
	case screenRepos:
		parts = append(parts, "repos")
	case screenAddRepo:
		parts = append(parts, "repos", "add")
	case screenApps:
		parts = append(parts, "apps")
	case screenAddApp:
		parts = append(parts, "apps", "add")
	default:
		repo := m.place.owner + "/" + m.place.name
		parts = append(parts, "repos", repo)
		tail := m.navTail()
		parts = append(parts, tail...)
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

func (m model) navTail() (parts []string) {
	switch m.place.screen {
	case screenLabels:
		parts = []string{"labels"}
	case screenAddLabel:
		parts = []string{"labels", "add"}
	case screenRepoApps:
		parts = []string{"apps"}
	case screenPickApp:
		parts = []string{"apps", "add"}
	default:
		parts = []string{}
	}
	return
}

func (m model) formHelp() (help string) {
	one := m.place.screen == screenAddApp
	last := m.field == 1
	saving := one || last
	if saving {
		help = formSaveHelp
		return
	}
	help = formContinueHelp
	return
}

func (m model) onForm() (form bool) {
	switch m.place.screen {
	case screenAddRepo, screenAddLabel, screenAddApp:
		form = true
	}
	return
}

func (m model) twoFields() (two bool) {
	two = m.place.screen == screenAddRepo || m.place.screen == screenAddLabel
	return
}

func (m model) quit() (updated model, cmd tea.Cmd) {
	updated = m.closeDB()
	cmd = tea.Quit
	return
}

func (m model) startAdd() (updated model, cmd tea.Cmd) {
	updated = m
	switch updated.place.screen {
	case screenRepos:
		updated.place.screen = screenAddRepo
		updated.field = 0
		updated.first = newField("owner: ")
		updated.second = newField("name: ")
		updated.notice = ""
		updated, cmd = updated.focusActive()
	case screenLabels:
		missing := updated.db == nil
		if missing {
			updated.notice = missingConfig
			return
		}
		updated.place.screen = screenAddLabel
		updated.field = 0
		updated.first = newField("name: ")
		updated.second = newField("color: ")
		updated.notice = ""
		updated, cmd = updated.focusActive()
	case screenApps:
		updated.place.screen = screenAddApp
		updated.field = 0
		updated.first = newField("app id: ")
		updated.notice = ""
		updated, cmd = updated.focusActive()
	case screenRepoApps:
		missing := updated.db == nil
		if missing {
			updated.notice = missingConfig
			return
		}
		updated = updated.showPick()
	default:
	}
	return
}

func (m model) openSelected() (updated model) {
	updated = m
	switch updated.place.screen {
	case screenHome:
		updated = updated.openHome()
	case screenRepos:
		updated = updated.openRepo()
	case screenSections:
		updated = updated.openSection()
	case screenPickApp:
		updated = updated.pickApp()
	default:
	}
	return
}

func (m model) openHome() (updated model) {
	updated = m
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	apps := item.value == "apps"
	if apps {
		updated = updated.showApps()
		return
	}
	updated = updated.showRepos()
	return
}

func (m model) openRepo() (updated model) {
	updated = m
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	index := indexOfRepo(updated.repos, item.value)
	missing := index < 0
	if missing {
		return
	}
	repo := updated.repos[index]
	updated = updated.enterRepo(repo)
	return
}

func (m model) enterRepo(repo AppRepo) (updated model) {
	updated = m.closeDB()
	updated.place.owner = repo.owner
	updated.place.name = repo.name
	updated.place.screen = screenSections
	updated.notice = ""
	updated.labels = nil
	updated.attached = nil

	path := ConfigPath(updated.factoryDir, repo.owner, repo.name)
	db, missing, err := OpenConfig(updated.ctx, path)
	if err != nil {
		updated.notice = err.Error()
		updated = updated.showSections()
		return
	}
	if missing {
		updated.notice = missingConfig
		updated = updated.showSections()
		return
	}
	updated.db = db
	updated = updated.showSections()
	return
}

func (m model) openSection() (updated model) {
	updated = m
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	switch item.value {
	case "apps":
		updated = updated.showRepoApps()
	default:
		updated = updated.showLabels()
	}
	return
}

func (m model) back() (updated model) {
	updated = m
	switch updated.place.screen {
	case screenRepos, screenApps:
		updated = updated.showHome()
	case screenSections:
		updated = updated.closeDB()
		updated.notice = ""
		updated = updated.showRepos()
	case screenLabels, screenRepoApps:
		updated.notice = ""
		updated = updated.showSections()
	case screenPickApp:
		updated.notice = ""
		updated = updated.showRepoApps()
	default:
	}
	return
}

func (m model) cancelForm() (updated model) {
	updated = m
	updated.notice = ""
	switch updated.place.screen {
	case screenAddRepo:
		updated = updated.showRepos()
	case screenAddLabel:
		updated = updated.showLabels()
	default:
		updated = updated.showApps()
	}
	return
}

func (m model) advanceForm() (updated model, cmd tea.Cmd) {
	one := m.place.screen == screenAddApp
	if one {
		updated = m.submitApp()
		return
	}
	first := m.field == 0
	if first {
		updated = m
		updated.field = 1
		updated.notice = ""
		updated, cmd = updated.focusActive()
		return
	}
	addingRepo := m.place.screen == screenAddRepo
	if addingRepo {
		updated = m.submitRepo()
		return
	}
	updated = m.submitLabel()
	return
}

func (m model) submitRepo() (updated model) {
	updated = m
	owner := updated.first.Value()
	name := updated.second.Value()
	db, err := CreateRepo(updated.ctx, updated.factoryDir, owner, name)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	err = db.Close()
	if err != nil {
		log.Error("close config", "err", err)
	}
	updated.notice = ""
	var reloadErr error
	updated, reloadErr = updated.reloadRepos()
	if reloadErr != nil {
		updated.notice = reloadErr.Error()
		return
	}
	updated = updated.showRepos()
	return
}

func (m model) submitLabel() (updated model) {
	updated = m
	missing := updated.db == nil
	if missing {
		updated.notice = missingConfig
		return
	}
	nameText := updated.first.Value()
	colorText := updated.second.Value()
	name, err := AddLabel(updated.ctx, updated.db, nameText, colorText)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.notice = ""
	var listErr error
	updated, listErr = updated.reloadLabels()
	if listErr != nil {
		updated.notice = listErr.Error()
		return
	}
	updated = updated.showLabels()
	index := indexOfLabel(updated.labels, name)
	found := index >= 0
	if found {
		updated.list.Select(index)
	}
	return
}

func (m model) submitApp() (updated model) {
	updated = m
	raw := updated.first.Value()
	id, err := CreateAppFile(updated.factoryDir, raw)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.notice = secretName(id)
	var listErr error
	updated, listErr = updated.reloadApps()
	if listErr != nil {
		updated.notice = listErr.Error()
		return
	}
	updated = updated.showApps()
	text := strconv.FormatInt(id, 10)
	index := indexOfText(updated.appIDs, text)
	found := index >= 0
	if found {
		updated.list.Select(index)
	}
	return
}

func (m model) pickApp() (updated model) {
	updated = m
	missing := updated.db == nil
	if missing {
		updated.notice = missingConfig
		return
	}
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(item.value, 10, 64)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	err = AttachApp(updated.ctx, updated.db, id)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.notice = secretName(id)
	var listErr error
	updated, listErr = updated.reloadAttached()
	if listErr != nil {
		updated.notice = listErr.Error()
		return
	}
	updated = updated.showRepoApps()
	return
}

func (m model) showHome() (updated model) {
	updated = m
	updated.place.screen = screenHome
	items := []string{"repos", "apps"}
	values := []string{"repos", "apps"}
	updated = updated.setList("Setup", items, values, 0, homeHelp)
	return
}

func (m model) showRepos() (updated model) {
	updated = m
	updated.place.screen = screenRepos
	items := make([]string, 0, len(updated.repos))
	values := make([]string, 0, len(updated.repos))
	for _, repo := range updated.repos {
		line := repo.owner + "/" + repo.name
		missing := !repo.hasConfig
		if missing {
			line = line + "  " + missingConfig
		}
		items = append(items, line)
		values = append(values, repo.owner+"/"+repo.name)
	}
	updated = updated.setList("Repos", items, values, 0, listHelp)
	return
}

func (m model) showSections() (updated model) {
	updated = m
	updated.place.screen = screenSections
	heading := updated.place.owner + "/" + updated.place.name
	items := []string{"labels", "apps"}
	values := []string{"labels", "apps"}
	updated = updated.setList(heading, items, values, 0, listHelp)
	return
}

func (m model) showLabels() (updated model) {
	updated = m
	updated.place.screen = screenLabels
	var listErr error
	updated, listErr = updated.reloadLabels()
	if listErr != nil {
		updated.notice = listErr.Error()
	}
	items := make([]string, 0, len(updated.labels))
	values := make([]string, 0, len(updated.labels))
	for _, label := range updated.labels {
		line := label.Name + "  " + label.Color
		items = append(items, line)
		values = append(values, label.Name)
	}
	heading := updated.place.owner + "/" + updated.place.name
	updated = updated.setList(heading, items, values, 0, listHelp)
	return
}

func (m model) showApps() (updated model) {
	updated = m
	updated.place.screen = screenApps
	var listErr error
	updated, listErr = updated.reloadApps()
	if listErr != nil {
		updated.notice = listErr.Error()
	}
	items := idLines(updated.appIDs)
	updated = updated.setList("Apps", items, items, 0, listHelp)
	return
}

func (m model) showRepoApps() (updated model) {
	updated = m
	updated.place.screen = screenRepoApps
	var listErr error
	updated, listErr = updated.reloadAttached()
	if listErr != nil {
		updated.notice = listErr.Error()
	}
	items := idLines(updated.attached)
	heading := updated.place.owner + "/" + updated.place.name
	updated = updated.setList(heading, items, items, 0, listHelp)
	return
}

func (m model) showPick() (updated model) {
	updated = m
	updated.place.screen = screenPickApp
	var listErr error
	updated, listErr = updated.reloadApps()
	if listErr != nil {
		updated.notice = listErr.Error()
		return
	}
	items := idLines(updated.appIDs)
	updated = updated.setList("Apps", items, items, 0, backHelp)
	return
}

func (m model) setList(title string, items []string, values []string, index int, help string) (updated model) {
	updated = m
	rows := make([]list.Item, 0, len(items))
	for i, item := range items {
		value := item
		hasValue := i < len(values)
		if hasValue {
			value = values[i]
		}
		next := row{title: item, value: value}
		rows = append(rows, next)
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	chooser := list.New(rows, delegate, 60, 16)
	chooser.Title = title
	chooser.SetShowTitle(false)
	chooser.SetFilteringEnabled(false)
	chooser.SetShowStatusBar(false)
	chooser.SetShowHelp(false)
	chooser.SetShowPagination(false)
	chooser.SetShowFilter(false)
	hasRows := len(rows) > 0
	if hasRows {
		chooser.Select(index)
	}
	updated.list = chooser
	updated.help = help
	return
}

func (m model) activeField() (field textinput.Model) {
	second := m.field == 1 && m.twoFields()
	if second {
		field = m.second
		return
	}
	field = m.first
	return
}

func (m model) setActiveField(field textinput.Model) (updated model) {
	updated = m
	second := updated.field == 1 && updated.twoFields()
	if second {
		updated.second = field
		return
	}
	updated.first = field
	return
}

func newField(prompt string) (field textinput.Model) {
	field = textinput.New()
	field.Prompt = prompt
	return
}

func (m model) focusActive() (updated model, cmd tea.Cmd) {
	updated = m
	updated.first.Blur()
	updated.second.Blur()
	field := updated.activeField()
	cmd = field.Focus()
	updated = updated.setActiveField(field)
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
	repos, err := ListRepos(updated.factoryDir)
	if err != nil {
		return
	}
	updated.repos = repos
	return
}

func (m model) reloadLabels() (updated model, err error) {
	updated = m
	if updated.db == nil {
		updated.labels = nil
		return
	}
	labels, err := updated.db.Queries.ListLabels(updated.ctx)
	if err != nil {
		log.Error("list labels", "err", err)
		return
	}
	updated.labels = labels
	return
}

func (m model) reloadApps() (updated model, err error) {
	updated = m
	ids, err := commands.ListAppIDs(updated.factoryDir)
	if err != nil {
		return
	}
	updated.appIDs = ids
	return
}

func (m model) reloadAttached() (updated model, err error) {
	updated = m
	if updated.db == nil {
		updated.attached = nil
		return
	}
	ids, err := updated.db.Queries.ListGithubApps(updated.ctx)
	if err != nil {
		log.Error("list apps", "err", err)
		return
	}
	updated.attached = ids
	return
}

func selectedRow(chooser list.Model) (item row, ok bool) {
	selected := chooser.SelectedItem()
	if selected == nil {
		return
	}
	item, ok = selected.(row)
	return
}

func idLines(ids []int64) (lines []string) {
	lines = make([]string, 0, len(ids))
	for _, id := range ids {
		text := strconv.FormatInt(id, 10)
		lines = append(lines, text)
	}
	return
}

func indexOfRepo(repos []AppRepo, key string) (index int) {
	index = -1
	for i, repo := range repos {
		same := repo.owner+"/"+repo.name == key
		if same {
			index = i
			return
		}
	}
	return
}

func indexOfLabel(labels []configrecords.Label, name string) (index int) {
	index = -1
	for i, label := range labels {
		same := label.Name == name
		if same {
			index = i
			return
		}
	}
	return
}

func indexOfText(ids []int64, text string) (index int) {
	index = -1
	for i, id := range ids {
		line := strconv.FormatInt(id, 10)
		same := line == text
		if same {
			index = i
			return
		}
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
