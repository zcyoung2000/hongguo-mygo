package uiapp

type ResumeProgress struct {
	SeriesID   string  `json:"seriesId"`
	EpisodeNum int     `json:"episodeNum"`
	Position   float64 `json:"positionSec"`
	Session    int64   `json:"session"`
	Sequence   uint64  `json:"sequence"`
}

func (a *App) UpdateResumeProgress(payload ResumeProgress) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if payload.Sequence == 0 || payload.Sequence <= a.resumeSequence || payload.SeriesID == "" || payload.EpisodeNum <= 0 {
		return
	}
	a.resumeSequence = payload.Sequence
	if payload.Session == a.mediaSession && payload.SeriesID == a.detail.ID {
		a.resume = CatalogItem{ID: payload.SeriesID, ResumeEpisode: payload.EpisodeNum, ResumePosition: payload.Position}
		a.invalidate()
	}
}

func (a *App) resetResumeLocked(seriesID string) {
	a.mediaOn = false
	if a.detail.ID != seriesID {
		a.resume = CatalogItem{}
	}
	a.resumeSequence = 0
}
