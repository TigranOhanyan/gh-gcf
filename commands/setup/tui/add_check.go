package tui

import (
	"context"

	"charm.land/bubbletea/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/development"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

type addCheck struct {
	ctx         context.Context
	dir         string
	owner       string
	name        string
	development string
	db          *configdb.DB
	form        form
}

func newAddCheck(ctx context.Context, dir string, owner string, name string, db *configdb.DB, developmentName string) (s addCheck, cmd tea.Cmd) {
	s.ctx = ctx
	s.dir = dir
	s.owner = owner
	s.name = name
	s.development = developmentName
	s.db = db
	s.form, cmd = newForm("name: ")
	return
}

func (s addCheck) Init() (cmd tea.Cmd) {
	return
}

func (s addCheck) release() {
	closeDB(s.db)
}

func (s addCheck) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
		updated = newChecks(s.ctx, s.dir, s.owner, s.name, s.db, s.development, "", "")
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

func (s addCheck) View() (view tea.View) {
	trail := repoTrail(s.owner, s.name, "developments", s.development, "add")
	cursor := s.form.cursor()
	view = render(trail, s.form.body(), s.form.notice, s.form.help(), cursor)
	return
}

func (s addCheck) advance() (updated tea.Model, cmd tea.Cmd) {
	next, done, focus := s.form.advance()
	s.form = next
	if done {
		updated = s.submit()
		return
	}
	updated = s
	cmd = focus
	return
}

func (s addCheck) submit() (updated tea.Model) {
	rawName := s.form.value(0)
	stored, err := development.AddCheck(s.ctx, s.db, s.development, rawName)
	if err != nil {
		s.form.notice = err.Error()
		updated = s
		return
	}
	updated = newChecks(s.ctx, s.dir, s.owner, s.name, s.db, s.development, stored, "")
	return
}
