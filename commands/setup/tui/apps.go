package tui

import (
	"context"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

type apps struct {
	ctx    context.Context
	dir    string
	list   list.Model
	notice string
}

func newApps(ctx context.Context, dir string, selectID string, notice string) (s apps) {
	s.ctx = ctx
	s.dir = dir
	s.notice = notice
	ids, err := commands.ListAppIDs(dir)
	if err != nil {
		log.Error("list apps", "err", err)
		s.notice = err.Error()
	}
	lines := idLines(ids)
	s.list = newChooser("Apps", lines, lines, selectID)
	return
}

func (s apps) Init() (cmd tea.Cmd) {
	return
}

func (s apps) release() {}

func (s apps) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
	case actionAdd:
		updated, cmd = s.add()
	case actionBack:
		updated = newHome(s.ctx, s.dir)
	case actionMove:
		s.list, cmd = s.list.Update(press)
		updated = s
	default:
		updated = s
	}
	return
}

func (s apps) View() (view tea.View) {
	body := listBody(s.list, noApps)
	trail := []string{"gcf", "apps"}
	view = render(trail, body, s.notice, listKeys, nil)
	return
}

func (s apps) add() (updated tea.Model, cmd tea.Cmd) {
	next, focus := newAddApp(s.ctx, s.dir)
	updated = next
	cmd = focus
	return
}
