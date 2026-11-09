package tui

import (
	"context"
	"strconv"

	"charm.land/bubbletea/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/config"
)

type addApp struct {
	ctx  context.Context
	dir  string
	form form
}

func newAddApp(ctx context.Context, dir string) (s addApp, cmd tea.Cmd) {
	s.ctx = ctx
	s.dir = dir
	s.form, cmd = newForm("app id: ", "")
	return
}

func (s addApp) Init() (cmd tea.Cmd) {
	return
}

func (s addApp) release() {}

func (s addApp) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
		updated = newApps(s.ctx, s.dir, "", "")
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

func (s addApp) View() (view tea.View) {
	trail := []string{"gcf", "apps", "add"}
	cursor := s.form.cursor()
	view = render(trail, s.form.body(), s.form.notice, s.form.help(), cursor)
	return
}

func (s addApp) advance() (updated tea.Model, cmd tea.Cmd) {
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

func (s addApp) submit() (updated tea.Model) {
	raw := s.form.first.Value()
	id, err := config.CreateAppFile(s.dir, raw)
	if err != nil {
		s.form.notice = err.Error()
		updated = s
		return
	}
	text := strconv.FormatInt(id, 10)
	notice := appSecret(id)
	updated = newApps(s.ctx, s.dir, text, notice)
	return
}
