package setup

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func newSetupMember() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "member",
		Short: "Add or list members",
	}
	cmd.AddCommand(newAddBuilder())
	cmd.AddCommand(newAddReviewer())
	cmd.AddCommand(newAddFixer())
	cmd.AddCommand(newListMembers())
	return
}

func newAddBuilder() (cmd *cobra.Command) {
	var owner string
	var repo string
	var name string
	var starting string
	var attention string
	cmd = &cobra.Command{
		Use:   "add-builder",
		Short: "Store a builder and its instruction file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAddBuilder(cmd, owner, repo, name, starting, attention)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&name, "name", "", "member name")
	flags.StringVar(&starting, "starting", "", "starting label")
	flags.StringVar(&attention, "attention", "", "attention label")
	RequireFlag(cmd, "owner")
	RequireFlag(cmd, "repo")
	RequireFlag(cmd, "name")
	RequireFlag(cmd, "starting")
	RequireFlag(cmd, "attention")
	return
}

func newAddReviewer() (cmd *cobra.Command) {
	var owner string
	var repo string
	var builder string
	var name string
	var app string
	var attention string
	cmd = &cobra.Command{
		Use:   "add-reviewer",
		Short: "Store a reviewer and its instruction file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAddReviewer(cmd, owner, repo, builder, name, app, attention)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&builder, "builder", "", "builder name")
	flags.StringVar(&name, "name", "", "member name")
	flags.StringVar(&app, "app", "", "app id")
	flags.StringVar(&attention, "attention", "", "attention label")
	RequireFlag(cmd, "owner")
	RequireFlag(cmd, "repo")
	RequireFlag(cmd, "builder")
	RequireFlag(cmd, "name")
	RequireFlag(cmd, "app")
	RequireFlag(cmd, "attention")
	return
}

func newAddFixer() (cmd *cobra.Command) {
	var owner string
	var repo string
	var builder string
	var name string
	var attention string
	cmd = &cobra.Command{
		Use:   "add-fixer",
		Short: "Store a fixer and its instruction file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAddFixer(cmd, owner, repo, builder, name, attention)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&builder, "builder", "", "builder name")
	flags.StringVar(&name, "name", "", "member name")
	flags.StringVar(&attention, "attention", "", "attention label")
	RequireFlag(cmd, "owner")
	RequireFlag(cmd, "repo")
	RequireFlag(cmd, "builder")
	RequireFlag(cmd, "name")
	RequireFlag(cmd, "attention")
	return
}

func newListMembers() (cmd *cobra.Command) {
	var owner string
	var repo string
	cmd = &cobra.Command{
		Use:   "list",
		Short: "Print stored members",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runListMembers(cmd, owner, repo)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	RequireFlag(cmd, "owner")
	RequireFlag(cmd, "repo")
	return
}

func runAddBuilder(cmd *cobra.Command, rawOwner string, rawRepo string, rawName string, rawStarting string, rawAttention string) (err error) {
	ctx := cmd.Context()
	use := func(dir string, owner string, repo string, db *configdb.DB) (useErr error) {
		starting, useErr := NormalizeLabelName(rawStarting)
		if useErr != nil {
			return
		}
		attention, useErr := NormalizeLabelName(rawAttention)
		if useErr != nil {
			return
		}
		_, useErr = addBuilderMember(ctx, db, dir, owner, repo, rawName, starting, attention)
		return
	}
	err = WithRepo(ctx, rawOwner, rawRepo, use)
	return
}

func runAddReviewer(cmd *cobra.Command, rawOwner string, rawRepo string, rawBuilder string, rawName string, rawApp string, rawAttention string) (err error) {
	ctx := cmd.Context()
	use := func(dir string, owner string, repo string, db *configdb.DB) (useErr error) {
		builder, useErr := normalizeMemberName(rawBuilder)
		if useErr != nil {
			return
		}
		attention, useErr := NormalizeLabelName(rawAttention)
		if useErr != nil {
			return
		}
		appID, useErr := ParseAppID(rawApp)
		if useErr != nil {
			return
		}
		_, useErr = addReviewerMember(ctx, db, dir, owner, repo, builder, rawName, appID, attention)
		return
	}
	err = WithRepo(ctx, rawOwner, rawRepo, use)
	return
}

func runAddFixer(cmd *cobra.Command, rawOwner string, rawRepo string, rawBuilder string, rawName string, rawAttention string) (err error) {
	ctx := cmd.Context()
	use := func(dir string, owner string, repo string, db *configdb.DB) (useErr error) {
		builder, useErr := normalizeMemberName(rawBuilder)
		if useErr != nil {
			return
		}
		attention, useErr := NormalizeLabelName(rawAttention)
		if useErr != nil {
			return
		}
		_, useErr = addFixerMember(ctx, db, dir, owner, repo, builder, rawName, attention)
		return
	}
	err = WithRepo(ctx, rawOwner, rawRepo, use)
	return
}

func runListMembers(cmd *cobra.Command, owner string, repo string) (err error) {
	ctx := cmd.Context()
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		writer := cmd.OutOrStdout()
		useErr = printMembers(ctx, writer, db)
		return
	}
	err = WithRepo(ctx, owner, repo, use)
	return
}

func printMembers(ctx context.Context, writer io.Writer, db *configdb.DB) (err error) {
	builders, err := db.Queries.ListBuilderMembers(ctx)
	if err != nil {
		log.Error("list builders", "err", err)
		return
	}
	for _, builder := range builders {
		_, err = fmt.Fprintf(writer, "builder  %s  %s  %s\n", builder.Name, builder.StartingLabel, builder.AttentionLabel)
		if err != nil {
			return
		}
		err = printReviewers(ctx, writer, db, builder.Name)
		if err != nil {
			return
		}
		err = printFixers(ctx, writer, db, builder.Name)
		if err != nil {
			return
		}
	}
	return
}

func printReviewers(ctx context.Context, writer io.Writer, db *configdb.DB, builder string) (err error) {
	rows, err := db.Queries.ListReviewerMembers(ctx, builder)
	if err != nil {
		log.Error("list reviewers", "err", err)
		return
	}
	for _, row := range rows {
		app := strconv.FormatInt(row.GithubApp, 10)
		_, err = fmt.Fprintf(writer, "reviewer  %s  %s  %s  %s\n", row.Name, builder, app, row.AttentionLabel)
		if err != nil {
			return
		}
	}
	return
}

func printFixers(ctx context.Context, writer io.Writer, db *configdb.DB, builder string) (err error) {
	rows, err := db.Queries.ListFixerMembers(ctx, builder)
	if err != nil {
		log.Error("list fixers", "err", err)
		return
	}
	for _, row := range rows {
		_, err = fmt.Fprintf(writer, "fixer  %s  %s  %s\n", row.Name, builder, row.AttentionLabel)
		if err != nil {
			return
		}
	}
	return
}
