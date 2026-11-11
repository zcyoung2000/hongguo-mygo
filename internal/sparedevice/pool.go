package sparedevice

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"myguo/internal/store"
)

const (
	DefaultBorrowCooldown   = 15 * time.Minute
	DefaultFailureThreshold = 3

	DefaultRegistrationDailyLimit  = 1
	DefaultRegistrationMinInterval = 24 * time.Hour
)

var ErrNoDeviceAvailable = errors.New("no spare device available")

type PlaybackResolver func(ctx context.Context, episodeID string, options sdk.PlaybackOptions) (*model.Playback, error)

type ClientFactory func(profile sdk.ProfileStore) (PlaybackResolver, error)

type ClientConstructor func(sdk.Options) (*sdk.Client, error)

type Manager struct {
	store            *store.Store
	mu               sync.Mutex
	borrowed         map[string]string
	factory          ClientFactory
	cooldown         time.Duration
	failureThreshold int
	now              func() time.Time
	logger           *log.Logger
}

type Option func(*Manager)

func WithClientConstructor(constructor ClientConstructor, transport sdk.Transport) Option {
	return func(m *Manager) {
		m.factory = func(profile sdk.ProfileStore) (PlaybackResolver, error) {
			client, err := constructor(sdk.Options{Transport: transport, ProfileStore: profile})
			if err != nil {
				return nil, err
			}
			return client.ResolvePlayback, nil
		}
	}
}

func WithClientFactory(factory ClientFactory) Option {
	return func(m *Manager) { m.factory = factory }
}

func WithCooldown(cooldown time.Duration) Option {
	return func(m *Manager) {
		if cooldown > 0 {
			m.cooldown = cooldown
		}
	}
}

func WithFailureThreshold(threshold int) Option {
	return func(m *Manager) {
		if threshold > 0 {
			m.failureThreshold = threshold
		}
	}
}

func WithClock(clock func() time.Time) Option {
	return func(m *Manager) {
		if clock != nil {
			m.now = clock
		}
	}
}

func WithLogger(logger *log.Logger) Option {
	return func(m *Manager) {
		if logger != nil {
			m.logger = logger
		}
	}
}

func NewManager(state *store.Store, opts ...Option) *Manager {
	manager := &Manager{
		store:            state,
		borrowed:         make(map[string]string),
		cooldown:         DefaultBorrowCooldown,
		failureThreshold: DefaultFailureThreshold,
		now:              time.Now,
		logger:           log.Default(),
	}
	for _, opt := range opts {
		opt(manager)
	}
	return manager
}

type profileStore struct {
	device store.SpareDevice
	now    func() time.Time
}

func (p profileStore) Load(context.Context) (*sdk.DeviceProfile, error) {
	profile := sdk.DeviceProfile{
		CDID:      p.device.CDID,
		OpenUDID:  p.device.OpenUDID,
		DeviceID:  p.device.DeviceID,
		InstallID: p.device.InstallID,
		CreatedAt: time.Unix(p.device.CreatedAt, 0).UTC(),
	}
	if p.device.CreatedAt == 0 {
		profile.CreatedAt = p.now().UTC()
	}
	if profile.CDID == "" {
		profile.CDID = deterministicID("cdid", p.device.DeviceID)
	}
	if profile.OpenUDID == "" {
		profile.OpenUDID = deterministicID("openudid", p.device.DeviceID)
	}
	profile.Device = &model.Device{}
	return &profile, nil
}

func (p profileStore) Save(context.Context, *sdk.DeviceProfile) error { return nil }
func (p profileStore) Reset(context.Context) error                    { return nil }

func deterministicID(prefix, deviceID string) string {
	var raw [16]byte
	copy(raw[:], prefix+":"+deviceID)
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	value := hex.EncodeToString(raw[:])
	return value[0:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:32]
}

func IsSwitchablePlaybackFailure(err error) bool {
	return errors.Is(err, sdk.ErrAnonymousLimited) ||
		errors.Is(err, sdk.ErrRateLimited) ||
		errors.Is(err, context.DeadlineExceeded) ||
		(err != nil && strings.Contains(strings.ToLower(err.Error()), "empty playback"))
}

func (m *Manager) ResolveForSession(ctx context.Context, session, episodeID string, cause error, primary PlaybackResolver) (*model.Playback, error) {
	if session == "" || episodeID == "" || primary == nil {
		return nil, fmt.Errorf("spare playback resolver requires session, episode, and primary resolver")
	}
	if !IsSwitchablePlaybackFailure(cause) {
		return nil, cause
	}
	m.mu.Lock()
	deviceID := m.borrowed[session]
	m.mu.Unlock()

	if deviceID != "" {
		if resolver, device, err := m.resolverForDevice(deviceID); err == nil {
			playback, resolveErr := resolver(ctx, episodeID, sdk.PlaybackOptions{})
			m.note(session, episodeID, cause, device.DeviceID, resolveErr)
			if resolveErr == nil {
				return playback, nil
			}
			m.releaseWithFailure(session, device.DeviceID, resolveErr)
		}
	}

	device, err := m.borrow(session, episodeID, cause)
	if err != nil {
		return nil, err
	}
	resolver, err := m.resolver(device)
	if err != nil {
		m.releaseWithFailure(session, device.DeviceID, err)
		return nil, err
	}
	playback, resolveErr := resolver(ctx, episodeID, sdk.PlaybackOptions{})
	m.note(session, episodeID, cause, device.DeviceID, resolveErr)
	if resolveErr != nil {
		m.releaseWithFailure(session, device.DeviceID, resolveErr)
		return nil, resolveErr
	}
	return playback, nil
}

