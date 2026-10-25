package pull

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	testRepo = "TigranOhanyan/github-cursor-factory"
	commitA  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitB  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	commitC  = "cccccccccccccccccccccccccccccccccccccccc"
)

func TestPullInstallsMappingAndLock(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, "README.md", "host readme")
	seed(t, dir, ".gitignore", "host ignore")
	seed(t, dir, ".github/workflows/validate.yml", "host validate")
	seed(t, dir, ".github/workflows/ci.yml", "project ci")
	seed(t, dir, ".factory/custom.txt", "custom")
	seed(t, dir, ".factory/state/terraform.tfstate", "SECRET")
	seed(t, dir, ".factory/labels_override.tf", "OVERRIDE")

	files := map[string]string{
		"labels.tf":                         "labels-v1",
		"variables.tf":                      "vars-v1",
		"old.tf":                            "old-v1",
		"empty.tf":                          "",
		"statement.tf":                      "statement",
		"docs/README.md":                    "nested readme",
		"subdir/.gitignore":                 "nested ignore",
		"github/helpers/note.txt":           "note",
		"github/workflows/factory.yml":      "workflow-v1",
		"github/workflows/nested/extra.yml": "extra-v1",
		"modules/labels_override.tf":        "nested override",
		"README.md":                         "factory readme",
		".gitignore":                        "factory ignore",
		".github/workflows/validate.yml":    "factory ci",
		"LICENSE":                           "license",
	}
	sum := mustPull(t, dir, files, "v1.0.0", commitA, testRepo, false)
	wantAdded := []string{
		".factory/LICENSE",
		".factory/docs/README.md",
		".factory/empty.tf",
		".factory/github/helpers/note.txt",
		".factory/labels.tf",
		".factory/modules/labels_override.tf",
		".factory/old.tf",
		".factory/statement.tf",
		".factory/subdir/.gitignore",
		".factory/variables.tf",
		".github/workflows/factory.yml",
		".github/workflows/nested/extra.yml",
	}
	assertSummary(t, sum, wantAdded, nil, nil, nil)

	assertFile(t, dir, ".factory/labels.tf", "labels-v1")
	assertFile(t, dir, ".factory/empty.tf", "")
	assertFile(t, dir, ".factory/statement.tf", "statement")
	assertFile(t, dir, ".factory/modules/labels_override.tf", "nested override")
	assertFile(t, dir, ".factory/github/helpers/note.txt", "note")
	assertFile(t, dir, ".github/workflows/factory.yml", "workflow-v1")
	assertFile(t, dir, ".github/workflows/nested/extra.yml", "extra-v1")
	assertFile(t, dir, "README.md", "host readme")
	assertFile(t, dir, ".gitignore", "host ignore")
	assertFile(t, dir, ".github/workflows/validate.yml", "host validate")
	assertFile(t, dir, ".github/workflows/ci.yml", "project ci")
	assertFile(t, dir, ".factory/custom.txt", "custom")
	assertFile(t, dir, ".factory/state/terraform.tfstate", "SECRET")
	assertFile(t, dir, ".factory/labels_override.tf", "OVERRIDE")
	assertAbsent(t, dir, ".factory/README.md")
	assertAbsent(t, dir, ".factory/.gitignore")

	info, err := os.Stat(filepath.Join(dir, ".factory", "labels.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}

	lock := readLock(t, dir)
	if lock.Version != 1 || lock.Source != testRepo || lock.Ref != "v1.0.0" || lock.Commit != commitA {
		t.Fatalf("lock header: %+v", lock)
	}
	if len(lock.Files) != len(wantAdded) {
		t.Fatalf("lock files: %+v", lock.Files)
	}
	for i, path := range wantAdded {
		if lock.Files[i].Path != path {
			t.Fatalf("lock order: got %s want %s", lock.Files[i].Path, path)
		}
		body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if lock.Files[i].SHA256 != digest(body) {
			t.Fatalf("%s checksum %s", path, lock.Files[i].SHA256)
		}
	}
	if lock.Files[fileIndex(lock, ".factory/empty.tf")].SHA256 != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatal("empty file checksum")
	}
	raw := readRawLock(t, dir)
	if !strings.HasPrefix(raw, "{\n  \"version\": 1,\n  \"source\": ") || !strings.HasSuffix(raw, "}\n") {
		t.Fatalf("lock json:\n%s", raw)
	}
	if strings.Contains(raw, "gcf.lock") {
		t.Fatalf("lock lists itself:\n%s", raw)
	}

	again := mustPull(t, dir, files, "v1.0.0", commitA, testRepo, false)
	assertSummary(t, again, nil, nil, nil, nil)
	if readRawLock(t, dir) != raw {
		t.Fatal("second pull rewrote an unchanged lock")
	}
}

