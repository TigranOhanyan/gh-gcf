package tui

import (
	"context"

	"charm.land/bubbletea/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/repo"
)

type addRepo struct {
	ctx  context.Context
	dir  string
	form form
}

func newAddRepo(ctx context.Context, dir string) (s addRepo, cmd tea.Cmd) {
	s.ctx = ctx
	s.dir = dir
	s.form, cmd = newForm("owner: ", "name: ")
	return
}

func (s addRepo) Init() (cmd tea.Cmd) {
	return
}

func (s addRepo) release() {}

func (s addRepo) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
		updated = newRepos(s.ctx, s.dir, "")
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

func (s addRepo) View() (view tea.View) {
	trail := []string{"gcf", "repos", "add"}
	cursor := s.form.cursor()
	view = render(trail, s.form.body(), s.form.notice, s.form.help(), cursor)
	return
}

func (s addRepo) advance() (updated tea.Model, cmd tea.Cmd) {
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

func (s addRepo) submit() (updated tea.Model) {
	rawOwner := s.form.value(0)
	rawName := s.form.value(1)
	db, err := repo.Create(s.ctx, s.dir, rawOwner, rawName)
	if err != nil {
		s.form.notice = err.Error()
		updated = s
		return
	}
	closeDB(db)
	owner, ownerErr := common.NormalizeSegment(rawOwner)
	name, nameErr := common.NormalizeSegment(rawName)
	key := ""
	named := ownerErr == nil && nameErr == nil
	if named {
		key = owner + "/" + name
	}
	updated = newRepos(s.ctx, s.dir, key)
	return
}
