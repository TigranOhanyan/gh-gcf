package bind

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/github"
)

const (
	deployRel          = ".github/workflows/factory-deploy.yml"
	deleteRel          = ".github/workflows/factory-delete.yml"
	secretsWorkflowRel = ".github/workflows/factory-secrets.yml"
	templateRel        = "infrastructure.yaml"
	secretsRel         = "secrets.yaml"
	legacyTickRel      = ".github/workflows/factory-tick.yml"
	gitignoreRel       = ".worktrees/.gitignore"
	hintRel            = ".worktrees/internal_use_only"
	commitMessage      = "bind factory"
	versionToken       = "__GCF_VERSION__"
	devVersion         = "dev"
	appSecretsToken    = "__APP_SECRETS__"
)

// Version is the release tag of this binary. The release build sets it with -X.
// A local build leaves it as dev, and bind refuses to run.
var Version = devVersion

func NewBind() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "bind",
		Short: "Write the factory install and migrate config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			dir, err := os.Getwd()
			if err != nil {
				log.Error("current directory", "err", err)
				return
			}
			err = Bind(cmd.Context(), dir)
			return
		},
	}
	return
}

func Bind(ctx context.Context, factoryDir string) (err error) {
	err = requireRelease()
	if err != nil {
		log.Error("gh-gcf version", "version", Version, "err", err)
		return
	}

	err = commands.RequireCheckout(ctx, factoryDir)
	if err != nil {
		log.Error("factory repo", "path", factoryDir, "err", err)
		return
	}
	log.Info("factory repo", "path", factoryDir)

	err = installDeploy(factoryDir)
	if err != nil {
		log.Error("install deploy", "path", factoryDir, "err", err)
		return
	}

	err = installDelete(factoryDir)
	if err != nil {
		log.Error("install delete", "path", factoryDir, "err", err)
		return
	}

	err = installTemplate(factoryDir)
	if err != nil {
		log.Error("install template", "path", factoryDir, "err", err)
		return
	}

	err = installSecrets(factoryDir)
	if err != nil {
		log.Error("install secrets", "path", factoryDir, "err", err)
		return
	}

	err = installSecretsWorkflow(factoryDir)
	if err != nil {
		log.Error("install secrets workflow", "path", factoryDir, "err", err)
		return
	}

	err = installWorktrees(factoryDir)
	if err != nil {
		log.Error("install worktrees", "path", factoryDir, "err", err)
		return
	}

	tracked, err := removeLegacyTick(ctx, factoryDir)
	if err != nil {
		log.Error("remove tick workflow", "path", factoryDir, "err", err)
		return
	}

	configRels, err := migrateConfigs(ctx, factoryDir)
	if err != nil {
		log.Error("migrate config", "path", factoryDir, "err", err)
		return
	}

	files := []string{deployRel, deleteRel, secretsWorkflowRel, templateRel, secretsRel, gitignoreRel, hintRel}
	if tracked {
		files = append(files, legacyTickRel)
	}
	files = append(files, configRels...)
	err = commitCheckout(ctx, factoryDir, files)
	return
}

func installDeploy(factoryDir string) (err error) {
	contents, err := deployContents()
	if err != nil {
		return
	}
	err = installFile(factoryDir, deployRel, contents)
	return
}

func installDelete(factoryDir string) (err error) {
	contents, err := workflowfile.ReadFactoryDelete()
	if err != nil {
		return
	}
	err = installFile(factoryDir, deleteRel, contents)
	return
}

func installTemplate(factoryDir string) (err error) {
	contents, err := templateContents()
	if err != nil {
		return
	}
	err = installFile(factoryDir, templateRel, contents)
	return
}

func installFile(factoryDir string, rel string, contents []byte) (err error) {
	path := filepath.Join(factoryDir, filepath.FromSlash(rel))
	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return
	}
	err = os.WriteFile(path, contents, 0644)
	return
}

