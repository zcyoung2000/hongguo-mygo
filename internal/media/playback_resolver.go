package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

const (
	playbackRaceConcurrency = 3
	playbackRaceTimeout     = 30 * time.Second
	playbackResponseLimit   = 8 << 20
	fallbackUserAgent       = "com.phoenix.read/73932 (Linux; U; Android 14; zh_CN; 23127PN0CC; Build/UKQ1.230804.001; Cronet/TTNetVersion:8d40f833 2026-03-03 QuicVersion:21ac1950 2025-11-18)"
)

type PlaybackResolver interface {
	ResolvePlayback(ctx context.Context, episodeID string) (*playbackResolveResult, error)
}

type playbackResolveResult struct {
	Playback *model.Playback
	Fallback string
}

type playbackSource struct {
	name string
	call func(ctx context.Context, episodeID string) (*playbackResolveResult, error)
}

type playbackSourceResult struct {
	name   string
	result *playbackResolveResult
	err    error
}

var errPlaybackModelUnavailable = errors.New("playback model unavailable")

type playbackRaceResolver struct {
	sources []playbackSource
	timeout time.Duration
}

func newPlaybackRaceResolver(primary, spare PlaybackResolver) *playbackRaceResolver {
	sources := make([]playbackSource, 0, 2)
	if primary != nil {
		sources = append(sources, playbackSource{name: "video_model", call: primary.ResolvePlayback})
	}
	if spare != nil {
		sources = append(sources, playbackSource{name: "multi_video_model", call: spare.ResolvePlayback})
	}
	return &playbackRaceResolver{sources: sources, timeout: playbackRaceTimeout}
}

func (r *playbackRaceResolver) ResolvePlayback(ctx context.Context, episodeID string) (*playbackResolveResult, error) {
	if len(r.sources) > playbackRaceConcurrency {
		r.sources = r.sources[:playbackRaceConcurrency]
	}
	if len(r.sources) == 0 {
		return nil, errors.New("no playback source")
	}
	timeout := r.timeout
	if timeout <= 0 {
		timeout = playbackRaceTimeout
	}
	raceCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	results := make(chan playbackSourceResult, len(r.sources))
	wg := &sync.WaitGroup{}
	wg.Add(len(r.sources))
	for _, source := range r.sources {
		go func(source playbackSource) {
			defer wg.Done()
			result, err := source.call(raceCtx, episodeID)
			results <- playbackSourceResult{name: source.name, result: result, err: err}
		}(source)
	}
	resultsClosed := make(chan struct{})
	go func() {
		wg.Wait()
		close(results)
		close(resultsClosed)
	}()

	var failures []error
	var terminal error
	for event := range results {
		if event.err == nil && event.result != nil && playbackUsable(event.result.Playback) {
			return event.result, nil
		}
		if event.err == nil && event.result != nil && strings.TrimSpace(event.result.Fallback) != "" {
			return event.result, nil
		}
		if event.err != nil && !playbackSwitchable(event.err) {
			terminal = event.err
			cancel()
			<-results
			break
		}
		if event.err == nil {
			event.err = errPlaybackModelUnavailable
		}
		failures = append(failures, fmt.Errorf("%s: %w", event.name, event.err))
	}
	cancel()
	if terminal != nil {
		<-resultsClosed
	}
	if terminal != nil {
		return nil, errors.Join(append(failures, terminal)...)
	}
	return nil, errors.Join(failures...)
}

func playbackUsable(pb *model.Playback) bool {
	return pb != nil && len(pb.Streams) > 0 && pb.Streams[0].URL != ""
}

func playbackSwitchable(err error) bool {
	return errors.Is(err, sdk.ErrAnonymousLimited) || sdk.Retryable(err)
}

func newSDKPlaybackResolver(client *sdk.Client) *sdkPlaybackResolver {
	return &sdkPlaybackResolver{client: client}
}

type sdkPlaybackResolver struct {
	client *sdk.Client
	http   *http.Client
}

func (r sdkPlaybackResolver) fetchPrimary(ctx context.Context, episodeID string) (*playbackResolveResult, error) {
	pb, err := r.client.ResolvePlayback(ctx, episodeID, sdk.PlaybackOptions{})
	if err != nil && !errors.Is(err, sdk.ErrDRMProtected) {
		return nil, err
	}
	return &playbackResolveResult{Playback: pb}, nil
}

func (r sdkPlaybackResolver) fetchFallback(ctx context.Context, fallbackURL string) (*playbackResolveResult, error) {
	target, err := url.Parse(strings.TrimSpace(fallbackURL))
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, errors.New("invalid fallback_api URL")
	}
	httpClient := r.http
	if httpClient == nil {
		httpClient = &http.Client{Timeout: playbackRaceTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fallbackUserAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fallback_api returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, playbackResponseLimit))
	if err != nil {
		return nil, err
	}
	if len(raw) >= playbackResponseLimit {
		return nil, errors.New("fallback_api response is too large")
	}
	episodeID := fallbackEpisodeID(target)
	if !isHexID(episodeID) {
		return nil, errors.New("fallback_api is missing an episode id")
	}
	pb, parseErr := parsePlaybackWithFallback(raw, episodeID)
	if parseErr != nil {
		return nil, parseErr
	}
	return &playbackResolveResult{Playback: pb}, nil
}

func (r sdkPlaybackResolver) ResolvePlayback(ctx context.Context, episodeID string) (*playbackResolveResult, error) {
	if r.client == nil {
		return nil, errNilPlaybackClient
	}
	return r.fetchPrimary(ctx, episodeID)
}

