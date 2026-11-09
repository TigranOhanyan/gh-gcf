package label

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

var colorHex = regexp.MustCompile(`^[0-9a-f]{6}$`)

func Add(ctx context.Context, db *configdb.DB, rawName string, rawColor string) (name string, err error) {
	name, err = common.NormalizeLabelName(rawName)
	if err != nil {
		return
	}
	color, err := normalizeColor(rawColor)
	if err != nil {
		return
	}

	labels, err := db.Queries.ListLabels(ctx)
	if err != nil {
		log.Error("list labels", "err", err)
		return
	}
	for _, existing := range labels {
		same := existing.Name == name
		if same {
			err = fmt.Errorf("label %s already exists", name)
			log.Info("label exists", "name", name)
			return
		}
	}

	params := configrecords.CreateLabelParams{
		Name:  name,
		Color: color,
	}
	err = db.Queries.CreateLabel(ctx, params)
	if err != nil {
		log.Error("create label", "name", name, "err", err)
		return
	}
	log.Info("create label", "name", name, "color", color)
	return
}

func normalizeColor(raw string) (color string, err error) {
	color = strings.TrimSpace(raw)
	color = strings.TrimPrefix(color, "#")
	color = strings.ToLower(color)
	matched := colorHex.MatchString(color)
	if !matched {
		err = fmt.Errorf("color must be six hex digits")
		color = ""
	}
	return
}
