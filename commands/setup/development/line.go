package development

import (
	"strconv"

	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func Line(row configrecords.Development) (text string) {
	standards := strconv.FormatInt(row.StandardsReviewerApp, 10)
	specification := strconv.FormatInt(row.SpecificationReviewerApp, 10)
	text = row.Name + "  " + row.StartingLabel + "  " + row.AttentionLabel + "  " + standards + "  " + specification
	return
}
