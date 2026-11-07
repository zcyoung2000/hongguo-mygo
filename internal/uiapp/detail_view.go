package uiapp

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

const detailEpisodeCollapsedRows = 3

//go:embed icons/chevron-down.svg
var chevronSVG []byte

var chevronIcon = ui.MustParseSVG(chevronSVG)

func (a *App) detailEpisodeCollapsedIndexes(episodes []CatalogItem) []int {
	count := detailEpisodeCollapsedCount(len(episodes))
	indexes := make([]int, 0, count+1)
	for index := range episodes[:count] {
		indexes = append(indexes, index)
	}
	current := a.currentDetailEpisodeIndex(episodes)
	if current >= count && current < len(episodes) {
		indexes = append(indexes, current)
	}
	return indexes
}

func (a *App) currentDetailEpisodeIndex(episodes []CatalogItem) int {
	for index, episode := range episodes {
		if a.resume.ResumeEpisode > 0 && episode.EpisodeCount == a.resume.ResumeEpisode {
			return index
		}
	}
	return -1
}

func detailEpisodeCollapsedCount(episodeCount int) int {
	if episodeCount <= detailEpisodeCollapsedRows*16 {
		return episodeCount
	}
	return detailEpisodeCollapsedRows*16 - 1
}
