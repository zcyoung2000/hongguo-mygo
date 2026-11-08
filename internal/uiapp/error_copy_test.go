package uiapp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

type resultErrorProvider struct {
	rankResult    ipc.RankResponse
	releaseResult ipc.NewReleaseResponse
	profile       *ipc.ActorProfileDTO
	rankErr       error
	releaseErr    error
	profileErr    error
}

func (p *resultErrorProvider) GetRankings(context.Context, string, string, string) (ipc.RankResponse, error) {
	return p.rankResult, p.rankErr
}

func (p *resultErrorProvider) GetNewReleases(context.Context, string) (ipc.NewReleaseResponse, error) {
	return p.releaseResult, p.releaseErr
}

func (p *resultErrorProvider) GetActorProfile(context.Context, string) (*ipc.ActorProfileDTO, error) {
	return p.profile, p.profileErr
}

func (p *resultErrorProvider) GetActorWorks(context.Context, string, string) (*ipc.ActorWorksResponse, error) {
	return nil, nil
}

func (p *resultErrorProvider) GetSuggestions(context.Context, string) ([]string, error) {
	return nil, nil
}

func (p *resultErrorProvider) GetSeriesSeasons(context.Context, string) (*ipc.SeriesCollectionDTO, error) {
	return nil, nil
}

func (p *resultErrorProvider) GetSeriesAdaptations(context.Context, string, string, string, string, string) (*ipc.SeriesCollectionDTO, error) {
	return nil, nil
}

func TestUserFacingErrorMapsAuthNetworkAndUnknownErrors(t *testing.T) {
	apiErr := &sdk.APIError{HTTPStatus: http.StatusUnauthorized}
	timeoutErr := &net.DNSError{IsTimeout: true}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "auth sentinel", err: sdk.ErrAuthenticationRequired, want: "会话已失效，请重新登录"},
		{name: "http unauthorized", err: errors.Join(sdk.ErrTransport, apiErr), want: "会话已失效，请重新登录"},
		{name: "rate limited", err: sdk.ErrRateLimited, want: "网络异常或访问受限，请稍后重试；匿名访问受限时可在设置中重置设备"},
		{name: "anonymous limited", err: sdk.ErrAnonymousLimited, want: "网络异常或访问受限，请稍后重试；匿名访问受限时可在设置中重置设备"},
		{name: "device rejected", err: sdk.ErrDeviceRejected, want: "网络异常或访问受限，请稍后重试；匿名访问受限时可在设置中重置设备"},
		{name: "network timeout", err: &net.OpError{Op: "get", Err: timeoutErr}, want: "网络异常，请检查网络后重试"},
		{name: "deadline", err: context.DeadlineExceeded, want: "网络异常或访问受限，请稍后重试；匿名访问受限时可在设置中重置设备"},
		{name: "unknown english", err: errors.New("ranking API returned 401"), want: "加载失败，请稍后重试"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := userFacingError(test.err); got != test.want {
				t.Fatalf("userFacingError() = %q, want %q", got, test.want)
			}
			if got := userFacingError(nil); got != "" {
				t.Fatalf("userFacingError(nil) = %q, want empty", got)
			}
		})
	}
}

func waitFeature(t *testing.T, app *App, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		app.mu.Lock()
		ready := done()
		app.mu.Unlock()
		if ready {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("feature load did not finish")
}

func TestFeatureErrorsUseUnifiedChineseCopy(t *testing.T) {
	provider := &resultErrorProvider{
		rankErr:    errors.New("ranking API returned 401"),
		releaseErr: sdk.ErrAuthenticationRequired,
		profileErr: &sdk.APIError{HTTPStatus: http.StatusUnauthorized},
	}
	app := NewTestApp()
	app.SetInvalidator(func() {})
	app.featuresSource = provider
	app.testMode = false
	app.LoadRank(false)
	waitFeature(t, app, func() bool { return !app.rank.Loading })
	if app.rank.Error != "加载失败，请稍后重试" {
		t.Fatalf("rank error = %q, want unified unknown copy", app.rank.Error)
	}

	app.SelectRelease(true, "")
	waitFeature(t, app, func() bool { return !app.releases.Loading })
	if app.releases.Error != "会话已失效，请重新登录" {
		t.Fatalf("release error = %q, want auth copy", app.releases.Error)
	}

	app.actor.ID = "123456"
	app.actor.version++
	app.actor.Loading = true
	go app.loadActor(false)
	waitFeature(t, app, func() bool { return !app.actor.Loading && !app.actor.requesting })
	if app.actor.Error != "会话已失效，请重新登录" {
		t.Fatalf("actor error = %q, want auth copy", app.actor.Error)
	}
}

func TestSuccessfulFilteredNewReleasesUseHotBoardEmptyCopy(t *testing.T) {
	provider := &resultErrorProvider{rankResult: ipc.RankResponse{
		Content: "all",
		Board:   "ranklist_new_rank_sc",
		Items:   []ipc.RankItemDTO{{SeriesID: "123456", ItemType: "actor", Title: "演员"}},
	}}
	app := NewTestApp()
	app.SetInvalidator(func() {})
	app.featuresSource = provider
	app.testMode = false
	app.SelectRelease(false, "")
	waitFeature(t, app, func() bool { return !app.releases.Loading })
	if app.releases.Error != "" {
		t.Fatalf("release error = %q, want empty", app.releases.Error)
	}
	if len(app.releases.Items) != 0 {
		t.Fatalf("release items = %+v, want filtered empty", app.releases.Items)
	}
	app.view = "new"
	tester := ui.NewTester(app.Render, 1200, 800)
	if !tester.HasText("当前新剧热榜暂无剧集") || tester.HasText("暂无新剧") {
		t.Fatalf("empty copy = %v", tester.Texts())
	}
}

func TestSuccessfulReleasesKeepResultPriorityWhenErrorAlsoPresent(t *testing.T) {
	provider := &resultErrorProvider{
		releaseResult: ipc.NewReleaseResponse{CurrentDate: "20261109", Items: []ipc.NewReleaseItemDTO{{SeriesID: "123456", Title: "新剧"}}},
		releaseErr:    errors.New("ignored diagnostics"),
	}
	app := NewTestApp()
	app.SetInvalidator(func() {})
	app.featuresSource = provider
	app.testMode = false
	app.SelectRelease(true, "")
	waitFeature(t, app, func() bool { return !app.releases.Loading })
	if app.releases.Error == "" || len(app.releases.Items) != 0 {
		t.Fatalf("releases = %+v, want error to override successful result", app.releases)
	}
}