type playbackFallbackResolver struct {
	inner   PlaybackResolver
	fetcher func(ctx context.Context, fallbackURL string) (*playbackResolveResult, error)
}

func newSDKFallbackPlaybackResolver(client *sdk.Client, httpClient *http.Client) *playbackFallbackResolver {
	sdkResolver := sdkPlaybackResolver{client: client, http: httpClient}
	return &playbackFallbackResolver{
		inner:   &sdkResolver,
		fetcher: sdkResolver.fetchFallback,
	}
}

func (r playbackFallbackResolver) ResolvePlayback(ctx context.Context, episodeID string) (*playbackResolveResult, error) {
	result, err := r.inner.ResolvePlayback(ctx, episodeID)
	if err != nil && !playbackSwitchable(err) {
		return result, err
	}
	if result == nil {
		return nil, err
	}
	if playbackUsable(result.Playback) {
		return result, nil
	}
	fallback := strings.TrimSpace(result.Fallback)
	if fallback == "" {
		return result, err
	}
	return r.fetcher(ctx, fallback)
}

func parseFallbackAPI(raw []byte) (string, error) {
	var body struct {
		Data struct {
			VideoModel json.RawMessage `json:"video_model"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", err
	}
	modelBytes := bytes.TrimSpace(body.Data.VideoModel)
	if len(modelBytes) == 0 || string(modelBytes) == "null" {
		return "", errors.New("video model is empty")
	}
	if modelBytes[0] == '"' {
		var encoded string
		if err := json.Unmarshal(modelBytes, &encoded); err != nil {
			return "", err
		}
		modelBytes = []byte(encoded)
	}
	var parsed struct {
		FallbackAPI string `json:"fallback_api"`
	}
	if err := json.Unmarshal(modelBytes, &parsed); err != nil {
		return "", err
	}
	if strings.TrimSpace(parsed.FallbackAPI) == "" {
		return "", errors.New("fallback_api is empty")
	}
	return strings.TrimSpace(parsed.FallbackAPI), nil
}

func isHexID(id string) bool {
	return len(id) >= 4 && len(id) <= 25 && strings.Trim(id, "0123456789abcdefABCDEF") == ""
}

func fallbackEpisodeID(target *url.URL) string {
	id := target.Query().Get("video_id")
	if id == "" {
		id = target.Query().Get("item_id")
	}
	return id
}

func parsePlaybackWithFallback(raw []byte, episodeID string) (*model.Playback, error) {
	var envelope struct {
		Code json.RawMessage `json:"code"`
		Data struct {
			VideoModel json.RawMessage `json:"video_model"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	code := strings.Trim(strings.TrimSpace(string(envelope.Code)), `"`)
	if code != "" && code != "0" {
		return nil, fmt.Errorf("fallback playback code %s", code)
	}
	modelBytes := bytes.TrimSpace(envelope.Data.VideoModel)
	if len(modelBytes) == 0 || string(modelBytes) == "null" {
		return nil, errPlaybackModelUnavailable
	}
	if modelBytes[0] == '"' {
		var encoded string
		if err := json.Unmarshal(modelBytes, &encoded); err != nil {
			return nil, err
		}
		modelBytes = []byte(encoded)
	}
	var video struct {
		Status    int             `json:"status"`
		VideoID   string          `json:"video_id"`
		Duration  float64         `json:"video_duration"`
		ExpiresAt int64           `json:"url_expire"`
		VideoList json.RawMessage `json:"video_list"`
	}
	if err := json.Unmarshal(modelBytes, &video); err != nil {
		return nil, err
	}
	var representations map[string]struct {
		MainURL     string `json:"main_url"`
		BackupURL   string `json:"backup_url_1"`
		Definition  string `json:"definition"`
		Codec       string `json:"codec"`
		Width       int    `json:"vwidth"`
		Height      int    `json:"vheight"`
		Bitrate     int64  `json:"bit_rate"`
		Size        int64  `json:"file_size"`
		EncryptInfo struct {
			SpadeA string `json:"spade_a"`
			Kid    string `json:"kid"`
		} `json:"encrypt_info"`
	}
	if err := json.Unmarshal(video.VideoList, &representations); err != nil {
		return nil, err
	}
	pb := &model.Playback{EpisodeID: episodeID, VideoID: video.VideoID, DurationSeconds: int(video.Duration + 0.5), ExpiresAt: video.ExpiresAt}
	for _, representation := range representations {
		if strings.TrimSpace(representation.MainURL) == "" {
			continue
		}
		protected := representation.EncryptInfo.SpadeA != "" || representation.EncryptInfo.Kid != ""
		pb.Streams = append(pb.Streams, model.Stream{
			URL: representation.MainURL, BackupURL: representation.BackupURL,
			Definition: representation.Definition, Codec: representation.Codec,
			Width: representation.Width, Height: representation.Height,
			Bitrate: representation.Bitrate, Size: representation.Size,
			Protected: protected, Encryption: encryptionLabel(protected),
		})
		if protected {
			pb.KeyMaterial = representation.EncryptInfo.SpadeA
			pb.Protected = true
		}
	}
	if len(pb.Streams) == 0 {
		return nil, errPlaybackModelUnavailable
	}
	return pb, nil
}

func encryptionLabel(protected bool) string {
	if protected {
		return "cenc-aes-ctr"
	}
	return ""
}
