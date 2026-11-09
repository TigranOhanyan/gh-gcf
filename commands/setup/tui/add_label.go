package tui

import (
	"context"

	"charm.land/bubbletea/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/label"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

type addLabel struct {
	ctx   context.Context
	dir   string
	owner string
	name  string
	db    *configdb.DB
	form  form
}

func newAddLabel(ctx context.Context, dir string, owner string, name string, db *configdb.DB) (s addLabel, cmd tea.Cmd) {
	s.ctx = ctx
	s.dir = dir
	s.owner = owner
	s.name = name
	s.db = db
	s.form, cmd = newForm("name: ", "color: ")
	return
}

func (s addLabel) Init() (cmd tea.Cmd) {
	return
}

func (s addLabel) release() {
	closeDB(s.db)
}

func (s addLabel) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
		updated = newLabels(s.ctx, s.dir, s.owner, s.name, s.db, "", "")
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

func (s addLabel) View() (view tea.View) {
	trail := repoTrail(s.owner, s.name, "labels", "add")
	cursor := s.form.cursor()
	view = render(trail, s.form.body(), s.form.notice, s.form.help(), cursor)
	return
}

func (s addLabel) advance() (updated tea.Model, cmd tea.Cmd) {
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

func (s addLabel) submit() (updated tea.Model) {
	rawName := s.form.value(0)
	rawColor := s.form.value(1)
	stored, err := label.Add(s.ctx, s.db, rawName, rawColor)
	if err != nil {
		s.form.notice = err.Error()
		updated = s
		return
	}
	updated = newLabels(s.ctx, s.dir, s.owner, s.name, s.db, stored, "")
	return
}