func TestPullUpdateEditAndRemoval(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, ".factory/state/terraform.tfstate", "SECRET")
	seed(t, dir, ".factory/labels_override.tf", "OVERRIDE")
	seed(t, dir, ".factory/custom.txt", "custom")
	seed(t, dir, ".github/workflows/ci.yml", "project ci")

	v1 := map[string]string{
		"labels.tf":                         "labels-v1",
		"variables.tf":                      "vars-v1",
		"old.tf":                            "old-v1",
		"empty.tf":                          "",
		"statement.tf":                      "statement",
		"docs/README.md":                    "nested readme",
		"subdir/.gitignore":                 "nested ignore",
		"github/helpers/note.txt":           "note",
		"github/workflows/factory.yml":      "workflow-v1",
		"github/workflows/nested/extra.yml": "extra-v1",
		"modules/labels_override.tf":        "nested override",
		"LICENSE":                           "license",
	}
	mustPull(t, dir, v1, "v1", commitA, testRepo, false)
	varsSum := readLock(t, dir).Files[fileIndex(readLock(t, dir), ".factory/variables.tf")].SHA256

	seed(t, dir, ".factory/variables.tf", "vars-local")
	seed(t, dir, ".factory/old.tf", "old-local")

	v2 := map[string]string{
		"labels.tf":                    "labels-v2",
		"variables.tf":                 "vars-v2",
		"empty.tf":                     "",
		"docs/README.md":               "nested readme",
		"github/workflows/factory.yml": "workflow-v1",
		"modules/labels_override.tf":   "nested override",
		"new.tf":                       "new",
	}
	sum := mustPull(t, dir, v2, "v2", commitB, "fork/factory", false)
	assertSummary(t, sum,
		[]string{".factory/new.tf"},
		[]string{".factory/labels.tf"},
		[]string{
			".factory/LICENSE",
			".factory/github/helpers/note.txt",
			".factory/statement.tf",
			".factory/subdir/.gitignore",
			".github/workflows/nested/extra.yml",
		},
		[]string{".factory/old.tf", ".factory/variables.tf"},
	)
	if len(sum.Warnings) != 2 {
		t.Fatalf("warnings: %v", sum.Warnings)
	}
	assertFile(t, dir, ".factory/labels.tf", "labels-v2")
	assertFile(t, dir, ".factory/variables.tf", "vars-local")
	assertFile(t, dir, ".factory/old.tf", "old-local")
	assertFile(t, dir, ".factory/new.tf", "new")
	assertAbsent(t, dir, ".factory/statement.tf")
	assertAbsent(t, dir, ".factory/LICENSE")
	assertAbsent(t, dir, ".github/workflows/nested/extra.yml")
	assertFile(t, dir, ".factory/state/terraform.tfstate", "SECRET")
	assertFile(t, dir, ".factory/labels_override.tf", "OVERRIDE")
	assertFile(t, dir, ".factory/custom.txt", "custom")
	assertFile(t, dir, ".github/workflows/ci.yml", "project ci")

	lock := readLock(t, dir)
	if lock.Source != "fork/factory" || lock.Ref != "v2" || lock.Commit != commitB {
		t.Fatalf("lock header: %+v", lock)
	}
	if got := lock.Files[fileIndex(lock, ".factory/variables.tf")].SHA256; got != varsSum {
		t.Fatalf("edited file checksum changed: %s", got)
	}
	if _, ok := lockPathSet(lock)[".factory/old.tf"]; ok {
		t.Fatal("edited removed file stayed in the lock")
	}
	if lock.Files[fileIndex(lock, ".factory/labels.tf")].SHA256 != digest([]byte("labels-v2")) {
		t.Fatal("updated checksum")
	}

	third := mustPull(t, dir, v2, "v2", commitB, "fork/factory", false)
	assertSummary(t, third, nil, nil, nil, []string{".factory/variables.tf"})
	assertFile(t, dir, ".factory/old.tf", "old-local")
	joined := strings.Join(third.Warnings, "\n")
	if strings.Contains(joined, "old.tf") {
		t.Fatalf("unmanaged leftover warned again: %v", third.Warnings)
	}

	forced := mustPull(t, dir, v2, "v2", commitB, "fork/factory", true)
	assertSummary(t, forced, nil, []string{".factory/variables.tf"}, nil, nil)
	assertFile(t, dir, ".factory/variables.tf", "vars-v2")
	assertFile(t, dir, ".factory/old.tf", "old-local")
	if readLock(t, dir).Files[fileIndex(readLock(t, dir), ".factory/variables.tf")].SHA256 != digest([]byte("vars-v2")) {
		t.Fatal("force did not record the new checksum")
	}
}

