package tui

import (
	"context"

	"charm.land/bubbletea/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

type addDevelopment struct {
	ctx   context.Context
	dir   string
	owner string
	name  string
	db    *configdb.DB
	form  form
}

func newAddDevelopment(ctx context.Context, dir string, owner string, name string, db *configdb.DB, initial string) (s addDevelopment, cmd tea.Cmd) {
	s.ctx = ctx
	s.dir = dir
	s.owner = owner
	s.name = name
	s.db = db
	s.form, cmd = newForm("name: ")
	blank := initial == ""
	if !blank {
		s.form = s.form.setValue(0, initial)
	}
	return
}

func (s addDevelopment) Init() (cmd tea.Cmd) {
	return
}

func (s addDevelopment) release() {
	closeDB(s.db)
}

func (s addDevelopment) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
	sized := noteSize(msg)
	if sized {
		updated = s
		return
	}
	press, isPress := msg.(tea.KeyPressMsg)
	if !isPress {
		s.form, cmd = s.form.apply(msg)
		updated = s
		return
	}
	stroke := press.String()
	quitNow := stroke == "ctrl+c"
	if quitNow {
		updated = s
		cmd = tea.Quit
		return
	}
	escape := stroke == "esc"
	if escape {
		updated = newDevelopments(s.ctx, s.dir, s.owner, s.name, s.db, "", "")
		return
	}
	forward := movesForward(press)
	if forward {
		updated, cmd = s.advance()
		return
	}
	s.form, cmd = s.form.apply(press)
	updated = s
	return
}

func (s addDevelopment) View() (view tea.View) {
	trail := repoTrail(s.owner, s.name, "developments", "add")
	cursor := s.form.cursor()
	view = render(trail, s.form.body(), s.form.notice, formNextKeys, cursor)
	return
}

func (s addDevelopment) advance() (updated tea.Model, cmd tea.Cmd) {
	next, done, focus := s.form.advance()
	s.form = next
	if done {
		updated = s.next()
		return
	}
	updated = s
	cmd = focus
	return
}

func (s addDevelopment) next() (updated tea.Model) {
	raw := s.form.value(0)
	_, err := common.NormalizeSegment(raw)
	if err != nil {
		s.form.notice = err.Error()
		updated = s
		return
	}
	draft := developmentDraft{name: raw}
	updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, draft, pickStarting, "")
	return
}
