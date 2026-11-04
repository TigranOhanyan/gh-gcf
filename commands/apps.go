package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"charm.land/log/v2"
)

// ListAppIDs reads the factory's registered App IDs from apps/<app id>.
// A missing apps directory is an empty list.
func ListAppIDs(factoryDir string) (ids []int64, err error) {
	dir := filepath.Join(factoryDir, "apps")
	entries, err := os.ReadDir(dir)
	missing := os.IsNotExist(err)
	if missing {
		err = nil
		return
	}
	if err != nil {
		log.Error("list apps", "path", dir, "err", err)
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		id, parseErr := strconv.ParseInt(name, 10, 64)
		canonical := strconv.FormatInt(id, 10)
		isDir := entry.IsDir()
		bad := isDir || parseErr != nil || id <= 0 || canonical != name
		if bad {
			err = fmt.Errorf("app file name is not an app id: %s", name)
			log.Error("list apps", "err", err)
			return
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i int, j int) bool {
		return ids[i] < ids[j]
	})
	return
}