func TestPullForceDeletesEditedRemoval(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, dir, map[string]string{"old.tf": "old-v1", "labels.tf": "labels-v1"}, "v1", commitA, testRepo, false)
	seed(t, dir, ".factory/old.tf", "old-local")
	sum := mustPull(t, dir, map[string]string{"labels.tf": "labels-v1"}, "v2", commitB, testRepo, true)
	assertSummary(t, sum, nil, nil, []string{".factory/old.tf"}, nil)
	assertAbsent(t, dir, ".factory/old.tf")
}

func TestPullLocalDeletion(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, dir, map[string]string{"labels.tf": "labels-v1"}, "v1", commitA, testRepo, false)
	if err := os.Remove(filepath.Join(dir, ".factory", "labels.tf")); err != nil {
		t.Fatal(err)
	}
	sum := mustPull(t, dir, map[string]string{"labels.tf": "labels-v2"}, "v2", commitB, testRepo, false)
	assertSummary(t, sum, nil, nil, nil, []string{".factory/labels.tf"})
	assertAbsent(t, dir, ".factory/labels.tf")
	if readLock(t, dir).Files[0].SHA256 != digest([]byte("labels-v1")) {
		t.Fatal("deletion replaced the locked checksum")
	}
	sum = mustPull(t, dir, map[string]string{"labels.tf": "labels-v2"}, "v2", commitB, testRepo, true)
	assertSummary(t, sum, nil, []string{".factory/labels.tf"}, nil, nil)
	assertFile(t, dir, ".factory/labels.tf", "labels-v2")
}

func TestPullFirstPullConflict(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, ".factory/labels.tf", "existing")
	seed(t, dir, ".github/workflows/factory.yml", "existing workflow")
	_, err := Pull(Request{
		Dir: dir, Source: testRepo, Ref: "main", Commit: commitA,
		Archive: testArchive(t, map[string]string{
			"labels.tf":                    "labels-v1",
			"new.tf":                       "new",
			"github/workflows/factory.yml": "workflow-v1",
		}),
	})
	if err == nil || !strings.Contains(err.Error(), ".factory/labels.tf") || !strings.Contains(err.Error(), ".github/workflows/factory.yml") {
		t.Fatalf("error: %v", err)
	}
	assertFile(t, dir, ".factory/labels.tf", "existing")
	assertFile(t, dir, ".github/workflows/factory.yml", "existing workflow")
	assertAbsent(t, dir, ".factory/new.tf")
	assertAbsent(t, dir, ".factory/gcf.lock")

	sum := mustPull(t, dir, map[string]string{
		"labels.tf":                    "labels-v1",
		"new.tf":                       "new",
		"github/workflows/factory.yml": "workflow-v1",
	}, "main", commitA, testRepo, true)
	assertSummary(t, sum, []string{".factory/labels.tf", ".factory/new.tf", ".github/workflows/factory.yml"}, nil, nil, nil)
	assertFile(t, dir, ".factory/labels.tf", "labels-v1")
	assertFile(t, dir, ".github/workflows/factory.yml", "workflow-v1")
}

func TestPullUpdateConflictDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, dir, map[string]string{"labels.tf": "labels-v1"}, "v1", commitA, testRepo, false)
	seed(t, dir, ".factory/extra.tf", "unmanaged")
	_, err := Pull(Request{
		Dir: dir, Source: testRepo, Ref: "v2", Commit: commitB,
		Archive: testArchive(t, map[string]string{"labels.tf": "labels-v2", "extra.tf": "from-factory"}),
	})
	if err == nil || !strings.Contains(err.Error(), ".factory/extra.tf") {
		t.Fatalf("error: %v", err)
	}
	assertFile(t, dir, ".factory/labels.tf", "labels-v1")
	assertFile(t, dir, ".factory/extra.tf", "unmanaged")
	if readLock(t, dir).Commit != commitA {
		t.Fatal("lock changed after a failed pull")
	}

	sum := mustPull(t, dir, map[string]string{"labels.tf": "labels-v2", "extra.tf": "from-factory"}, "v2", commitB, testRepo, true)
	assertSummary(t, sum, []string{".factory/extra.tf"}, []string{".factory/labels.tf"}, nil, nil)
	assertFile(t, dir, ".factory/extra.tf", "from-factory")
	assertFile(t, dir, ".factory/labels.tf", "labels-v2")
}

