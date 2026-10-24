package pull

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestMapArchiveLayout(t *testing.T) {
	files, err := mapArchive(testArchive(t, map[string]string{
		"labels.tf":                         "labels",
		"empty.tf":                          "",
		"statement.tf":                      "statement",
		"docs/README.md":                    "nested readme",
		"subdir/.gitignore":                 "nested ignore",
		"github/helpers/note.txt":           "note",
		"github/workflows/factory.yml":      "workflow",
		"github/workflows/nested/extra.yml": "extra",
		"modules/labels_override.tf":        "nested override",
		"README.md":                         "factory readme",
		".gitignore":                        "factory ignore",
		".github/workflows/validate.yml":    "factory ci",
		"LICENSE":                           "license",
		strings.Repeat("n", 180) + ".tf":    "long",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		".factory/LICENSE",
		".factory/docs/README.md",
		".factory/empty.tf",
		".factory/github/helpers/note.txt",
		".factory/labels.tf",
		".factory/modules/labels_override.tf",
		".factory/" + strings.Repeat("n", 180) + ".tf",
		".factory/statement.tf",
		".factory/subdir/.gitignore",
		".github/workflows/factory.yml",
		".github/workflows/nested/extra.yml",
	}
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d: %v", len(files), len(want), keys(files))
	}
	for _, path := range want {
		if _, ok := files[path]; !ok {
			t.Errorf("missing %s", path)
		}
	}
	if files[".factory/labels.tf"].body == nil || string(files[".factory/labels.tf"].body) != "labels" {
		t.Fatalf("labels body: %q", files[".factory/labels.tf"].body)
	}
	if string(files[".factory/empty.tf"].body) != "" {
		t.Fatal("empty file body")
	}
	for _, excluded := range []string{".factory/README.md", ".factory/.gitignore", ".github/workflows/validate.yml", "README.md", ".gitignore"} {
		if _, ok := files[excluded]; ok {
			t.Errorf("included excluded path %s", excluded)
		}
	}
}

func TestMapArchiveFileNamedWorkflows(t *testing.T) {
	files, err := mapArchive(testArchive(t, map[string]string{
		"github/workflows": "not-a-workflow-directory",
	}))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := files[".factory/github/workflows"]
	if !ok || string(got.body) != "not-a-workflow-directory" {
		t.Fatalf("files: %v", keys(files))
	}
}

func TestMapArchiveReserved(t *testing.T) {
	_, err := mapArchive(testArchive(t, map[string]string{
		"labels.tf":               "ok",
		"state/terraform.tfstate": "secret",
		"state/nested/file":       "secret",
		"labels_override.tf":      "override",
		"gcf.lock":                "lock",
	}))
	if err == nil {
		t.Fatal("expected reserved-path error")
	}
	for _, path := range []string{
		"state/terraform.tfstate -> .factory/state/terraform.tfstate",
		"state/nested/file -> .factory/state/nested/file",
		"labels_override.tf -> .factory/labels_override.tf",
		"gcf.lock -> .factory/gcf.lock",
	} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error missing %s:\n%v", path, err)
		}
	}
}

func TestMapArchiveRejectsUnsafe(t *testing.T) {
	cases := []struct {
		name    string
		entries []tarEntry
		want    string
	}{
		{
			name: "symlink",
			entries: []tarEntry{{
				name:     githubPrefix + "link.tf",
				typeflag: tar.TypeSymlink,
				link:     "labels.tf",
			}},
			want: "link",
		},
		{
			name: "traversal",
			entries: []tarEntry{{
				name: githubPrefix + "../outside.txt",
				body: "nope",
			}},
			want: "invalid factory archive path",
		},
		{
			name: "two roots",
			entries: []tarEntry{
				{name: "one/a.txt", body: "a"},
				{name: "two/b.txt", body: "b"},
			},
			want: "multiple top-level",
		},
		{
			name: "duplicate",
			entries: []tarEntry{
				{name: githubPrefix + "labels.tf", body: "a"},
				{name: githubPrefix + "labels.tf", body: "b"},
			},
			want: "more than one file",
		},
		{
			name:    "empty",
			entries: nil,
			want:    "no files",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mapArchive(buildTar(t, tc.entries))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error: %v", err)
			}
		})
	}
	if _, err := mapArchive([]byte("not a gzip")); err == nil {
		t.Fatal("expected gzip error")
	}
}

func keys(files map[string]factoryFile) []string {
	out := make([]string, 0, len(files))
	for path := range files {
		out = append(out, path)
	}
	return out
}

const githubPrefix = "TigranOhanyan-github-cursor-factory-7e4e379/"

type tarEntry struct {
	name     string
	body     string
	typeflag byte
	link     string
}

func testArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	entries := []tarEntry{{name: githubPrefix, typeflag: tar.TypeDir}}
	for name, body := range files {
		entries = append(entries, tarEntry{name: githubPrefix + name, body: body})
	}
	return buildTar(t, entries)
}

func buildTar(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, entry := range entries {
		flag := entry.typeflag
		if flag == 0 {
			flag = tar.TypeReg
		}
		hdr := &tar.Header{
			Name:     entry.name,
			Mode:     0o644,
			Size:     int64(len(entry.body)),
			Typeflag: flag,
			Linkname: entry.link,
		}
		if flag == tar.TypeDir {
			hdr.Mode = 0o755
			hdr.Size = 0
		}
		if flag == tar.TypeSymlink || flag == tar.TypeLink {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if flag == tar.TypeReg && len(entry.body) > 0 {
			if _, err := tw.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
