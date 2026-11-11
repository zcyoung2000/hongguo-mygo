package upstream

import (
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type Family string

const (
	FamilyPlayback Family = "playback"
	FamilyComment  Family = "comment"
)

const (
	DefaultSuccessGain      = 1
	DefaultFailurePenalty   = 4
	DefaultMaxHealth        = 24
	DefaultMinHealth        = -24
	DefaultBadThreshold     = -4
	DefaultCooldown         = 10 * time.Minute
	DefaultHalfOpenInterval = 30 * time.Second
	DefaultStaleAge         = 24 * time.Hour
)

var (
	DefaultPlaybackHosts = []string{
		"api5-normal-lq.fqnovel.com",
		"api3-normal-lq.fqnovel.com",
		"api5-normal-hl.fqnovel.com",
		"api3-normal-hl.fqnovel.com",
		"api5-normal-lf.fqnovel.com",
		"api3-normal-lf.fqnovel.com",
	}
	DefaultCommentHosts = []string{
		"api5-normal-sinfonlinea.fqnovel.com",
		"api5-normal-sinfonlineb.fqnovel.com",
		"api5-normal-sinfonlinec.fqnovel.com",
	}
)

type HostState struct {
	Family              Family `json:"family"`
	Host                string `json:"host"`
	Health              int    `json:"health"`
	ConsecutiveFailures int    `json:"consecutiveFailures"`
	LastFailureAt       int64  `json:"lastFailureAt"`
	LastSuccessAt       int64  `json:"lastSuccessAt"`
}

type StateStore interface {
	LoadHostStates() ([]HostState, error)
	SaveHostStates([]HostState) error
}

type HostPicker struct {
	mu               sync.Mutex
	families         map[Family][]string
	states           map[Family]map[string]*HostState
	store            StateStore
	now              func() time.Time
	successGain      int
	failurePenalty   int
	maxHealth        int
	minHealth        int
	badThreshold     int
	cooldown         time.Duration
	halfOpenInterval time.Duration
	staleAge         time.Duration
}

func NewHostPicker(store StateStore) *HostPicker {
	return &HostPicker{
		families: map[Family][]string{
			FamilyPlayback: append([]string(nil), DefaultPlaybackHosts...),
			FamilyComment:  append([]string(nil), DefaultCommentHosts...),
		},
		states: map[Family]map[string]*HostState{
			FamilyPlayback: {},
			FamilyComment:  {},
		},
		store:            store,
		now:              time.Now,
		successGain:      DefaultSuccessGain,
		failurePenalty:   DefaultFailurePenalty,
		maxHealth:        DefaultMaxHealth,
		minHealth:        DefaultMinHealth,
		badThreshold:     DefaultBadThreshold,
		cooldown:         DefaultCooldown,
		halfOpenInterval: DefaultHalfOpenInterval,
		staleAge:         DefaultStaleAge,
	}
}

func (p *HostPicker) Configure(successGain, failurePenalty, maxHealth, minHealth, badThreshold int, cooldown, halfOpenInterval, staleAge time.Duration) {
	if successGain > 0 {
		p.successGain = successGain
	}
	if failurePenalty > 0 {
		p.failurePenalty = failurePenalty
	}
	if maxHealth > 0 {
		p.maxHealth = maxHealth
	}
	if minHealth < 0 {
		p.minHealth = minHealth
	}
	if badThreshold < 0 {
		p.badThreshold = badThreshold
	}
	if cooldown > 0 {
		p.cooldown = cooldown
	}
	if halfOpenInterval > 0 {
		p.halfOpenInterval = halfOpenInterval
	}
	if staleAge > 0 {
		p.staleAge = staleAge
	}
}

func StaleAgeFromEnvironment() time.Duration {
	value := strings.TrimSpace(os.Getenv("MYGUO_MULTI_HOST_STALE_AGE"))
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return DefaultStaleAge
	}
	return parsed
}

func (p *HostPicker) SetNow(now func() time.Time) {
	if now != nil {
		p.now = now
	}
}

