package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/sign"
)

const (
	// The reverse report proves the endpoint but not its server-side ceiling.
	// Two is the conservative requested window; it must be verified before
	// being raised.
	multiVideoModelBatchSize = 2
	// multiVideoModelWindow is the i+1/i+2 lookahead requested while an episode
	// plays. It deliberately matches the conservative batch above.
	multiVideoModelWindow      = 2
	multiVideoModelPath        = "/novel/player/multi_video_model/v1/"
	multiVideoModelOrigin      = "https://api5-normal-lq.fqnovel.com"
	multiVideoModelHTTPTimeout = 8 * time.Second
	maxPlaybackResponseBytes   = int64(8 * 1024 * 1024)
	sdkVersionName             = "7.3.9.32"
)

type MultiVideoModelRequest struct {
	VideoID  []string       `json:"video_id"`
	BizParam map[string]any `json:"biz_param"`
}

type multiVideoModelResponse struct {
	Code    json.RawMessage            `json:"code"`
	Message string                     `json:"message"`
	Data    map[string]json.RawMessage `json:"data"`
}

type multiModelTask struct {
	episodes []model.Episode
}

type multiModelLoad struct {
	done      chan struct{}
	setCancel chan context.CancelFunc
}

type multiModelWindow struct {
	tasks []multiModelTask
}

// MultiVideoModelBatcher is isolated from media-file prefetching. Its HTTP work
// only fills the existing playback-model cache, so normal playback and model
// prefetch share deduplication and expiry instead of creating a second cache.
type MultiVideoModelBatcher struct {
	client             *sdk.Client
	httpClient         *http.Client
	multiVideoModelURL string
	deviceIdentity     func() (deviceID, installID, cdid, openudid string)

	mu       sync.Mutex
	loads    map[string]*multiModelLoad
	window   *multiModelWindow
	windowID uint64
}

func NewMultiVideoModelBatcher(client *sdk.Client, httpClient *http.Client) *MultiVideoModelBatcher {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: multiVideoModelHTTPTimeout}
	}
	return &MultiVideoModelBatcher{
		client: client, httpClient: httpClient, multiVideoModelURL: multiVideoModelOrigin,
		loads: make(map[string]*multiModelLoad),
	}
}

// NextEpisodes returns at most the following two playable episodes, preserving
// episode order and excluding duplicates.
func NextEpisodes(episodes []model.Episode, currentNumber int) []model.Episode {
	if len(episodes) == 0 || multiVideoModelWindow <= 0 {
		return nil
	}
	ordered := append([]model.Episode(nil), episodes...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Number < ordered[j].Number })
	seen := make(map[int]bool, len(ordered))
	out := make([]model.Episode, 0, multiVideoModelWindow)
	for _, episode := range ordered {
		if episode.Number <= currentNumber || !episode.Playable || strings.TrimSpace(episode.ID) == "" {
			continue
		}
		if seen[episode.Number] {
			continue
		}
		seen[episode.Number] = true
		out = append(out, episode)
		if len(out) == multiVideoModelWindow {
			break
		}
	}
	return out
}

func multiVideoModelBody(episodes []model.Episode) ([]byte, error) {
	if len(episodes) == 0 || len(episodes) > multiVideoModelBatchSize {
		return nil, fmt.Errorf("multi video model batch must contain 1..%d episodes", multiVideoModelBatchSize)
	}
	ids := make([]string, 0, len(episodes))
	for _, episode := range episodes {
		ids = append(ids, episode.ID)
	}
	return json.Marshal(MultiVideoModelRequest{
		VideoID: ids,
		BizParam: map[string]any{
			"video_id_type":             0,
			"need_all_video_definition": true,
			"video_platform":            3,
		},
	})
}

func decodeMultiVideoModel(body []byte, episodeID string) (*model.Playback, error) {
	var decoded model.Playback
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("multi video model %s is not JSON: %w", episodeID, err)
	}
	decoded.EpisodeID = episodeID
	if len(decoded.Streams) == 0 {
		return nil, fmt.Errorf("multi video model %s has no stream", episodeID)
	}
	return &decoded, nil
}

func parseMultiVideoModels(body []byte, episodes []model.Episode) (map[string]*model.Playback, error) {
	var response multiVideoModelResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	code := strings.Trim(strings.TrimSpace(string(response.Code)), `"`)
	if code != "" && code != "0" {
		return nil, fmt.Errorf("multi video model code %s", code)
	}
	out := make(map[string]*model.Playback, len(episodes))
	for _, episode := range episodes {
		raw := response.Data[episode.ID]
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		playback, err := decodeMultiVideoModel(raw, episode.ID)
		if err != nil {
			return nil, err
		}
		out[episode.ID] = playback
	}
	if len(out) == 0 {
		return nil, errors.New("multi video model response has no models")
	}
	return out, nil
}

func (b *MultiVideoModelBatcher) setDeviceIdentity(identity func() (deviceID, installID, cdid, openudid string)) {
	b.mu.Lock()
	b.deviceIdentity = identity
	b.mu.Unlock()
}

