package tui

import (
	"context"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/config"
)

type repos struct {
	ctx    context.Context
	dir    string
	list   list.Model
	notice string
}

func newRepos(ctx context.Context, dir string, selectKey string) (s repos) {
	s.ctx = ctx
	s.dir = dir
	found, err := config.ListRepos(dir)
	if err != nil {
		log.Error("list repos", "err", err)
		s.notice = err.Error()
	}
	items := make([]string, 0, len(found))
	values := make([]string, 0, len(found))
	for _, repo := range found {
		line := config.RepoKey(repo)
		missing := !repo.HasConfig
		if missing {
			line = line + "  " + config.MissingConfig
		}
		items = append(items, line)
		values = append(values, config.RepoKey(repo))
	}
	s.list = newChooser("Repos", items, values, selectKey)
	return
}

func (s repos) Init() (cmd tea.Cmd) {
	return
}

func (s repos) release() {}

func (s repos) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
		updated = newHome(s.ctx, s.dir)
	case actionMove:
		s.list, cmd = s.list.Update(press)
		updated = s
	default:
		updated = s
	}
	return
}

func (s repos) View() (view tea.View) {
	body := listBody(s.list, noRepos)
	trail := []string{"gcf", "repos"}
	view = render(trail, body, s.notice, listKeys, nil)
	return
}

func (s repos) add() (updated tea.Model, cmd tea.Cmd) {
	next, focus := newAddRepo(s.ctx, s.dir)
	updated = next
	cmd = focus
	return
}

func (s repos) open() (updated tea.Model) {
	item, ok := selectedRow(s.list)
	if !ok {
		updated = s
		return
	}
	owner, name, ok := splitRepo(item.value)
	if !ok {
		updated = s
		return
	}
	updated = newMenu(s.ctx, s.dir, owner, name)
	return
}

func splitRepo(key string) (owner string, name string, ok bool) {
	slash := -1
	for i := 0; i < len(key); i++ {
		found := key[i] == '/'
		if found {
			slash = i
			break
		}
	}
	missing := slash <= 0 || slash >= len(key)-1
	if missing {
		return
	}
	owner = key[:slash]
	name = key[slash+1:]
	ok = true
	return
}
