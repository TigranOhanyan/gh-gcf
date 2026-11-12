package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
)

const envCursorKey = "CURSOR_API_KEY"
const envFactoryKey = "FACTORY_GITHUB_APP_PRIVATE_KEY"
const envRepoOwner = "FACTORY_REPO_OWNER"
const envRepoName = "FACTORY_REPO_NAME"
const appSecretPrefix = "FACTORY_GITHUB_APP_"

func main() {
	lambda.Start(handle)
}

func handle(ctx context.Context) (err error) {
	err = loadFixedSecrets(ctx)
	if err != nil {
		zap.L().Error("secrets", zap.Error(err))
		return
	}
	err = prepareGit()
	if err != nil {
		zap.L().Error("git", zap.Error(err))
		return
	}

	app, err := commands.LoadFactoryApp()
	if err != nil {
		return
	}
	ready := app.Configured()
	if !ready {
		err = fmt.Errorf("factory app is not configured")
		zap.L().Error("factory app", zap.Error(err))
		return
	}

	owner := os.Getenv(envRepoOwner)
	name := os.Getenv(envRepoName)
	missingRepo := owner == "" || name == ""
	if missingRepo {
		err = fmt.Errorf("factory repo is missing")
		zap.L().Error("factory repo", zap.Error(err))
		return
	}

	token, err := app.Token(ctx, owner, name)
	if err != nil {
		return
	}
	dir, err := cloneFactory(ctx, token, owner, name)
	if err != nil {
		zap.L().Error("clone factory", zap.String("owner", owner), zap.String("name", name), zap.Error(err))
		return
	}
	err = os.Chdir(dir)
	if err != nil {
		zap.L().Error("factory repo", zap.String("path", dir), zap.Error(err))
		return
	}
	err = loadAppSecrets(ctx, dir)
	if err != nil {
		zap.L().Error("app secrets", zap.Error(err))
		return
	}
	err = runTick(ctx, dir, token)
	return
}

func runTick(ctx context.Context, dir string, token string) (err error) {
	err = runSteps(ctx)
	if err != nil {
		return
	}
	err = pushState(ctx, dir, token)
	if err != nil {
		zap.L().Error("push state", zap.Error(err))
	}
	return
}

func runSteps(ctx context.Context) (err error) {
	stdout, err := runLine(ctx, "run tick")
	if err != nil {
		return
	}
	id := strings.TrimSpace(stdout)
	missing := id == ""
	if missing {
		err = fmt.Errorf("tick id is empty")
		zap.L().Error("tick", zap.Error(err))
		return
	}
	zap.L().Info("tick", zap.String("id", id))

	lines := stepLines(id)
	for _, line := range lines {
		_, err = runLine(ctx, line)
		if err != nil {
			return
		}
	}
	return
}

func stepLines(id string) (lines []string) {
	lines = []string{
		fmt.Sprintf("compile app --tick %s", id),
		fmt.Sprintf("compile label --tick %s", id),
		fmt.Sprintf("compile development --tick %s", id),
		fmt.Sprintf("run builder --tick %s", id),
		fmt.Sprintf("check builder --tick %s", id),
		fmt.Sprintf("run standards-reviewer --tick %s", id),
		fmt.Sprintf("check standards-reviewer --tick %s", id),
		fmt.Sprintf("run specification-reviewer --tick %s", id),
		fmt.Sprintf("check specification-reviewer --tick %s", id),
		fmt.Sprintf("run fixer --tick %s", id),
		fmt.Sprintf("check fixer --tick %s", id),
	}
	return
}

func runLine(ctx context.Context, line string) (stdout string, err error) {
	command := root.NewRoot()
	var out strings.Builder
	command.SetOut(&out)
	command.SetErr(os.Stderr)
	wrapped := commands.NewCmd(command)
	err = wrapped.Shell(ctx, line)
	stdout = out.String()
	if err != nil {
		zap.L().Error("step", zap.String("line", line), zap.Error(err))
	}
	return
}

func prepareGit() (err error) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	root := filepath.Join(filepath.Dir(exe), "gitroot")
	bin := filepath.Join(root, "bin")
	_, statErr := os.Stat(bin)
	missing := os.IsNotExist(statErr)
	if missing {
		return
	}
	if statErr != nil {
		err = statErr
		return
	}
	path := bin + string(os.PathListSeparator) + os.Getenv("PATH")
	err = os.Setenv("PATH", path)
	if err != nil {
		return
	}
	execPath := filepath.Join(root, "libexec", "git-core")
	err = os.Setenv("GIT_EXEC_PATH", execPath)
	if err != nil {
		return
	}
	lib := filepath.Join(root, "lib")
	err = os.Setenv("LD_LIBRARY_PATH", lib)
	return
}

func cloneFactory(ctx context.Context, token string, owner string, name string) (dir string, err error) {
	parent, err := os.MkdirTemp("", "gcf-")
	if err != nil {
		return
	}
	dir = filepath.Join(parent, "factory")
	url := fmt.Sprintf("https://github.com/%s/%s.git", owner, name)
	line := "clone --branch main --single-branch " + strconv.Quote(url) + " " + strconv.Quote(dir)
	_, err = gitWithToken(ctx, parent, token, line)
	if err != nil {
		return
	}
	_, err = gitWithToken(ctx, dir, token, "fetch origin state:refs/heads/state")
	if err != nil {
		return
	}
	_, err = commands.Git(ctx, dir, "config user.name factory")
	if err != nil {
		return
	}
	_, err = commands.Git(ctx, dir, "config user.email factory@users.noreply.github.com")
	return
}

func pushState(ctx context.Context, dir string, token string) (err error) {
	blank := token == ""
	if blank {
		_, err = commands.Git(ctx, dir, "push origin state")
		return
	}
	_, err = gitWithToken(ctx, dir, token, "push origin state")
	return
}

func gitWithToken(ctx context.Context, dir string, token string, line string) (output []byte, err error) {
	header := gitHeader(token)
	config := "http.extraheader=" + header
	full := "-c " + strconv.Quote(config) + " " + line
	output, err = commands.Git(ctx, dir, full)
	return
}

func gitHeader(token string) (header string) {
	pair := "x-access-token:" + token
	encoded := base64.StdEncoding.EncodeToString([]byte(pair))
	header = "AUTHORIZATION: basic " + encoded
	return
}
