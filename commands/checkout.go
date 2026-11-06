package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// RequireCheckout fails unless dir is the top of a git checkout.
func RequireCheckout(ctx context.Context, dir string) (err error) {
	top, err := Git(ctx, dir, "rev-parse --show-toplevel")
	if err != nil {
		err = fmt.Errorf("factory repo is not a git checkout")
		return
	}

	root := strings.TrimSpace(string(top))
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		return
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return
	}
	same := filepath.Clean(dirAbs) == filepath.Clean(rootAbs)
	if !same {
		err = fmt.Errorf("factory repo is not a git checkout")
	}
	return
}
