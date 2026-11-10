package tui

import (
	"context"
	"path/filepath"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/development"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

const (
	pickStarting int = iota
	pickAttention
	pickStandards
	pickSpecification
	pickBuilder
	pickFixer
)

type developmentDraft struct {
	name          string
	starting      string
	attention     string
	standards     string
	specification string
	builder       string
	fixer         string
}

type developmentPick struct {
	ctx    context.Context
	dir    string
	owner  string
	name   string
	db     *configdb.DB
	draft  developmentDraft
	step   int
	list   list.Model
	empty  string
	notice string
}

func newDevelopmentPick(ctx context.Context, dir string, owner string, name string, db *configdb.DB, draft developmentDraft, step int, notice string) (s developmentPick) {
	s.ctx = ctx
	s.dir = dir
	s.owner = owner
	s.name = name
	s.db = db
	s.draft = draft
	s.step = step
	s.notice = notice
	items, values, empty, err := s.choices()
	if err != nil {
		log.Error("list development choices", "err", err)
		s.notice = err.Error()
	}
	s.empty = empty
	heading := owner + "/" + name
	s.list = newChooser(heading, items, values, "")
	return
}

func (s developmentPick) choices() (items []string, values []string, empty string, err error) {
	label := s.step == pickStarting || s.step == pickAttention
	if label {
		found, listErr := s.db.Queries.ListLabels(s.ctx)
		err = listErr
		if err != nil {
			return
		}
		items = make([]string, 0, len(found))
		values = make([]string, 0, len(found))
		for _, row := range found {
			line := row.Name + "  " + row.Color
			items = append(items, line)
			values = append(values, row.Name)
		}
		empty = noLabels
		return
	}
	ids, err := s.db.Queries.ListGithubApps(s.ctx)
	if err != nil {
		return
	}
	lines := idLines(ids)
	items = lines
	values = lines
	empty = noApps
	return
}

func (s developmentPick) Init() (cmd tea.Cmd) {
	return
}

func (s developmentPick) release() {
	closeDB(s.db)
}

func (s developmentPick) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
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
		updated = s.choose()
	case actionBack:
		updated, cmd = s.back()
	case actionMove:
		s.list, cmd = s.list.Update(press)
		updated = s
	default:
		updated = s
	}
	return
}

func (s developmentPick) View() (view tea.View) {
	body := listBody(s.list, s.empty)
	title := pickTitle(s.step)
	trail := repoTrail(s.owner, s.name, "developments", "add", title)
	view = render(trail, body, s.notice, pickKeys, nil)
	return
}

func (s developmentPick) choose() (updated tea.Model) {
	item, ok := selectedRow(s.list)
	if !ok {
		updated = s
		return
	}
	switch s.step {
	case pickStarting:
		s.draft.starting = item.value
		updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, s.draft, pickAttention, "")
	case pickAttention:
		s.draft.attention = item.value
		updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, s.draft, pickStandards, "")
	case pickStandards:
		s.draft.standards = item.value
		updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, s.draft, pickSpecification, "")
	case pickSpecification:
		s.draft.specification = item.value
		updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, s.draft, pickBuilder, "")
	case pickBuilder:
		s.draft.builder = item.value
		updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, s.draft, pickFixer, "")
	default:
		s.draft.fixer = item.value
		updated = s.finish()
	}
	return
}

func (s developmentPick) finish() (updated tea.Model) {
	repoDir := filepath.Join(s.dir, "repos", s.owner, s.name)
	stored, err := development.Add(s.ctx, s.db, repoDir, s.draft.name, s.draft.starting, s.draft.attention, s.draft.standards, s.draft.specification, s.draft.builder, s.draft.fixer)
	if err != nil {
		s.notice = err.Error()
		updated = s
		return
	}
	updated = newDevelopments(s.ctx, s.dir, s.owner, s.name, s.db, stored, "")
	return
}

func (s developmentPick) back() (updated tea.Model, cmd tea.Cmd) {
	switch s.step {
	case pickStarting:
		updated, cmd = newAddDevelopment(s.ctx, s.dir, s.owner, s.name, s.db, s.draft.name)
	case pickAttention:
		updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, s.draft, pickStarting, "")
	case pickStandards:
		updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, s.draft, pickAttention, "")
	case pickSpecification:
		updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, s.draft, pickStandards, "")
	case pickBuilder:
		updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, s.draft, pickSpecification, "")
	default:
		updated = newDevelopmentPick(s.ctx, s.dir, s.owner, s.name, s.db, s.draft, pickBuilder, "")
	}
	return
}

func pickTitle(step int) (title string) {
	switch step {
	case pickStarting:
		title = "starting"
	case pickAttention:
		title = "attention"
	case pickStandards:
		title = "standards"
	case pickSpecification:
		title = "specification"
	case pickBuilder:
		title = "builder"
	default:
		title = "fixer"
	}
	return
}
