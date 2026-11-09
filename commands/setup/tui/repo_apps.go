package tui

import (
	"context"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/configdb"
)

type repoApps struct {
	ctx    context.Context
	dir    string
	owner  string
	name   string
	db     *configdb.DB
	list   list.Model
	notice string
}

func newRepoApps(ctx context.Context, dir string, owner string, name string, db *configdb.DB, selectID string, notice string) (s repoApps) {
	s.ctx = ctx
	s.dir = dir
	s.owner = owner
	s.name = name
	s.db = db
	s.notice = notice
	ids, err := db.Queries.ListGithubApps(ctx)
	if err != nil {
		log.Error("list apps", "err", err)
		s.notice = err.Error()
	}
	lines := idLines(ids)
	heading := owner + "/" + name
	s.list = newChooser(heading, lines, lines, selectID)
	return
}

func (s repoApps) Init() (cmd tea.Cmd) {
	return
}

func (s repoApps) release() {
	closeDB(s.db)
}

func (s repoApps) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
		updated = newPick(s.ctx, s.dir, s.owner, s.name, s.db)
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

func (s repoApps) View() (view tea.View) {
	body := listBody(s.list, noApps)
	trail := repoTrail(s.owner, s.name, "apps")
	view = render(trail, body, s.notice, listKeys, nil)
	return
}

func (s repoApps) back() (updated tea.Model) {
	closeDB(s.db)
	updated = newMenu(s.ctx, s.dir, s.owner, s.name)
	return
}
