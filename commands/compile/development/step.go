package development

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/logging"

	"github.com/TigranOhanyan/gh-gcf/commands"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "compile_development"
const roleBuilder = "builder"
const roleStandards = "standards_reviewer"
const roleSpecification = "specification_reviewer"
const roleFixer = "fixer"

type compileStep struct{}

type instructionSums struct {
	builder       string
	standards     string
	specification string
	fixer         string
}

func (compileStep) Name() (name string) {
	name = stepName
	return
}

func (step compileStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, _ records.Step) (message string, err error) {
	items, err := repo.Config.Queries.ListDevelopments(ctx)
	if err != nil {
		logging.From(ctx).Error("list developments", zap.Any("owner", repo.Owner), zap.Any("name", repo.Name), zap.Error(err))
		return
	}

	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		logging.From(ctx).Error("begin compile", zap.Error(err))
		return
	}
	queries := repo.State.Queries.WithTx(tx)

	var lines []string
	for _, item := range items {
		added, copyErr := copyDevelopment(ctx, repo, queries, item)
		failed := copyErr != nil
		if failed {
			err = fmt.Errorf("[%s] %s", item.Name, copyErr.Error())
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

func copyDevelopment(ctx context.Context, repo commands.Repo, queries *records.Queries, item configrecords.Development) (lines []string, err error) {
	sums, err := readSums(repo.Dir, item.Name)
	if err != nil {
		return
	}
	rowLine, err := saveDevelopment(ctx, queries, item, sums)
	if err != nil {
		return
	}
	lines = appendLine(lines, rowLine)

	checks, err := repo.Config.Queries.ListDevelopmentChecks(ctx, item.Name)
	if err != nil {
		logging.From(ctx).Error("list checks", zap.Any("development", item.Name), zap.Error(err))
		return
	}
	for _, check := range checks {
		var line string
		line, err = saveCheck(ctx, queries, item.Name, check)
		if err != nil {
			return
		}
		lines = appendLine(lines, line)
	}
	return
}

func readSums(dir string, name string) (sums instructionSums, err error) {
	sums.builder, err = commands.DevelopmentInstructionFingerprint(dir, name, roleBuilder)
	if err != nil {
		return
	}
	sums.standards, err = commands.DevelopmentInstructionFingerprint(dir, name, roleStandards)
	if err != nil {
		return
	}
	sums.specification, err = commands.DevelopmentInstructionFingerprint(dir, name, roleSpecification)
	if err != nil {
		return
	}
	sums.fixer, err = commands.DevelopmentInstructionFingerprint(dir, name, roleFixer)
	return
}

func saveDevelopment(ctx context.Context, queries *records.Queries, item configrecords.Development, sums instructionSums) (line string, err error) {
	stored, err := queries.GetDevelopment(ctx, item.Name)
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		logging.From(ctx).Error("get development", zap.Any("development", item.Name), zap.Error(err))
		return
	}
	err = nil

	if absent {
		params := records.CreateDevelopmentParams{
			Name:                                   item.Name,
			StartingLabel:                          item.StartingLabel,
			AttentionLabel:                         item.AttentionLabel,
			StandardsReviewerApp:                   item.StandardsReviewerApp,
			SpecificationReviewerApp:               item.SpecificationReviewerApp,
			BuilderApp:                             item.BuilderApp,
			FixerApp:                               item.FixerApp,
			BuilderInstructionSha256:               sums.builder,
			StandardsReviewerInstructionSha256:     sums.standards,
			SpecificationReviewerInstructionSha256: sums.specification,
			FixerInstructionSha256:                 sums.fixer,
		}
		err = queries.CreateDevelopment(ctx, params)
		if err != nil {
			logging.From(ctx).Error("insert development", zap.Any("development", item.Name), zap.Error(err))
			return
		}
		line = fmt.Sprintf("[%s] recorded", item.Name)
		return
	}

	same := sameDevelopment(stored, item, sums)
	if same {
		return
	}
	params := records.UpdateDevelopmentParams{
		StartingLabel:                          item.StartingLabel,
		AttentionLabel:                         item.AttentionLabel,
		StandardsReviewerApp:                   item.StandardsReviewerApp,
		SpecificationReviewerApp:               item.SpecificationReviewerApp,
		BuilderApp:                             item.BuilderApp,
		FixerApp:                               item.FixerApp,
		BuilderInstructionSha256:               sums.builder,
		StandardsReviewerInstructionSha256:     sums.standards,
		SpecificationReviewerInstructionSha256: sums.specification,
		FixerInstructionSha256:                 sums.fixer,
		Name:                                   item.Name,
	}
	err = queries.UpdateDevelopment(ctx, params)
	if err != nil {
		logging.From(ctx).Error("update development", zap.Any("development", item.Name), zap.Error(err))
		return
	}
	line = fmt.Sprintf("[%s] updated", item.Name)
	return
}

func sameDevelopment(stored records.Development, item configrecords.Development, sums instructionSums) (same bool) {
	sameStart := stored.StartingLabel == item.StartingLabel
	sameAttention := stored.AttentionLabel == item.AttentionLabel
	sameStandardsApp := stored.StandardsReviewerApp == item.StandardsReviewerApp
	sameSpecificationApp := stored.SpecificationReviewerApp == item.SpecificationReviewerApp
	sameBuilderApp := stored.BuilderApp == item.BuilderApp
	sameFixerApp := stored.FixerApp == item.FixerApp
	sameBuilder := stored.BuilderInstructionSha256 == sums.builder
	sameStandards := stored.StandardsReviewerInstructionSha256 == sums.standards
	sameSpecification := stored.SpecificationReviewerInstructionSha256 == sums.specification
	sameFixer := stored.FixerInstructionSha256 == sums.fixer
	same = sameStart && sameAttention && sameStandardsApp && sameSpecificationApp && sameBuilderApp && sameFixerApp && sameBuilder && sameStandards && sameSpecification && sameFixer
	return
}

func saveCheck(ctx context.Context, queries *records.Queries, developmentName string, name string) (line string, err error) {
	params := records.GetDevelopmentCheckParams{
		DevelopmentName: developmentName,
		Name:            name,
	}
	_, err = queries.GetDevelopmentCheck(ctx, params)
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		logging.From(ctx).Error("get check", zap.Any("development", developmentName), zap.Any("check", name), zap.Error(err))
		return
	}
	err = nil
	if !absent {
		return
	}

	create := records.CreateDevelopmentCheckParams{
		DevelopmentName: developmentName,
		Name:            name,
	}
	err = queries.CreateDevelopmentCheck(ctx, create)
	if err != nil {
		logging.From(ctx).Error("insert check", zap.Any("development", developmentName), zap.Any("check", name), zap.Error(err))
		return
	}
	line = fmt.Sprintf("[%s] [%s] recorded", developmentName, name)
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