func TestPullReservedWritesNothing(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, ".factory/labels.tf", "original")
	seed(t, dir, ".factory/state/terraform.tfstate", "SECRET")
	seed(t, dir, ".factory/labels_override.tf", "OVERRIDE")
	archive := testArchive(t, map[string]string{
		"labels.tf":               "labels-v1",
		"state/terraform.tfstate": "shipped-state",
		"labels_override.tf":      "shipped-override",
	})
	for _, force := range []bool{false, true} {
		_, err := Pull(Request{Dir: dir, Source: testRepo, Ref: "main", Commit: commitA, Force: force, Archive: archive})
		if err == nil || !strings.Contains(err.Error(), ".factory/state/terraform.tfstate") || !strings.Contains(err.Error(), ".factory/labels_override.tf") {
			t.Fatalf("force %v error: %v", force, err)
		}
	}
	assertFile(t, dir, ".factory/labels.tf", "original")
	assertFile(t, dir, ".factory/state/terraform.tfstate", "SECRET")
	assertFile(t, dir, ".factory/labels_override.tf", "OVERRIDE")
	assertAbsent(t, dir, ".factory/gcf.lock")
}

func TestPullDoesNotCreateStateDir(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, dir, map[string]string{"github/workflows/factory.yml": "workflow"}, "main", commitA, testRepo, false)
	assertFile(t, dir, ".github/workflows/factory.yml", "workflow")
	assertAbsent(t, dir, ".factory/state")
	if _, err := os.Stat(filepath.Join(dir, ".factory", "gcf.lock")); err != nil {
		t.Fatal(err)
	}
	assertAbsent(t, dir, ".factory/labels.tf")
}

func TestPullTofuOnlyDoesNotCreateGitHubDir(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, dir, map[string]string{"labels.tf": "labels"}, "main", commitA, testRepo, false)
	assertAbsent(t, dir, ".github")
	assertFile(t, dir, ".factory/labels.tf", "labels")
}

func TestPullRefusesDirectoryAndSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".factory", "labels.tf"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Pull(Request{
		Dir: dir, Source: testRepo, Ref: "main", Commit: commitA, Force: true,
		Archive: testArchive(t, map[string]string{"labels.tf": "labels", "other.tf": "other"}),
	})
	if err == nil || !strings.Contains(err.Error(), ".factory/labels.tf") {
		t.Fatalf("error: %v", err)
	}
	assertAbsent(t, dir, ".factory/other.tf")
	assertAbsent(t, dir, ".factory/gcf.lock")

	dir = t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".factory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", filepath.Join(dir, ".factory", "labels.tf")); err != nil {
		t.Fatal(err)
	}
	_, err = Pull(Request{
		Dir: dir, Source: testRepo, Ref: "main", Commit: commitA, Force: true,
		Archive: testArchive(t, map[string]string{"labels.tf": "labels"}),
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error: %v", err)
	}
	info, statErr := os.Lstat(filepath.Join(dir, ".factory", "labels.tf"))
	if statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was replaced")
	}
}

func TestPullRefusesBadLock(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, ".factory/state/terraform.tfstate", "SECRET")
	writeRawLock(t, dir, `{
  "version": 1,
  "source": "TigranOhanyan/github-cursor-factory",
  "ref": "main",
  "commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "files": [{"path": "../evil.txt", "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}]
}`)
	_, err := Pull(Request{
		Dir: dir, Source: testRepo, Ref: "main", Commit: commitB,
		Archive: testArchive(t, map[string]string{"labels.tf": "labels"}),
	})
	if err == nil || !strings.Contains(err.Error(), "invalid path") {
		t.Fatalf("error: %v", err)
	}
	assertAbsent(t, filepath.Dir(dir), "evil.txt")
	assertAbsent(t, dir, ".factory/labels.tf")

	writeRawLock(t, dir, `{
  "version": 1,
  "source": "TigranOhanyan/github-cursor-factory",
  "ref": "main",
  "commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "files": [{"path": ".factory/state/terraform.tfstate", "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}]
}`)
	_, err = Pull(Request{
		Dir: dir, Source: testRepo, Ref: "main", Commit: commitB,
		Archive: testArchive(t, map[string]string{"labels.tf": "labels"}),
	})
	if err == nil || !strings.Contains(err.Error(), "reserved path") {
		t.Fatalf("error: %v", err)
	}
	assertFile(t, dir, ".factory/state/terraform.tfstate", "SECRET")

	writeRawLock(t, dir, `{"version": 9}`)
	_, err = Pull(Request{
		Dir: dir, Source: testRepo, Ref: "main", Commit: commitB,
		Archive: testArchive(t, map[string]string{"labels.tf": "labels"}),
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported version") {
		t.Fatalf("error: %v", err)
	}
}