func removeLegacyTick(ctx context.Context, factoryDir string) (tracked bool, err error) {
	line := "cat-file -e HEAD:" + legacyTickRel
	_, statErr := commands.Git(ctx, factoryDir, line)
	tracked = statErr == nil

	path := filepath.Join(factoryDir, filepath.FromSlash(legacyTickRel))
	removeErr := os.Remove(path)
	missing := os.IsNotExist(removeErr)
	if missing {
		removeErr = nil
	}
	err = removeErr
	return
}

func requireRelease() (err error) {
	missing := Version == "" || Version == devVersion
	if missing {
		err = fmt.Errorf("gh-gcf version is not a release")
	}
	return
}

func installSecrets(factoryDir string) (err error) {
	contents, err := secretsContents(factoryDir)
	if err != nil {
		return
	}
	err = installFile(factoryDir, secretsRel, contents)
	return
}

func installSecretsWorkflow(factoryDir string) (err error) {
	contents, err := workflowfile.ReadFactorySecrets()
	if err != nil {
		return
	}
	err = installFile(factoryDir, secretsWorkflowRel, contents)
	return
}

func deployContents() (contents []byte, err error) {
	template, err := workflowfile.ReadFactoryDeploy()
	if err != nil {
		return
	}
	token := []byte(versionToken)
	hasToken := bytes.Contains(template, token)
	if !hasToken {
		err = fmt.Errorf("deploy workflow has no version token")
		return
	}
	version := []byte(Version)
	contents = bytes.ReplaceAll(template, token, version)
	return
}

func templateContents() (contents []byte, err error) {
	contents, err = workflowfile.ReadInfrastructure()
	return
}

func secretsContents(factoryDir string) (contents []byte, err error) {
	template, err := workflowfile.ReadSecrets()
	if err != nil {
		return
	}
	ids, err := commands.ListAppIDs(factoryDir)
	if err != nil {
		return
	}
	contents, err = replaceMarker(template, appSecretsToken, appSecretResources(ids))
	return
}

func replaceMarker(contents []byte, marker string, replacement [][]byte) (next []byte, err error) {
	lines := bytes.Split(contents, []byte("\n"))
	index := -1
	var indent string
	token := []byte(marker)
	for i, line := range lines {
		body, lead := markerBody(line)
		match := bytes.Equal(body, token)
		if !match {
			continue
		}
		already := index >= 0
		if already {
			err = fmt.Errorf("template has more than one %s marker", marker)
			return
		}
		index = i
		indent = lead
	}
	missing := index < 0
	if missing {
		err = fmt.Errorf("template has no %s marker", marker)
		return
	}

	indented := indentLines(indent, replacement)
	var out [][]byte
	head := lines[:index]
	tail := lines[index+1:]
	out = append(out, head...)
	out = append(out, indented...)
	out = append(out, tail...)
	next = bytes.Join(out, []byte("\n"))
	return
}

func indentLines(indent string, lines [][]byte) (indented [][]byte) {
	indented = make([][]byte, 0, len(lines))
	for _, line := range lines {
		joined := make([]byte, 0, len(indent)+len(line))
		joined = append(joined, indent...)
		joined = append(joined, line...)
		indented = append(indented, joined)
	}
	return
}

func markerBody(line []byte) (body []byte, indent string) {
	trimmed := bytes.TrimLeft(line, " \t")
	lead := len(line) - len(trimmed)
	indent = string(line[:lead])
	body = bytes.TrimRight(trimmed, " \t\r")
	return
}

func appSecretResources(ids []int64) (lines [][]byte) {
	lines = make([][]byte, 0, len(ids)*8)
	for _, id := range ids {
		text := strconv.FormatInt(id, 10)
		lines = append(lines, []byte("AppKey"+text+":"))
		lines = append(lines, []byte("  Type: AWS::SecretsManager::Secret"))
		lines = append(lines, []byte("  DeletionPolicy: Delete"))
		lines = append(lines, []byte("  UpdateReplacePolicy: Delete"))
		lines = append(lines, []byte("  Properties:"))
		name := "    Name: !Sub \"${AWS::StackName}/app/" + text + "\""
		lines = append(lines, []byte(name))
		description := "    Description: GitHub App " + text + " private key"
		lines = append(lines, []byte(description))
	}
	return
}

