// Package pull installs a factory tarball into a host repository.
package pull

import (
	"fmt"
	"sort"
	"strings"

	"github.com/TigranOhanyan/gh-gcf/internal/source"
)

// Request is one installation of a resolved factory archive.
type Request struct {
	Dir     string
	Source  string
	Ref     string
	Commit  string
	Force   bool
	Archive []byte
}

// Summary counts the files a pull added, updated, removed, or left in place.
type Summary struct {
	Added    []string
	Updated  []string
	Removed  []string
	Skipped  []string
	Warnings []string
}

type changeKind int

const (
	changeAdd changeKind = iota
	changeUpdate
	changeRemove
	changeSkip
	changeKeep // already matches the incoming bytes; lock checksum moves forward
)

type change struct {
	kind    changeKind
	path    string
	body    []byte
	sum     string
	warning string
	record  bool
}

// Pull maps the factory archive into the host tree and writes .factory/gcf.lock.
// It does not commit. Files under .factory/state/ and .factory/*_override.tf are never touched.
func Pull(req Request) (Summary, error) {
	if _, _, err := source.ParseRepo(req.Source); err != nil {
		return Summary{}, err
	}
	if err := source.ValidateRef(req.Ref); err != nil {
		return Summary{}, err
	}
	req.Commit = strings.ToLower(req.Commit)
	if !commitRE.MatchString(req.Commit) {
		return Summary{}, fmt.Errorf("resolved commit must be a 40-character SHA, got %q", req.Commit)
	}

	incoming, err := mapArchive(req.Archive)
	if err != nil {
		return Summary{}, err
	}
	lock, err := loadLock(req.Dir)
	if err != nil {
		return Summary{}, err
	}
	changes, err := plan(req, incoming, lock)
	if err != nil {
		return Summary{}, err
	}
	if err := apply(req.Dir, changes); err != nil {
		return Summary{}, err
	}
	next := Lock{
		Version: lockVersion,
		Source:  req.Source,
		Ref:     req.Ref,
		Commit:  req.Commit,
		Files:   make([]LockFile, 0, len(changes)),
	}
	for _, ch := range changes {
		if !ch.record {
			continue
		}
		next.Files = append(next.Files, LockFile{Path: ch.path, SHA256: ch.sum})
	}
	if err := saveLock(req.Dir, next); err != nil {
		return Summary{}, err
	}
	return summarize(changes), nil
}

func plan(req Request, incoming map[string]factoryFile, lock *Lock) ([]change, error) {
	old := map[string]string{}
	if lock != nil {
		for _, file := range lock.Files {
			old[file.Path] = file.SHA256
		}
	}

	var changes []change
	var problems []string
	var conflicts []string

	paths := make([]string, 0, len(incoming))
	for path := range incoming {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		file := incoming[path]
		kind, sum, err := inspect(req.Dir, path)
		if err != nil {
			return nil, err
		}
		prev, owned := old[path]
		desired := sha256Hex(file.body)
		ch := change{path: path, body: file.body, sum: desired}

		switch kind {
		case diskOther:
			problems = append(problems, fmt.Sprintf("%s is not a regular file (refusing to follow a symlink or replace a directory)", path))
			continue
		case diskMissing:
			if !owned {
				ch.kind = changeAdd
				ch.record = true
			} else if req.Force {
				ch.kind = changeUpdate
				ch.record = true
			} else {
				ch.kind = changeSkip
				ch.sum = prev
				ch.body = nil
				ch.record = true
				ch.warning = fmt.Sprintf("%s was deleted locally and was not restored (re-run with --force to restore)", path)
			}
		case diskFile:
			if !owned {
				if !req.Force {
					conflicts = append(conflicts, path)
					continue
				}
				ch.kind = changeAdd
				ch.record = true
				if sum == desired {
					ch.body = nil
				}
			} else if sum == desired {
				// The working tree already matches the factory file, including a local edit
				// that happens to match. Keep the bytes and record the new checksum.
				ch.kind = changeKeep
				ch.record = true
				ch.body = nil
			} else if sum == prev {
				ch.kind = changeUpdate
				ch.record = true
			} else if req.Force {
				ch.kind = changeUpdate
				ch.record = true
			} else {
				// Keep the previous installed checksum so the next pull still sees the edit.
				ch.kind = changeSkip
				ch.sum = prev
				ch.record = true
				ch.body = nil
				ch.warning = fmt.Sprintf("%s has local changes and was left unchanged (re-run with --force to overwrite)", path)
			}
		}
		changes = append(changes, ch)
	}

	var stale []string
	for path := range old {
		if _, ok := incoming[path]; !ok {
			stale = append(stale, path)
		}
	}
	sort.Strings(stale)
	for _, path := range stale {
		kind, sum, err := inspect(req.Dir, path)
		if err != nil {
			return nil, err
		}
		ch := change{path: path}
		switch kind {
		case diskOther:
			problems = append(problems, fmt.Sprintf("%s is not a regular file (refusing to follow a symlink or replace a directory)", path))
			continue
		case diskMissing:
			ch.kind = changeRemove
		case diskFile:
			if sum == old[path] || req.Force {
				ch.kind = changeRemove
			} else {
				ch.kind = changeSkip
				ch.warning = fmt.Sprintf("%s has local changes and was left in place; the factory no longer ships it, so it is no longer managed", path)
			}
		}
		changes = append(changes, ch)
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("refusing to continue:\n%s", bullets(problems))
	}
	if len(conflicts) > 0 {
		return nil, fmt.Errorf("these files already exist and are not managed by gh gcf:\n%s\nre-run with --force to overwrite them", bullets(conflicts))
	}
	return changes, nil
}

func apply(dir string, changes []change) error {
	for _, ch := range changes {
		if (ch.kind == changeAdd || ch.kind == changeUpdate) && ch.body != nil {
			if err := writeFileAtomic(dir, ch.path, ch.body); err != nil {
				return err
			}
		}
	}
	for _, ch := range changes {
		if ch.kind == changeRemove {
			if err := deleteFile(dir, ch.path); err != nil {
				return err
			}
		}
	}
	return nil
}

func summarize(changes []change) Summary {
	sum := Summary{
		Added:    []string{},
		Updated:  []string{},
		Removed:  []string{},
		Skipped:  []string{},
		Warnings: []string{},
	}
	for _, ch := range changes {
		switch ch.kind {
		case changeAdd:
			sum.Added = append(sum.Added, ch.path)
		case changeUpdate:
			sum.Updated = append(sum.Updated, ch.path)
		case changeRemove:
			sum.Removed = append(sum.Removed, ch.path)
		case changeSkip:
			sum.Skipped = append(sum.Skipped, ch.path)
			if ch.warning != "" {
				sum.Warnings = append(sum.Warnings, ch.warning)
			}
		}
	}
	sort.Strings(sum.Added)
	sort.Strings(sum.Updated)
	sort.Strings(sum.Removed)
	sort.Strings(sum.Skipped)
	sort.Strings(sum.Warnings)
	return sum
}
