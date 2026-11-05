package compile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "compile"
const roleBuilder = "builder"
const roleReviewer = "reviewer"
const roleFixer = "fixer"

type compileStep struct{}

func (compileStep) Name() (name string) {
	name = stepName
	return
}

func (step compileStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, _ records.Step) (message string, err error) {
	builders, err := repo.Config.Queries.ListBuilderMembers(ctx)
	if err != nil {
		log.Error("list builders", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}

	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin compile", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)

	var lines []string
	for _, builder := range builders {
		added, copyErr := copyBuilder(ctx, repo, queries, builder)
		failed := copyErr != nil
		if failed {
			err = fmt.Errorf("[%s] %s", builder.Name, copyErr.Error())
			err = commands.FinishTx(tx, err)
			return
		}
		lines = append(lines, added...)
	}

	err = commands.FinishTx(tx, nil)
	if err != nil {
		return
	}
	message = strings.Join(lines, "\n")
	return
}

func copyBuilder(ctx context.Context, repo commands.Repo, queries *records.Queries, builder configrecords.ListBuilderMembersRow) (lines []string, err error) {
	sum, err := commands.InstructionFingerprint(repo.Dir, builder.Name)
	if err != nil {
		return
	}
	memberLine, err := saveMember(ctx, queries, builder.Name, roleBuilder, builder.AttentionLabel, sum)
	if err != nil {
		return
	}
	builderLine, err := saveBuilderMember(ctx, queries, builder.Name, builder.StartingLabel, memberLine)
	if err != nil {
		return
	}
	lines = appendLine(lines, memberLine)
	lines = appendLine(lines, builderLine)

	reviewers, err := repo.Config.Queries.ListReviewerMembers(ctx, builder.Name)
	if err != nil {
		log.Error("list reviewers", "builder", builder.Name, "err", err)
		return
	}
	for _, reviewer := range reviewers {
		var added []string
		added, err = copyReviewer(ctx, repo.Dir, queries, builder.Name, reviewer)
		if err != nil {
			return
		}
		lines = append(lines, added...)
	}

	fixers, err := repo.Config.Queries.ListFixerMembers(ctx, builder.Name)
	if err != nil {
		log.Error("list fixers", "builder", builder.Name, "err", err)
		return
	}
	for _, fixer := range fixers {
		var added []string
		added, err = copyFixer(ctx, repo.Dir, queries, builder.Name, fixer)
		if err != nil {
			return
		}
		lines = append(lines, added...)
	}

	checks, err := repo.Config.Queries.ListGithubChecks(ctx, builder.Name)
	if err != nil {
		log.Error("list checks", "builder", builder.Name, "err", err)
		return
	}
	for _, check := range checks {
		var line string
		line, err = saveCheck(ctx, queries, builder.Name, check)
		if err != nil {
			return
		}
		lines = appendLine(lines, line)
	}
	return
}

func copyReviewer(ctx context.Context, dir string, queries *records.Queries, builderName string, reviewer configrecords.ListReviewerMembersRow) (lines []string, err error) {
	sum, err := commands.InstructionFingerprint(dir, reviewer.Name)
	if err != nil {
		return
	}
	memberLine, err := saveMember(ctx, queries, reviewer.Name, roleReviewer, reviewer.AttentionLabel, sum)
	if err != nil {
		return
	}
	rowLine, err := saveReviewerMember(ctx, queries, reviewer.Name, builderName, reviewer.GithubApp, memberLine)
	if err != nil {
		return
	}
	lines = appendLine(lines, memberLine)
	lines = appendLine(lines, rowLine)
	return
}

func copyFixer(ctx context.Context, dir string, queries *records.Queries, builderName string, fixer configrecords.ListFixerMembersRow) (lines []string, err error) {
	sum, err := commands.InstructionFingerprint(dir, fixer.Name)
	if err != nil {
		return
	}
	memberLine, err := saveMember(ctx, queries, fixer.Name, roleFixer, fixer.AttentionLabel, sum)
	if err != nil {
		return
	}
	rowLine, err := saveFixerMember(ctx, queries, fixer.Name, builderName, memberLine)
	if err != nil {
		return
	}
	lines = appendLine(lines, memberLine)
	lines = appendLine(lines, rowLine)
	return
}

func saveMember(ctx context.Context, queries *records.Queries, name string, role string, attention string, fingerprint string) (line string, err error) {
	stored, err := queries.GetMember(ctx, name)
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		log.Error("get member", "member", name, "err", err)
		return
	}
	err = nil

	if absent {
		params := records.CreateMemberParams{
			Name:              name,
			Role:              role,
			AttentionLabel:    attention,
			InstructionSha256: fingerprint,
		}
		err = queries.CreateMember(ctx, params)
		if err != nil {
			log.Error("insert member", "member", name, "err", err)
			return
		}
		line = fmt.Sprintf("[%s] recorded", name)
		return
	}

	wrongRole := stored.Role != role
	if wrongRole {
		err = fmt.Errorf("member %s is %s", name, stored.Role)
		log.Error("member role", "member", name, "role", stored.Role, "err", err)
		return
	}
	sameAttention := stored.AttentionLabel == attention
	sameHash := stored.InstructionSha256 == fingerprint
	same := sameAttention && sameHash
	if same {
		return
	}

	params := records.UpdateMemberParams{
		AttentionLabel:    attention,
		InstructionSha256: fingerprint,
		Name:              name,
	}
	err = queries.UpdateMember(ctx, params)
	if err != nil {
		log.Error("update member", "member", name, "err", err)
		return
	}
	line = fmt.Sprintf("[%s] updated", name)
	return
}