func (m *Manager) resolverForDevice(deviceID string) (PlaybackResolver, store.SpareDevice, error) {
	devices, err := m.store.SpareDevices()
	if err != nil {
		return nil, store.SpareDevice{}, err
	}
	for _, device := range devices {
		if device.DeviceID == deviceID {
			resolver, err := m.resolver(device)
			return resolver, device, err
		}
	}
	return nil, store.SpareDevice{}, fmt.Errorf("spare device %q is no longer in pool", deviceID)
}

func (m *Manager) resolver(device store.SpareDevice) (PlaybackResolver, error) {
	if m.factory == nil {
		return nil, fmt.Errorf("spare device client factory is unavailable")
	}
	return m.factory(profileStore{device: device, now: m.now})
}

func (m *Manager) borrow(session, episodeID string, cause error) (store.SpareDevice, error) {
	m.mu.Lock()
	if existing := m.borrowed[session]; existing != "" {
		m.mu.Unlock()
		return store.SpareDevice{}, fmt.Errorf("spare playback session already borrows %q", existing)
	}
	m.mu.Unlock()
	causeText := ""
	if cause != nil {
		causeText = cause.Error()
	}
	device, err := m.store.BorrowAvailableSpareDevice(
		session, episodeID, causeText, m.failureThreshold, m.cooldown, m.now(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return store.SpareDevice{}, ErrNoDeviceAvailable
	}
	if err != nil {
		return store.SpareDevice{}, err
	}
	m.mu.Lock()
	m.borrowed[session] = device.DeviceID
	m.mu.Unlock()
	m.audit(session, episodeID, device.DeviceID, cause, "borrow", "")
	return device, nil
}

func (m *Manager) note(session, episodeID string, cause error, deviceID string, resolveErr error) {
	if resolveErr == nil {
		m.audit(session, episodeID, deviceID, cause, "success", "")
	}
}

func (m *Manager) ReleaseSession(session string) {
	if session == "" {
		return
	}
	m.mu.Lock()
	deviceID := m.borrowed[session]
	delete(m.borrowed, session)
	m.mu.Unlock()
	if deviceID == "" {
		return
	}
	_ = m.store.CompleteSpareDeviceBorrow(session, deviceID, false,
		m.failureThreshold, m.cooldown, m.now())
	m.audit(session, "", deviceID, nil, "success", "")
}

func (m *Manager) releaseWithFailure(session, deviceID string, cause error) {
	_ = m.store.CompleteSpareDeviceBorrow(session, deviceID, true,
		m.failureThreshold, m.cooldown, m.now())
	m.audit(session, "", deviceID, cause, "failure", cause.Error())
}

func (m *Manager) audit(session, episodeID, deviceID string, cause error, event, detail string) {
	causeText := ""
	if cause != nil {
		causeText = cause.Error()
	}
	_ = m.store.AppendSpareDeviceEvent(store.SpareDeviceBorrowEvent{
		Session: session, DeviceID: deviceID, EpisodeID: episodeID,
		Cause: causeText, Event: event, Detail: detail, At: m.now().Unix(),
	})
	m.logger.Printf("[spare-device] session=%s device=%s event=%s episode=%s", session, deviceID, event, episodeID)
}

func (m *Manager) Import(device store.SpareDevice) error {
	device.Status = store.SpareDeviceStatusAvailable
	device.FailureCount = 0
	device.CooldownUntil = 0
	return m.store.UpsertSpareDevice(device)
}

func (m *Manager) Remove(deviceID string) error {
	return m.store.DeleteSpareDevice(deviceID)
}

func (m *Manager) Devices() ([]store.SpareDevice, error) { return m.store.SpareDevices() }

func (m *Manager) SetRegistrationEnabled(enabled bool) error {
	return m.store.SetSpareDeviceRegistrationEnabled(enabled)
}

func (m *Manager) Settings() (store.SpareDeviceSettings, error) {
	return m.store.GetSpareDeviceSettings()
}

func (m *Manager) RegistrationAllowed() (bool, error) {
	_, allowed, err := m.store.SpareDeviceRegistrationState(m.now())
	return allowed, err
}

func RandomSession() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("pb-%d", time.Now().UnixNano())
	}
	return "pb-" + base64.RawURLEncoding.EncodeToString(raw[:])
}
