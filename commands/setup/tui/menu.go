package tui

import (
	"context"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/config"
)

type menu struct {
	ctx    context.Context
	dir    string
	owner  string
	name   string
	list   list.Model
	notice string
}

func newMenu(ctx context.Context, dir string, owner string, name string) (s menu) {
	s.ctx = ctx
	s.dir = dir
	s.owner = owner
	s.name = name
	items := []string{"labels", "apps"}
	heading := owner + "/" + name
	s.list = newChooser(heading, items, items, "")
	return
}

func (s menu) Init() (cmd tea.Cmd) {
	return
}

func (s menu) release() {}

func (s menu) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
	press, isPress := msg.(tea.KeyPressMsg)
	if !isPress {
		updated = s
		return
	}
	kind := listAction(press)
	switch kind {
	case actionQuit:
		updated = s
		cmd = tea.Quit
	case actionOpen:
		updated = s.open()
	case actionBack:
		updated = newRepos(s.ctx, s.dir, "")
	case actionMove:
		s.list, cmd = s.list.Update(press)
		updated = s
	default:
		updated = s
	}
	return
}

func (s menu) View() (view tea.View) {
	body := listBody(s.list, "")
	trail := repoTrail(s.owner, s.name)
	view = render(trail, body, s.notice, listKeys, nil)
	return
}

func (s menu) open() (updated tea.Model) {
	item, ok := selectedRow(s.list)
	if !ok {
		updated = s
		return
	}
	labels := item.value == "labels"
	if labels {
		updated = s.openLabels()
		return
	}
	updated = s.openApps()
	return
}

func (s menu) openLabels() (updated tea.Model) {
	db, missing, err := openRepo(s.ctx, s.dir, s.owner, s.name)
	if err != nil {
		s.notice = err.Error()
		updated = s
		return
	}
	if missing {
		s.notice = config.MissingConfig
		updated = s
		return
	}
	updated = newLabels(s.ctx, s.dir, s.owner, s.name, db, "", "")
	return
}

func (s menu) openApps() (updated tea.Model) {
	db, missing, err := openRepo(s.ctx, s.dir, s.owner, s.name)
	if err != nil {
		s.notice = err.Error()
		updated = s
		return
	}
	if missing {
		s.notice = config.MissingConfig
		updated = s
		return
	}
	updated = newRepoApps(s.ctx, s.dir, s.owner, s.name, db, "", "")
	return
}