func saveBuilderMember(ctx context.Context, queries *records.Queries, name string, starting string, memberLine string) (line string, err error) {
	stored, err := queries.GetBuilderMember(ctx, name)
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		log.Error("get builder", "member", name, "err", err)
		return
	}
	err = nil

	if absent {
		params := records.CreateBuilderMemberParams{
			Name:          name,
			StartingLabel: starting,
		}
		err = queries.CreateBuilderMember(ctx, params)
		if err != nil {
			log.Error("insert builder", "member", name, "err", err)
			return
		}
		line = recordedLine(name, memberLine)
		return
	}

	same := stored.StartingLabel == starting
	if same {
		return
	}
	params := records.UpdateBuilderStartingLabelParams{
		StartingLabel: starting,
		Name:          name,
	}
	err = queries.UpdateBuilderStartingLabel(ctx, params)
	if err != nil {
		log.Error("update builder", "member", name, "err", err)
		return
	}
	line = changedLine(name, memberLine)
	return
}

func saveReviewerMember(ctx context.Context, queries *records.Queries, name string, builderName string, appID int64, memberLine string) (line string, err error) {
	stored, err := queries.GetReviewerMember(ctx, name)
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		log.Error("get reviewer", "member", name, "err", err)
		return
	}
	err = nil

	if absent {
		params := records.CreateReviewerMemberParams{
			Name:        name,
			BuilderName: builderName,
			GithubApp:   appID,
		}
		err = queries.CreateReviewerMember(ctx, params)
		if err != nil {
			log.Error("insert reviewer", "member", name, "err", err)
			return
		}
		line = recordedLine(name, memberLine)
		return
	}

	sameBuilder := stored.BuilderName == builderName
	sameApp := stored.GithubApp == appID
	same := sameBuilder && sameApp
	if same {
		return
	}
	params := records.UpdateReviewerMemberParams{
		BuilderName: builderName,
		GithubApp:   appID,
		Name:        name,
	}
	err = queries.UpdateReviewerMember(ctx, params)
	if err != nil {
		log.Error("update reviewer", "member", name, "err", err)
		return
	}
	line = changedLine(name, memberLine)
	return
}

func saveFixerMember(ctx context.Context, queries *records.Queries, name string, builderName string, memberLine string) (line string, err error) {
	stored, err := queries.GetFixerMember(ctx, name)
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		log.Error("get fixer", "member", name, "err", err)
		return
	}
	err = nil

	if absent {
		params := records.CreateFixerMemberParams{
			Name:        name,
			BuilderName: builderName,
		}
		err = queries.CreateFixerMember(ctx, params)
		if err != nil {
			log.Error("insert fixer", "member", name, "err", err)
			return
		}
		line = recordedLine(name, memberLine)
		return
	}

	same := stored.BuilderName == builderName
	if same {
		return
	}
	params := records.UpdateFixerBuilderParams{
		BuilderName: builderName,
		Name:        name,
	}
	err = queries.UpdateFixerBuilder(ctx, params)
	if err != nil {
		log.Error("update fixer", "member", name, "err", err)
		return
	}
	line = changedLine(name, memberLine)
	return
}

func saveCheck(ctx context.Context, queries *records.Queries, builderName string, name string) (line string, err error) {
	params := records.GetGithubCheckParams{
		BuilderName: builderName,
		Name:        name,
	}
	_, err = queries.GetGithubCheck(ctx, params)
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		log.Error("get check", "builder", builderName, "check", name, "err", err)
		return
	}
	err = nil
	if !absent {
		return
	}

	create := records.CreateGithubCheckParams{
		BuilderName: builderName,
		Name:        name,
	}
	err = queries.CreateGithubCheck(ctx, create)
	if err != nil {
		log.Error("insert check", "builder", builderName, "check", name, "err", err)
		return
	}
	line = fmt.Sprintf("[%s] [%s] recorded", builderName, name)
	return
}

func recordedLine(name string, memberLine string) (line string) {
	if memberLine != "" {
		return
	}
	line = fmt.Sprintf("[%s] recorded", name)
	return
}

func changedLine(name string, memberLine string) (line string) {
	if memberLine != "" {
		return
	}
	line = fmt.Sprintf("[%s] updated", name)
	return
}

func appendLine(lines []string, line string) (next []string) {
	next = lines
	if line == "" {
		return
	}
	next = append(next, line)
	return
}