func (b *MultiVideoModelBatcher) forgetWindow() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.windowID++
	b.window = nil
	for _, load := range b.loads {
		if cancel, ok := <-load.setCancel; ok {
			cancel()
		}
		select {
		case <-load.done:
		default:
			close(load.done)
		}
	}
	return b.windowID
}

func (b *MultiVideoModelBatcher) acquireLoads(episodes []model.Episode) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.loads == nil {
		b.loads = make(map[string]*multiModelLoad)
	}
	for _, episode := range episodes {
		if _, active := b.loads[episode.ID]; active {
			return false
		}
	}
	for _, episode := range episodes {
		load := &multiModelLoad{done: make(chan struct{}), setCancel: make(chan context.CancelFunc, 1)}
		b.loads[episode.ID] = load
	}
	return true
}

func (b *MultiVideoModelBatcher) releaseLoads(loadIDs []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, id := range loadIDs {
		if load := b.loads[id]; load != nil {
			select {
			case cancel := <-load.setCancel:
				cancel()
			default:
			}
			select {
			case <-load.done:
			default:
				close(load.done)
			}
			delete(b.loads, id)
		}
	}
}

func (b *MultiVideoModelBatcher) ReplaceWindow(episodes []model.Episode) uint64 {
	window := multiModelWindow{tasks: make([]multiModelTask, 0, (len(episodes)+multiVideoModelBatchSize-1)/multiVideoModelBatchSize)}
	for start := 0; start < len(episodes); start += multiVideoModelBatchSize {
		end := min(start+multiVideoModelBatchSize, len(episodes))
		window.tasks = append(window.tasks, multiModelTask{episodes: append([]model.Episode(nil), episodes[start:end]...)})
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.windowID++
	b.window = &window
	for _, load := range b.loads {
		select {
		case cancel := <-load.setCancel:
			cancel()
		default:
		}
		select {
		case <-load.done:
		default:
			close(load.done)
		}
	}
	return b.windowID
}

func (b *MultiVideoModelBatcher) snapshotWindow() (uint64, []multiModelTask, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.window == nil {
		return 0, nil, false
	}
	return b.windowID, b.window.tasks, true
}

func (b *MultiVideoModelBatcher) windowUnchanged(id uint64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.windowID == id
}

func (b *MultiVideoModelBatcher) Run(ctx context.Context, remember func(string, *model.Playback)) {
	if b == nil || b.client == nil {
		return
	}
	windowID, tasks, ok := b.snapshotWindow()
	if !ok {
		return
	}
	for _, task := range tasks {
		if ctx.Err() != nil || !b.windowUnchanged(windowID) {
			return
		}
		b.runTask(ctx, task, remember, windowID)
	}
}

func (b *MultiVideoModelBatcher) runTask(ctx context.Context, task multiModelTask, remember func(string, *model.Playback), windowID uint64) {
	if !b.acquireLoads(task.episodes) {
		return
	}
	loadIDs := make([]string, 0, len(task.episodes))
	for _, episode := range task.episodes {
		loadIDs = append(loadIDs, episode.ID)
	}
	defer b.releaseLoads(loadIDs)
	loads := make([]*multiModelLoad, 0, len(task.episodes))
	b.mu.Lock()
	for _, episode := range task.episodes {
		if load := b.loads[episode.ID]; load != nil {
			loads = append(loads, load)
		}
	}
	b.mu.Unlock()
	fetchCtx, fetchCancel := context.WithCancel(ctx)
	defer fetchCancel()
	for _, load := range loads {
		select {
		case <-load.done:
			fetchCancel()
			return
		case load.setCancel <- fetchCancel:
		}
		defer func() {
			select {
			case <-load.done:
			default:
				close(load.done)
			}
		}()
	}

	playbacks, err := b.fetch(fetchCtx, task.episodes)
	if err != nil || ctx.Err() != nil || !b.windowUnchanged(windowID) {
		return
	}
	for _, episode := range task.episodes {
		if playback := playbacks[episode.ID]; playback != nil {
			remember(episode.ID, playback)
		}
	}
}

func (b *MultiVideoModelBatcher) fetch(ctx context.Context, episodes []model.Episode) (map[string]*model.Playback, error) {
	body, err := multiVideoModelBody(episodes)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	identity := b.deviceIdentity
	b.mu.Unlock()
	var deviceID, installID, cdid, openudid string
	if identity != nil {
		deviceID, installID, cdid, openudid = identity()
	}
	ticket := time.Now().UnixMilli()
	requestURL := b.multiVideoModelURL
	if requestURL == "" {
		requestURL = multiVideoModelOrigin
	}
	query, headers := sign.SignPOST(multiVideoModelPath, defaultPlaybackDeviceQuery(deviceID, installID, cdid, openudid), ticket, deviceID, sdkVersionName, body, nil)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL+multiVideoModelPath+"?"+query, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	setPlaybackHeaders(request.Header, ticket)
	response, err := b.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("multi video model API returned %d", response.StatusCode)
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxPlaybackResponseBytes))
	if err != nil {
		return nil, err
	}
	return parseMultiVideoModels(responseBody, episodes)
}
