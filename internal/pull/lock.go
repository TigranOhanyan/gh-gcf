package pull

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/TigranOhanyan/gh-gcf/internal/source"
)

const (
	lockVersion = 1
	lockRelPath = ".factory/gcf.lock"
)

var (
	commitRE   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	checksumRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Lock is the record of one pull, written to .factory/gcf.lock.
type Lock struct {
	Version int        `json:"version"`
	Source  string     `json:"source"`
	Ref     string     `json:"ref"`
	Commit  string     `json:"commit"`
	Files   []LockFile `json:"files"`
}

// LockFile is one file the CLI installed, with the checksum of the bytes it wrote.
// The checksum stays at the last installed content when a later pull keeps a local edit.
type LockFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func loadLock(dir string) (*Lock, error) {
	abs := filepath.Join(dir, filepath.FromSlash(lockRelPath))
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink %s", lockRelPath)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a file", lockRelPath)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	var lock Lock
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&lock); err != nil {
		return nil, fmt.Errorf("read %s: %w", lockRelPath, err)
	}
	if err := validateLock(&lock); err != nil {
		return nil, fmt.Errorf("%s: %w", lockRelPath, err)
	}
	return &lock, nil
}

func validateLock(lock *Lock) error {
	if lock.Version != lockVersion {
		return fmt.Errorf("unsupported version %d (this gh gcf understands version %d)", lock.Version, lockVersion)
	}
	if _, _, err := source.ParseRepo(lock.Source); err != nil {
		return err
	}
	if err := source.ValidateRef(lock.Ref); err != nil {
		return err
	}
	lock.Commit = strings.ToLower(lock.Commit)
	if !commitRE.MatchString(lock.Commit) {
		return fmt.Errorf("commit must be a 40-character SHA, got %q", lock.Commit)
	}
	seen := make(map[string]struct{}, len(lock.Files))
	for i := range lock.Files {
		file := &lock.Files[i]
		if err := validateRelPath(file.Path); err != nil {
			return err
		}
		if reservedPath(file.Path) {
			return fmt.Errorf("lists reserved path %s", file.Path)
		}
		file.SHA256 = strings.ToLower(file.SHA256)
		if !checksumRE.MatchString(file.SHA256) {
			return fmt.Errorf("checksum for %s must be a sha256 hex digest", file.Path)
		}
		if _, ok := seen[file.Path]; ok {
			return fmt.Errorf("lists %s more than once", file.Path)
		}
		seen[file.Path] = struct{}{}
	}
	return nil
}

func saveLock(dir string, lock Lock) error {
	if lock.Files == nil {
		lock.Files = []LockFile{}
	}
	sort.Slice(lock.Files, func(i, j int) bool {
		return lock.Files[i].Path < lock.Files[j].Path
	})
	if err := validateLock(&lock); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(lock); err != nil {
		return err
	}
	return writeFileAtomic(dir, lockRelPath, buf.Bytes())
}

func validateRelPath(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || strings.Contains(p, "\x00") {
		return fmt.Errorf("invalid path %q", p)
	}
	if pathHasDotDot(p) || path.Clean(p) != p {
		return fmt.Errorf("invalid path %q", p)
	}
	return nil
}

func pathHasDotDot(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." || seg == "." || seg == "" {
			return true
		}
	}
	return false
}
