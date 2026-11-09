package tui

import (
	"context"
	"strconv"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/config"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

type pick struct {
	ctx    context.Context
	dir    string
	owner  string
	name   string
	db     *configdb.DB
	list   list.Model
	notice string
}

func newPick(ctx context.Context, dir string, owner string, name string, db *configdb.DB) (s pick) {
	s.ctx = ctx
	s.dir = dir
	s.owner = owner
	s.name = name
	s.db = db
	ids, err := commands.ListAppIDs(dir)
	if err != nil {
		log.Error("list apps", "err", err)
		s.notice = err.Error()
	}
	lines := idLines(ids)
	s.list = newChooser("Apps", lines, lines, "")
	return
}

func (s pick) Init() (cmd tea.Cmd) {
	return
}

func (s pick) release() {
	closeDB(s.db)
}

func (s pick) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
	case actionOpen:
		updated = s.open()
	case actionBack:
		updated = newRepoApps(s.ctx, s.dir, s.owner, s.name, s.db, "", "")
	case actionMove:
		s.list, cmd = s.list.Update(press)
		updated = s
	default:
		updated = s
	}
	return
}

func (s pick) View() (view tea.View) {
	body := listBody(s.list, noApps)
	trail := repoTrail(s.owner, s.name, "apps", "add")
	view = render(trail, body, s.notice, pickKeys, nil)
	return
}

func (s pick) open() (updated tea.Model) {
	item, ok := selectedRow(s.list)
	if !ok {
		updated = s
		return
	}
	id, err := strconv.ParseInt(item.value, 10, 64)
	if err != nil {
		s.notice = err.Error()
		updated = s
		return
	}
	err = config.AttachApp(s.ctx, s.db, id)
	if err != nil {
		s.notice = err.Error()
		updated = s
		return
	}
	text := strconv.FormatInt(id, 10)
	notice := appSecret(id)
	updated = newRepoApps(s.ctx, s.dir, s.owner, s.name, s.db, text, notice)
	return
}
