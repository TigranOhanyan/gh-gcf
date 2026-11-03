package setup

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

func requireCheckout(ctx context.Context, dir string) (err error) {
	top, err := commands.Git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		log.Error("git checkout", "path", dir, "err", err)
		err = fmt.Errorf("directory is not a git checkout")
		return
	}

	root := strings.TrimSpace(string(top))
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		log.Error("factory directory", "path", dir, "err", err)
		return
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		log.Error("git toplevel", "path", root, "err", err)
		return
	}

	same := filepath.Clean(dirAbs) == filepath.Clean(rootAbs)
	if !same {
		err = fmt.Errorf("directory is not a git checkout")
		log.Error("git checkout", "path", dir, "err", err)
	}
	return
}