func TestPullAdoptsMatchingUnmanagedFile(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, ".factory/labels.tf", "labels-v1")
	sum := mustPull(t, dir, map[string]string{"labels.tf": "labels-v1"}, "main", commitA, testRepo, true)
	assertSummary(t, sum, []string{".factory/labels.tf"}, nil, nil, nil)
	assertFile(t, dir, ".factory/labels.tf", "labels-v1")
	if readLock(t, dir).Files[0].SHA256 != digest([]byte("labels-v1")) {
		t.Fatal("adopted checksum")
	}
}

func TestPullReconcilesEditThatMatchesIncoming(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, dir, map[string]string{"labels.tf": "labels-v1"}, "v1", commitA, testRepo, false)
	seed(t, dir, ".factory/labels.tf", "labels-v2")
	sum := mustPull(t, dir, map[string]string{"labels.tf": "labels-v2"}, "v2", commitB, testRepo, false)
	assertSummary(t, sum, nil, nil, nil, nil)
	assertFile(t, dir, ".factory/labels.tf", "labels-v2")
	if readLock(t, dir).Files[0].SHA256 != digest([]byte("labels-v2")) || readLock(t, dir).Commit != commitB {
		t.Fatalf("lock: %+v", readLock(t, dir))
	}
}

func TestPullRejectsBadCommit(t *testing.T) {
	_, err := Pull(Request{
		Dir: t.TempDir(), Source: testRepo, Ref: "main", Commit: "abc",
		Archive: testArchive(t, map[string]string{"labels.tf": "x"}),
	})
	if err == nil || !strings.Contains(err.Error(), "40-character") {
		t.Fatalf("error: %v", err)
	}
}

func mustPull(t *testing.T, dir string, files map[string]string, ref, commit, repo string, force bool) Summary {
	t.Helper()
	sum, err := Pull(Request{
		Dir:     dir,
		Source:  repo,
		Ref:     ref,
		Commit:  commit,
		Force:   force,
		Archive: testArchive(t, files),
	})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	return sum
}

func seed(t *testing.T, dir, path, content string) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, dir, path, content string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != content {
		t.Fatalf("%s: got %q want %q", path, body, content)
	}
}

func assertAbsent(t *testing.T, dir, path string) {
	t.Helper()
	_, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(path)))
	if !os.IsNotExist(err) {
		t.Fatalf("%s exists or stat failed: %v", path, err)
	}
}

func assertSummary(t *testing.T, got Summary, added, updated, removed, skipped []string) {
	t.Helper()
	if added == nil {
		added = []string{}
	}
	if updated == nil {
		updated = []string{}
	}
	if removed == nil {
		removed = []string{}
	}
	if skipped == nil {
		skipped = []string{}
	}
	if !reflect.DeepEqual(got.Added, added) || !reflect.DeepEqual(got.Updated, updated) ||
		!reflect.DeepEqual(got.Removed, removed) || !reflect.DeepEqual(got.Skipped, skipped) {
		t.Fatalf("added %v\nupdated %v\nremoved %v\nskipped %v\nwarnings %v", got.Added, got.Updated, got.Removed, got.Skipped, got.Warnings)
	}
}

func readLock(t *testing.T, dir string) Lock {
	t.Helper()
	var lock Lock
	if err := json.Unmarshal([]byte(readRawLock(t, dir)), &lock); err != nil {
		t.Fatal(err)
	}
	return lock
}

func readRawLock(t *testing.T, dir string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, ".factory", "gcf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func writeRawLock(t *testing.T, dir, content string) {
	t.Helper()
	abs := filepath.Join(dir, ".factory", "gcf.lock")
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileIndex(lock Lock, path string) int {
	for i, file := range lock.Files {
		if file.Path == path {
			return i
		}
	}
	return -1
}

func lockPathSet(lock Lock) map[string]struct{} {
	out := make(map[string]struct{}, len(lock.Files))
	for _, file := range lock.Files {
		out[file.Path] = struct{}{}
	}
	return out
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
