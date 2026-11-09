package tui

import (
	"context"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/configdb"
)

type checks struct {
	ctx         context.Context
	dir         string
	owner       string
	name        string
	development string
	db          *configdb.DB
	list        list.Model
	notice      string
}

func newChecks(ctx context.Context, dir string, owner string, name string, db *configdb.DB, development string, selectName string, notice string) (s checks) {
	s.ctx = ctx
	s.dir = dir
	s.owner = owner
	s.name = name
	s.development = development
	s.db = db
	s.notice = notice
	found, err := db.Queries.ListDevelopmentChecks(ctx, development)
	if err != nil {
		log.Error("list development checks", "development", development, "err", err)
		s.notice = err.Error()
	}
	heading := owner + "/" + name
	s.list = newChooser(heading, found, found, selectName)
	return
}

func (s checks) Init() (cmd tea.Cmd) {
	return
}

func (s checks) release() {
	closeDB(s.db)
}

func (s checks) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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

func (s checks) View() (view tea.View) {
	body := listBody(s.list, noChecks)
	trail := repoTrail(s.owner, s.name, "developments", s.development)
	view = render(trail, body, s.notice, addKeys, nil)
	return
}

func (s checks) add() (updated tea.Model, cmd tea.Cmd) {
	next, focus := newAddCheck(s.ctx, s.dir, s.owner, s.name, s.db, s.development)
	updated = next
	cmd = focus
	return
}

func (s checks) back() (updated tea.Model) {
	updated = newDevelopments(s.ctx, s.dir, s.owner, s.name, s.db, s.development, "")
	return
}
