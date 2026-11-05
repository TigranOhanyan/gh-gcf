package setup

import (
	"strconv"

	"charm.land/log/v2"

	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func (m model) showBuilders() (updated model) {
	updated = m
	updated.place.screen = screenBuilders
	updated.place.builder = ""
	var listErr error
	updated, listErr = updated.reloadBuilders()
	if listErr != nil {
		updated.notice = listErr.Error()
	}
	items, values := builderLines(updated.builders)
	updated = updated.setList("Builders", items, values, 0, listHelp)
	return
}

func (m model) showBuilder() (updated model) {
	updated = m
	updated.place.screen = screenBuilder
	items := []string{"reviewers", "fixer", "checks"}
	values := []string{"reviewers", "fixer", "checks"}
	updated = updated.setList(updated.place.builder, items, values, 0, menuHelp)
	return
}

func (m model) showReviewers() (updated model) {
	updated = m
	updated.place.screen = screenReviewers
	var listErr error
	updated, listErr = updated.reloadReviewers()
	if listErr != nil {
		updated.notice = listErr.Error()
	}
	items := make([]string, 0, len(updated.reviewers))
	values := make([]string, 0, len(updated.reviewers))
	for _, row := range updated.reviewers {
		app := strconv.FormatInt(row.GithubApp, 10)
		line := row.Name + "  " + app
		items = append(items, line)
		values = append(values, row.Name)
	}
	updated = updated.setList("Reviewers", items, values, 0, listHelp)
	return
}

func (m model) showFixer() (updated model) {
	updated = m
	updated.place.screen = screenFixer
	var listErr error
	updated, listErr = updated.reloadFixers()
	if listErr != nil {
		updated.notice = listErr.Error()
	}
	items := make([]string, 0, len(updated.fixers))
	values := make([]string, 0, len(updated.fixers))
	for _, row := range updated.fixers {
		line := row.Name + "  " + row.AttentionLabel
		items = append(items, line)
		values = append(values, row.Name)
	}
	updated = updated.setList("Fixer", items, values, 0, listHelp)
	return
}

func (m model) showChecks() (updated model) {
	updated = m
	updated.place.screen = screenChecks
	var listErr error
	updated, listErr = updated.reloadChecks()
	if listErr != nil {
		updated.notice = listErr.Error()
	}
	updated = updated.setList("Checks", updated.checks, updated.checks, 0, listHelp)
	return
}

func (m model) showPickStart() (updated model) {
	updated = m
	updated.place.screen = screenPickStart
	updated.notice = ""
	missing := updated.db == nil
	if missing {
		updated.notice = missingConfig
		updated = updated.setList("Starting label", nil, nil, 0, pickHelp)
		return
	}
	var err error
	updated, err = updated.reloadLabels()
	if err != nil {
		updated.notice = err.Error()
		updated = updated.setList("Starting label", nil, nil, 0, pickHelp)
		return
	}
	used, err := updated.db.Queries.ListStartingLabels(updated.ctx)
	if err != nil {
		log.Error("list starting labels", "err", err)
		updated.notice = err.Error()
		used = nil
	}
	names := labelsExcept(updated.labels, used)
	updated = updated.setList("Starting label", names, names, 0, pickHelp)
	return
}

func (m model) showPickAttention() (updated model) {
	updated = m
	updated.place.screen = screenPickAttention
	updated.notice = ""
	var err error
	updated, err = updated.reloadLabels()
	if err != nil {
		updated.notice = err.Error()
	}
	skip := []string{updated.draftStart}
	names := labelsExcept(updated.labels, skip)
	updated = updated.setList("Attention label", names, names, 0, pickHelp)
	return
}

func (m model) showPickReviewerApp() (updated model) {
	updated = m
	updated.place.screen = screenPickReviewerApp
	updated.notice = ""
	missing := updated.db == nil
	if missing {
		updated.notice = missingConfig
		updated = updated.setList("Reviewer app", nil, nil, 0, pickHelp)
		return
	}
	var err error
	updated, err = updated.reloadAttached()
	if err != nil {
		updated.notice = err.Error()
	}
	used, err := updated.db.Queries.ListReviewerApps(updated.ctx)
	if err != nil {
		log.Error("list reviewer apps", "err", err)
		updated.notice = err.Error()
		used = nil
	}
	free := unusedApps(updated.attached, used)
	items := idLines(free)
	updated = updated.setList("Reviewer app", items, items, 0, pickHelp)
	return
}

func (m model) showPickReviewerLabel() (updated model) {
	updated = m.showLabelPick(screenPickReviewerLabel)
	return
}

func (m model) showPickFixerLabel() (updated model) {
	updated = m.showLabelPick(screenPickFixerLabel)
	return
}

func (m model) showLabelPick(next screen) (updated model) {
	updated = m
	updated.place.screen = next
	updated.notice = ""
	var err error
	updated, err = updated.reloadLabels()
	if err != nil {
		updated.notice = err.Error()
	}
	names := labelNames(updated.labels)
	updated = updated.setList("Attention label", names, names, 0, pickHelp)
	return
}

func (m model) openBuilder() (updated model) {
	updated = m
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	updated.place.builder = item.value
	updated.notice = ""
	updated = updated.showBuilder()
	return
}

func (m model) openBuilderSection() (updated model) {
	updated = m
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	switch item.value {
	case "reviewers":
		updated = updated.showReviewers()
	case "fixer":
		updated = updated.showFixer()
	case "checks":
		updated = updated.showChecks()
	}
	return
}

func (m model) draftBuilder() (updated model) {
	updated = m
	name, err := normalizeMemberName(updated.first.Value())
	if err != nil {
		updated.notice = err.Error()
		return
	}
	err = refuseTakenMember(updated.ctx, updated.db, name)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.draftName = name
	updated.notice = ""
	updated = updated.showPickStart()
	return
}

func (m model) draftReviewer() (updated model) {
	updated = m
	name, err := normalizeMemberName(updated.first.Value())
	if err != nil {
		updated.notice = err.Error()
		return
	}
	err = refuseTakenMember(updated.ctx, updated.db, name)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.draftName = name
	updated.notice = ""
	updated = updated.showPickReviewerApp()
	return
}

func (m model) draftFixer() (updated model) {
	updated = m
	name, err := normalizeMemberName(updated.first.Value())
	if err != nil {
		updated.notice = err.Error()
		return
	}
	err = refuseTakenMember(updated.ctx, updated.db, name)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.draftName = name
	updated.notice = ""
	updated = updated.showPickFixerLabel()
	return
}

func (m model) chooseStart() (updated model) {
	updated = m
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	updated.draftStart = item.value
	updated = updated.showPickAttention()
	return
}

func (m model) chooseAttention() (updated model) {
	updated = m
	missing := updated.db == nil
	if missing {
		updated.notice = missingConfig
		return
	}
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	name, err := addBuilderMember(updated.ctx, updated.db, updated.factoryDir, updated.place.owner, updated.place.name, updated.draftName, updated.draftStart, item.value)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.notice = ""
	updated = updated.showBuilders()
	index := indexOfBuilder(updated.builders, name)
	found := index >= 0
	if found {
		updated.list.Select(index)
	}
	return
}

func (m model) chooseReviewerApp() (updated model) {
	updated = m
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(item.value, 10, 64)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.draftApp = id
	updated = updated.showPickReviewerLabel()
	return
}

func (m model) chooseReviewerLabel() (updated model) {
	updated = m
	missing := updated.db == nil
	if missing {
		updated.notice = missingConfig
		return
	}
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	name, err := addReviewerMember(updated.ctx, updated.db, updated.factoryDir, updated.place.owner, updated.place.name, updated.place.builder, updated.draftName, updated.draftApp, item.value)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.notice = ""
	updated = updated.showReviewers()
	index := indexOfReviewer(updated.reviewers, name)
	found := index >= 0
	if found {
		updated.list.Select(index)
	}
	return
}

func (m model) chooseFixerLabel() (updated model) {
	updated = m
	missing := updated.db == nil
	if missing {
		updated.notice = missingConfig
		return
	}
	item, ok := selectedRow(updated.list)
	if !ok {
		return
	}
	_, err := addFixerMember(updated.ctx, updated.db, updated.factoryDir, updated.place.owner, updated.place.name, updated.place.builder, updated.draftName, item.value)
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.notice = ""
	updated = updated.showFixer()
	return
}

func (m model) saveCheck() (updated model) {
	updated = m
	missing := updated.db == nil
	if missing {
		updated.notice = missingConfig
		return
	}
	name, err := addGithubCheck(updated.ctx, updated.db, updated.place.builder, updated.first.Value())
	if err != nil {
		updated.notice = err.Error()
		return
	}
	updated.notice = ""
	updated = updated.showChecks()
	index := indexOfString(updated.checks, name)
	found := index >= 0
	if found {
		updated.list.Select(index)
	}
	return
}

func (m model) reloadBuilders() (updated model, err error) {
	updated = m
	if updated.db == nil {
		updated.builders = nil
		return
	}
	rows, err := updated.db.Queries.ListBuilderMembers(updated.ctx)
	if err != nil {
		log.Error("list builders", "err", err)
		return
	}
	updated.builders = rows
	return
}

func (m model) reloadReviewers() (updated model, err error) {
	updated = m
	if updated.db == nil {
		updated.reviewers = nil
		return
	}
	rows, err := updated.db.Queries.ListReviewerMembers(updated.ctx, updated.place.builder)
	if err != nil {
		log.Error("list reviewers", "err", err)
		return
	}
	updated.reviewers = rows
	return
}

func (m model) reloadFixers() (updated model, err error) {
	updated = m
	if updated.db == nil {
		updated.fixers = nil
		return
	}
	rows, err := updated.db.Queries.ListFixerMembers(updated.ctx, updated.place.builder)
	if err != nil {
		log.Error("list fixers", "err", err)
		return
	}
	updated.fixers = rows
	return
}

func (m model) reloadChecks() (updated model, err error) {
	updated = m
	if updated.db == nil {
		updated.checks = nil
		return
	}
	names, err := updated.db.Queries.ListGithubChecks(updated.ctx, updated.place.builder)
	if err != nil {
		log.Error("list checks", "err", err)
		return
	}
	updated.checks = names
	return
}

func builderLines(rows []configrecords.ListBuilderMembersRow) (items []string, values []string) {
	items = make([]string, 0, len(rows))
	values = make([]string, 0, len(rows))
	for _, row := range rows {
		line := row.Name + "  " + row.StartingLabel + "  " + row.AttentionLabel
		items = append(items, line)
		values = append(values, row.Name)
	}
	return
}

func labelNames(labels []configrecords.Label) (names []string) {
	names = make([]string, 0, len(labels))
	for _, label := range labels {
		names = append(names, label.Name)
	}
	return
}

func labelsExcept(labels []configrecords.Label, skip []string) (names []string) {
	names = make([]string, 0, len(labels))
	for _, label := range labels {
		blocked := false
		for _, name := range skip {
			same := label.Name == name
			if same {
				blocked = true
			}
		}
		if blocked {
			continue
		}
		names = append(names, label.Name)
	}
	return
}

func unusedApps(attached []int64, used []int64) (free []int64) {
	free = make([]int64, 0)
	for _, id := range attached {
		taken := false
		for _, existing := range used {
			same := existing == id
			if same {
				taken = true
			}
		}
		if taken {
			continue
		}
		free = append(free, id)
	}
	return
}

func indexOfBuilder(rows []configrecords.ListBuilderMembersRow, name string) (index int) {
	index = -1
	for i, row := range rows {
		same := row.Name == name
		if same {
			index = i
			return
		}
	}
	return
}

func indexOfReviewer(rows []configrecords.ListReviewerMembersRow, name string) (index int) {
	index = -1
	for i, row := range rows {
		same := row.Name == name
		if same {
			index = i
			return
		}
	}
	return
}

func indexOfString(names []string, name string) (index int) {
	index = -1
	for i, existing := range names {
		same := existing == name
		if same {
			index = i
			return
		}
	}
	return
}
