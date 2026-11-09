package tui

import (
	"context"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/development"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

type developments struct {
	ctx    context.Context
	dir    string
	owner  string
	name   string
	db     *configdb.DB
	list   list.Model
	notice string
}

func newDevelopments(ctx context.Context, dir string, owner string, name string, db *configdb.DB, selectName string, notice string) (s developments) {
	s.ctx = ctx
	s.dir = dir
	s.owner = owner
	s.name = name
	s.db = db
	s.notice = notice
	found, err := db.Queries.ListDevelopments(ctx)
	if err != nil {
		log.Error("list developments", "err", err)
		s.notice = err.Error()
	}
	items := make([]string, 0, len(found))
	values := make([]string, 0, len(found))
	for _, row := range found {
		line := development.Line(row)
		items = append(items, line)
		values = append(values, row.Name)
	}
	heading := owner + "/" + name
	s.list = newChooser(heading, items, values, selectName)
	return
}

func (s developments) Init() (cmd tea.Cmd) {
	return
}

func (s developments) release() {
	closeDB(s.db)
}

func (s developments) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
	case actionOpen:
		updated = s.open()
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

func (s developments) View() (view tea.View) {
	body := listBody(s.list, noDevelopments)
	trail := repoTrail(s.owner, s.name, "developments")
	view = render(trail, body, s.notice, listKeys, nil)
	return
}

func (s developments) add() (updated tea.Model, cmd tea.Cmd) {
	next, focus := newAddDevelopment(s.ctx, s.dir, s.owner, s.name, s.db, "")
	updated = next
	cmd = focus
	return
}

func (s developments) open() (updated tea.Model) {
	item, ok := selectedRow(s.list)
	if !ok {
		updated = s
		return
	}
	updated = newChecks(s.ctx, s.dir, s.owner, s.name, s.db, item.value, "", "")
	return
}

func (s developments) back() (updated tea.Model) {
	closeDB(s.db)
	updated = newMenu(s.ctx, s.dir, s.owner, s.name)
	return
}
