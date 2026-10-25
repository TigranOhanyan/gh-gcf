package pull

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type diskKind int

const (
	diskMissing diskKind = iota
	diskFile
	diskOther
)

func inspect(dir, slashPath string) (diskKind, string, error) {
	if err := validateRelPath(slashPath); err != nil {
		return 0, "", err
	}
	abs := filepath.Join(dir, filepath.FromSlash(slashPath))
	if err := withinRoot(dir, abs); err != nil {
		return 0, "", err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return diskMissing, "", nil
		}
		return 0, "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return diskOther, "", nil
	}
	f, err := os.Open(abs)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return 0, "", err
	}
	return diskFile, hex.EncodeToString(h.Sum(nil)), nil
}

func writeFileAtomic(dir, slashPath string, content []byte) error {
	if err := validateRelPath(slashPath); err != nil {
		return err
	}
	rel := filepath.FromSlash(slashPath)
	parent := filepath.Dir(rel)
	if parent != "." {
		if err := mkdirAllNoFollow(dir, parent); err != nil {
			return err
		}
	}
	abs := filepath.Join(dir, rel)
	if err := withinRoot(dir, abs); err != nil {
		return err
	}
	info, err := os.Lstat(abs)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to follow symlink %s", slashPath)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", slashPath)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(abs), ".gcf-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, abs); err != nil {
		return err
	}
	success = true
	return nil
}

func deleteFile(dir, slashPath string) error {
	if err := validateRelPath(slashPath); err != nil {
		return err
	}
	abs := filepath.Join(dir, filepath.FromSlash(slashPath))
	if err := withinRoot(dir, abs); err != nil {
		return err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to delete symlink %s", slashPath)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to delete %s", slashPath)
	}
	return os.Remove(abs)
}

func mkdirAllNoFollow(root, rel string) error {
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" {
		return nil
	}
	cur := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return fmt.Errorf("invalid directory %s", rel)
		}
		cur = filepath.Join(cur, part)
		if err := withinRoot(root, cur); err != nil {
			return err
		}
		info, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				if err := os.Mkdir(cur, 0o755); err != nil {
					return err
				}
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to follow symlink %s", cur)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", cur)
		}
	}
	return nil
}

func withinRoot(root, abs string) error {
	root = filepath.Clean(root)
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes the repository")
	}
	return nil
}