func (p *HostPicker) Restore() error {
	if p.store == nil {
		return nil
	}
	states, err := p.store.LoadHostStates()
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, state := range states {
		if !containsHost(p.families[state.Family], state.Host) {
			continue
		}
		normalized := state
		normalized.Health = clamp(normalized.Health, p.minHealth, p.maxHealth)
		normalized = p.decayStaleLocked(normalized, p.now())
		p.states[state.Family][state.Host] = &normalized
	}
	return nil
}

func (p *HostPicker) Order(family Family) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	hosts := append([]string(nil), p.families[family]...)
	now := p.now()
	sort.SliceStable(hosts, func(i, j int) bool {
		left, right := p.stateLocked(family, hosts[i]), p.stateLocked(family, hosts[j])
		return p.rankLocked(left, now) > p.rankLocked(right, now)
	})
	return hosts
}

func (p *HostPicker) MarkGood(family Family, host string) {
	p.mu.Lock()
	if !containsHost(p.families[family], host) {
		p.mu.Unlock()
		return
	}
	state := p.stateLocked(family, host)
	state.Health = clamp(state.Health+p.successGain, p.minHealth, p.maxHealth)
	state.ConsecutiveFailures = 0
	state.LastSuccessAt = p.now().Unix()
	p.mu.Unlock()
	p.persist()
}

func (p *HostPicker) MarkBad(family Family, host string) {
	p.mu.Lock()
	if !containsHost(p.families[family], host) {
		p.mu.Unlock()
		return
	}
	state := p.stateLocked(family, host)
	state.Health = clamp(state.Health-p.failurePenalty, p.minHealth, p.maxHealth)
	state.ConsecutiveFailures++
	state.LastFailureAt = p.now().Unix()
	p.mu.Unlock()
	p.persist()
}

func (p *HostPicker) States() []HostState {
	p.mu.Lock()
	defer p.mu.Unlock()
	var states []HostState
	for family := range p.families {
		for _, host := range p.families[family] {
			states = append(states, *p.stateLocked(family, host))
		}
	}
	return states
}

func (p *HostPicker) stateLocked(family Family, host string) *HostState {
	if !containsHost(p.families[family], host) {
		host = firstHost(p.families[family])
	}
	state := p.states[family][host]
	if state == nil {
		state = &HostState{Family: family, Host: host}
		p.states[family][host] = state
	}
	return state
}

func (p *HostPicker) rankLocked(state *HostState, now time.Time) int {
	if state.ConsecutiveFailures == 0 || state.Health > p.badThreshold {
		return state.Health
	}
	elapsed := now.Sub(time.Unix(state.LastFailureAt, 0))
	if elapsed >= p.cooldown+p.halfOpenInterval {
		return state.Health + 1000
	}
	return state.Health - 1000
}

func (p *HostPicker) decayStaleLocked(state HostState, now time.Time) HostState {
	last := state.LastSuccessAt
	if state.LastFailureAt > last {
		last = state.LastFailureAt
	}
	if last == 0 || now.Sub(time.Unix(last, 0)) < p.staleAge {
		return state
	}
	state.ConsecutiveFailures = 0
	if state.Health < 0 {
		state.Health = 0
	}
	return state
}

func (p *HostPicker) persist() {
	if p.store == nil {
		return
	}
	_ = p.store.SaveHostStates(p.States())
}

func HostOf(rawURL string) string {
	start := strings.Index(rawURL, "://")
	if start < 0 {
		start = 0
	} else {
		start += 3
	}
	rest := rawURL[start:]
	for index := 0; index < len(rest); index++ {
		if rest[index] == '/' || rest[index] == '?' || rest[index] == '#' {
			return rest[:index]
		}
	}
	return rest
}

func FamilyForPath(path string) Family {
	if strings.HasPrefix(path, "/novel/commentapi/") {
		return FamilyComment
	}
	return FamilyPlayback
}

func containsHost(hosts []string, host string) bool {
	for _, item := range hosts {
		if item == host {
			return true
		}
	}
	return false
}

func firstHost(hosts []string) string {
	if len(hosts) == 0 {
		return ""
	}
	return hosts[0]
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
