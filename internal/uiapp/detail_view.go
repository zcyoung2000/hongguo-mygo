package uiapp

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

const detailEpisodeCollapsedRows = 3
const detailEpisodeCollapsedColumns = 4

//go:embed icons/chevron-down.svg
var chevronSVG []byte

var chevronIcon = ui.MustParseSVG(chevronSVG)

func (a *App) detailEpisodeCollapsedIndexes(episodes []CatalogItem) []int {
	count := detailEpisodeCollapsedCount(len(episodes))
	indexes := make([]int, 0, count+1)
	for index := range episodes[:count] {
		indexes = append(indexes, index)
	}
	return indexes
}

func detailEpisodeCollapsedCount(episodeCount int) int {
	if episodeCount <= detailEpisodeCollapsedRows*detailEpisodeCollapsedColumns {
		return episodeCount
	}
	return detailEpisodeCollapsedRows * detailEpisodeCollapsedColumns
}