var worktreesGitignore = []byte("*\n!.gitignore\n!internal_use_only\n")

func installWorktrees(factoryDir string) (err error) {
	worktreesDir := filepath.Join(factoryDir, ".worktrees")
	err = os.MkdirAll(worktreesDir, 0755)
	if err != nil {
		return
	}

	gitignorePath := filepath.Join(factoryDir, filepath.FromSlash(gitignoreRel))
	err = os.WriteFile(gitignorePath, worktreesGitignore, 0644)
	if err != nil {
		return
	}

	hintPath := filepath.Join(factoryDir, filepath.FromSlash(hintRel))
	empty := []byte{}
	err = os.WriteFile(hintPath, empty, 0644)
	return
}

func migrateConfigs(ctx context.Context, factoryDir string) (rels []string, err error) {
	apps, err := listApps(factoryDir)
	if err != nil {
		return
	}

	for _, app := range apps {
		configRel := filepath.ToSlash(filepath.Join(app, "config.db"))
		configPath := filepath.Join(factoryDir, filepath.FromSlash(configRel))
		_, statErr := os.Stat(configPath)
		missing := os.IsNotExist(statErr)
		if missing {
			continue
		}
		if statErr != nil {
			err = statErr
			return
		}

		db, openErr := configdb.OpenAndMigrate(ctx, configPath)
		if openErr != nil {
			err = openErr
			return
		}
		err = db.Close()
		if err != nil {
			return
		}
		rels = append(rels, configRel)
		log.Info("migrated config", "path", configRel)
	}
	return
}

func commitCheckout(ctx context.Context, factoryDir string, files []string) (err error) {
	for _, rel := range files {
		addLine := "add -- " + strconv.Quote(rel)
		_, err = commands.Git(ctx, factoryDir, addLine)
		if err != nil {
			log.Error("stage bind file", "path", rel, "err", err)
			return
		}
	}

	line := "commit -m " + strconv.Quote(commitMessage) + " --"
	for _, rel := range files {
		line = line + " " + strconv.Quote(rel)
	}
	output, err := commands.Git(ctx, factoryDir, line)
	if err == nil {
		log.Info("committed bind", "dir", factoryDir)
		return
	}
	marker := []byte("nothing to commit")
	unchanged := bytes.Contains(output, marker)
	if unchanged {
		log.Info("bind files are unchanged", "dir", factoryDir)
		err = nil
		return
	}
	log.Error("commit bind", "dir", factoryDir, "err", err)
	return
}

func listApps(factoryDir string) (rels []string, err error) {
	reposDir := filepath.Join(factoryDir, "repos")
	owners, err := os.ReadDir(reposDir)
	missing := os.IsNotExist(err)
	if missing {
		err = nil
		return
	}
	if err != nil {
		return
	}

	for _, owner := range owners {
		ownerName := owner.Name()
		ownerIsDir := owner.IsDir()
		if !ownerIsDir {
			err = fmt.Errorf("path is not repos/<owner>/<name>: repos/%s", ownerName)
			return
		}
		ownerDir := filepath.Join(reposDir, ownerName)
		names, readErr := os.ReadDir(ownerDir)
		if readErr != nil {
			err = readErr
			return
		}
		for _, name := range names {
			nameText := name.Name()
			rel := filepath.ToSlash(filepath.Join("repos", ownerName, nameText))
			nameIsDir := name.IsDir()
			if !nameIsDir {
				err = fmt.Errorf("path is not repos/<owner>/<name>: %s", rel)
				return
			}
			rels = append(rels, rel)
		}
	}

	sort.Strings(rels)
	return
}
