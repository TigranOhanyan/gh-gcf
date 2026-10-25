package pull

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

const (
	maxArchiveBytes = 64 << 20
	maxFileBytes    = 32 << 20
	maxFiles        = 10000
)

// factoryFile is one file from the factory archive, addressed by its host path.
type factoryFile struct {
	source string
	dest   string
	body   []byte
}

func mapArchive(archive []byte) (map[string]factoryFile, error) {
	if len(archive) == 0 {
		return nil, fmt.Errorf("factory archive is empty")
	}
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("factory archive is not a gzip tarball: %w", err)
	}
	defer zr.Close()
	tr := tar.NewReader(zr)

	var (
		files  []factoryFile
		prefix string
		total  int64
		count  int
	)
	var reserved []string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read factory archive: %w", err)
		}
		switch hdr.Typeflag {
		case tar.TypeXHeader, tar.TypeXGlobalHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
			if err := discardEntry(tr, hdr.Size); err != nil {
				return nil, err
			}
			continue
		case tar.TypeDir:
			if err := discardEntry(tr, hdr.Size); err != nil {
				return nil, err
			}
			if _, err := cleanArchiveName(hdr.Name); err != nil {
				return nil, err
			}
			continue
		case tar.TypeSymlink, tar.TypeLink:
			return nil, fmt.Errorf("factory archive contains a link: %s", hdr.Name)
		case tar.TypeReg, tar.TypeRegA:
		default:
			return nil, fmt.Errorf("factory archive contains unsupported entry %q", hdr.Name)
		}

		name, err := cleanArchiveName(hdr.Name)
		if err != nil {
			return nil, err
		}
		if hdr.Size < 0 || hdr.Size > maxFileBytes {
			return nil, fmt.Errorf("factory file %s is too large", name)
		}
		if total+hdr.Size > maxArchiveBytes {
			return nil, fmt.Errorf("factory archive exceeds %d bytes", maxArchiveBytes)
		}
		body, err := readEntry(tr, hdr.Size)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		total += hdr.Size
		count++
		if count > maxFiles {
			return nil, fmt.Errorf("factory archive has too many files")
		}

		rel, top, err := stripPrefix(name, &prefix)
		if err != nil {
			return nil, err
		}
		if top == "" {
			return nil, fmt.Errorf("factory archive file %q is outside the top-level directory", hdr.Name)
		}
		dest, include, err := destFor(rel)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		if reservedPath(dest) {
			reserved = append(reserved, fmt.Sprintf("%s -> %s", rel, dest))
			continue
		}
		files = append(files, factoryFile{source: rel, dest: dest, body: body})
	}
	if len(reserved) > 0 {
		sort.Strings(reserved)
		return nil, fmt.Errorf("factory archive contains paths that would write reserved files:\n%s\ngh gcf does not write .factory/state/, .factory/*_override.tf, or .factory/gcf.lock", bullets(reserved))
	}
	if prefix == "" {
		return nil, fmt.Errorf("factory archive contains no files")
	}

	out := make(map[string]factoryFile, len(files))
	for _, file := range files {
		if _, exists := out[file.dest]; exists {
			return nil, fmt.Errorf("factory archive maps more than one file to %s", file.dest)
		}
		out[file.dest] = file
	}
	return out, nil
}

func stripPrefix(name string, prefix *string) (rel, top string, err error) {
	parts := strings.Split(name, "/")
	if len(parts) < 2 || parts[0] == "" {
		return "", "", fmt.Errorf("factory archive file %q is outside the top-level directory", name)
	}
	top = parts[0]
	if *prefix == "" {
		*prefix = top
	} else if *prefix != top {
		return "", "", fmt.Errorf("factory archive has multiple top-level directories (%s and %s)", *prefix, top)
	}
	return strings.Join(parts[1:], "/"), top, nil
}

func destFor(rel string) (dest string, include bool, err error) {
	rel = path.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
		return "", false, fmt.Errorf("invalid factory path %q", rel)
	}
	switch {
	case rel == "README.md" || rel == ".gitignore":
		return "", false, nil
	case rel == ".github" || strings.HasPrefix(rel, ".github/"):
		return "", false, nil
	case strings.HasPrefix(rel, "github/workflows/"):
		rest := strings.TrimPrefix(rel, "github/workflows/")
		if rest == "" || rest == "." {
			return "", false, fmt.Errorf("invalid workflow path %q", rel)
		}
		dest = path.Clean(path.Join(".github/workflows", rest))
		if !strings.HasPrefix(dest, ".github/workflows/") {
			return "", false, fmt.Errorf("refusing to map %s outside .github/workflows", rel)
		}
	default:
		dest = path.Clean(path.Join(".factory", rel))
		if !strings.HasPrefix(dest, ".factory/") {
			return "", false, fmt.Errorf("refusing to map %s outside .factory", rel)
		}
	}
	return dest, true, nil
}

// reservedPath reports paths the CLI must never write or delete.
// *_override.tf matches only files directly inside .factory/.
func reservedPath(dest string) bool {
	if dest == lockRelPath || dest == ".factory/state" || strings.HasPrefix(dest, ".factory/state/") {
		return true
	}
	rest, ok := strings.CutPrefix(dest, ".factory/")
	if !ok || strings.Contains(rest, "/") {
		return false
	}
	return strings.HasSuffix(rest, "_override.tf")
}

func cleanArchiveName(name string) (string, error) {
	if strings.Contains(name, "\\") || strings.Contains(name, "\x00") {
		return "", fmt.Errorf("invalid factory archive path %q", name)
	}
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimSuffix(name, "/")
	if name == "" || name == "." {
		return "", nil
	}
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("invalid factory archive path %q", name)
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." {
			return "", fmt.Errorf("invalid factory archive path %q", name)
		}
	}
	cleaned := path.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") || cleaned == "." {
		return "", fmt.Errorf("invalid factory archive path %q", name)
	}
	return cleaned, nil
}

func discardEntry(tr *tar.Reader, size int64) error {
	if size < 0 {
		return fmt.Errorf("invalid archive entry size")
	}
	_, err := io.Copy(io.Discard, io.LimitReader(tr, size))
	return err
}

func readEntry(tr *tar.Reader, size int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(tr, size))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) != size {
		return nil, fmt.Errorf("short file in archive")
	}
	return body, nil
}

func bullets(lines []string) string {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
