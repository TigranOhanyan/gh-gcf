package tui

import (
	"context"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/configdb"
)

type labels struct {
	ctx    context.Context
	dir    string
	owner  string
	name   string
	db     *configdb.DB
	list   list.Model
	notice string
}

func newLabels(ctx context.Context, dir string, owner string, name string, db *configdb.DB, selectName string, notice string) (s labels) {
	s.ctx = ctx
	s.dir = dir
	s.owner = owner
	s.name = name
	s.db = db
	s.notice = notice
	found, err := db.Queries.ListLabels(ctx)
	if err != nil {
		log.Error("list labels", "err", err)
		s.notice = err.Error()
	}
	items := make([]string, 0, len(found))
	values := make([]string, 0, len(found))
	for _, label := range found {
		line := label.Name + "  " + label.Color
		items = append(items, line)
		values = append(values, label.Name)
	}
	heading := owner + "/" + name
	s.list = newChooser(heading, items, values, selectName)
	return
}

func (s labels) Init() (cmd tea.Cmd) {
	return
}

func (s labels) release() {
	closeDB(s.db)
}

func (s labels) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
	sized := noteSize(msg)
	if sized {
		updated = s
		return
	}
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
		updated = s.back()
	case actionMove:
		s.list, cmd = s.list.Update(press)
		updated = s
	default:
		updated = s
	}
	return
}

func (s labels) View() (view tea.View) {
	body := listBody(s.list, noLabels)
	trail := repoTrail(s.owner, s.name, "labels")
	view = render(trail, body, s.notice, listKeys, nil)
	return
}

func (s labels) add() (updated tea.Model, cmd tea.Cmd) {
	next, focus := newAddLabel(s.ctx, s.dir, s.owner, s.name, s.db)
	updated = next
	cmd = focus
	return
}

func (s labels) back() (updated tea.Model) {
	closeDB(s.db)
	updated = newMenu(s.ctx, s.dir, s.owner, s.name)
	return
}
