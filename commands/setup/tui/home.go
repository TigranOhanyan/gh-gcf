package tui

import (
	"context"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"
)

type home struct {
	ctx    context.Context
	dir    string
	list   list.Model
	notice string
}

func newHome(ctx context.Context, dir string) (s home) {
	s.ctx = ctx
	s.dir = dir
	items := []string{"repos", "apps"}
	s.list = newChooser("Setup", items, items, "")
	return
}

func (s home) Init() (cmd tea.Cmd) {
	return
}

func (s home) release() {}

func (s home) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
	case actionMove:
		s.list, cmd = s.list.Update(press)
		updated = s
	default:
		updated = s
	}
	return
}

func (s home) View() (view tea.View) {
	body := listBody(s.list, "")
	trail := []string{"gcf"}
	view = render(trail, body, s.notice, homeKeys, nil)
	return
}

func (s home) open() (updated tea.Model) {
	item, ok := selectedRow(s.list)
	if !ok {
		updated = s
		return
	}
	apps := item.value == "apps"
	if apps {
		updated = newApps(s.ctx, s.dir, "", "")
		return
	}
	updated = newRepos(s.ctx, s.dir, "")
	return
}
