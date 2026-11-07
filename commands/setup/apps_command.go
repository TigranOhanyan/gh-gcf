package setup

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func newSetupApps() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "apps",
		Short: "Add or list Apps",
	}
	cmd.AddCommand(newAddApp())
	cmd.AddCommand(newListApps())
	cmd.AddCommand(newAttachApp())
	return
}

func newAddApp() (cmd *cobra.Command) {
	var id string
	cmd = &cobra.Command{
		Use:   "add",
		Short: "Register an App",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAddApp(cmd, id)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&id, "id", "", "app id")
	RequireFlag(cmd, "id")
	return
}

func newListApps() (cmd *cobra.Command) {
	var owner string
	var repo string
	cmd = &cobra.Command{
		Use:   "list",
		Short: "Print Apps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runListApps(cmd, owner, repo)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	return
}

func newAttachApp() (cmd *cobra.Command) {
	var owner string
	var repo string
	var id string
	cmd = &cobra.Command{
		Use:   "attach",
		Short: "Attach an App to an app repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAttachApp(cmd, owner, repo, id)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&id, "id", "", "app id")
	RequireFlag(cmd, "owner")
	RequireFlag(cmd, "repo")
	RequireFlag(cmd, "id")
	return
}

func runAddApp(cmd *cobra.Command, rawID string) (err error) {
	ctx := cmd.Context()
	dir, err := CurrentFactory(ctx)
	if err != nil {
		return
	}
	_, err = createAppFile(dir, rawID)
	return
}

func runListApps(cmd *cobra.Command, owner string, repo string) (err error) {
	ctx := cmd.Context()
	dir, err := CurrentFactory(ctx)
	if err != nil {
		return
	}
	ownerText := strings.TrimSpace(owner)
	repoText := strings.TrimSpace(repo)
	ownerSet := ownerText != ""
	repoSet := repoText != ""
	neither := !ownerSet && !repoSet
	if neither {
		err = printRegisteredApps(cmd, dir)
		return
	}
	both := ownerSet && repoSet
	if !both {
		err = fmt.Errorf("owner and repo must both be set")
		return
	}
	err = printAttachedApps(cmd, ownerText, repoText)
	return
}

func runAttachApp(cmd *cobra.Command, owner string, repo string, rawID string) (err error) {
	ctx := cmd.Context()
	use := func(dir string, _ string, _ string, db *configdb.DB) (useErr error) {
		id, useErr := ParseAppID(rawID)
		if useErr != nil {
			return
		}
		useErr = requireRegistered(dir, id)
		if useErr != nil {
			return
		}
		useErr = attachApp(ctx, db, id)
		return
	}
	err = WithRepo(ctx, owner, repo, use)
	return
}

func requireRegistered(factoryDir string, id int64) (err error) {
	ids, err := commands.ListAppIDs(factoryDir)
	if err != nil {
		return
	}
	for _, existing := range ids {
		same := existing == id
		if same {
			return
		}
	}
	text := strconv.FormatInt(id, 10)
	err = fmt.Errorf("app %s is not registered", text)
	log.Info("app is not registered", "app", id)
	return
}

func printRegisteredApps(cmd *cobra.Command, dir string) (err error) {
	ids, err := commands.ListAppIDs(dir)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printAppIDs(writer, ids)
	return
}

func printAttachedApps(cmd *cobra.Command, owner string, repo string) (err error) {
	ctx := cmd.Context()
	var ids []int64
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		ids, useErr = db.Queries.ListGithubApps(ctx)
		if useErr != nil {
			log.Error("list apps", "err", useErr)
		}
		return
	}
	err = WithRepo(ctx, owner, repo, use)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printAppIDs(writer, ids)
	return
}

func printAppIDs(writer io.Writer, ids []int64) (err error) {
	for _, id := range ids {
		text := strconv.FormatInt(id, 10)
		_, err = fmt.Fprintln(writer, text)
		if err != nil {
			return
		}
	}
	return
}
