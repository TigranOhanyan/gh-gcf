package development

import (
	"strconv"

	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func Line(row configrecords.Development) (text string) {
	standards := strconv.FormatInt(row.StandardsReviewerApp, 10)
	specification := strconv.FormatInt(row.SpecificationReviewerApp, 10)
	builder := strconv.FormatInt(row.BuilderApp, 10)
	fixer := strconv.FormatInt(row.FixerApp, 10)
	text = row.Name + "  " + row.StartingLabel + "  " + row.AttentionLabel + "  " + standards + "  " + specification + "  " + builder + "  " + fixer
	return
}
