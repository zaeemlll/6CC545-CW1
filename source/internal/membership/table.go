package membership

import (
	"errors"
	"net"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"

	"example.edu/6cc545/cw1/internal/config"
)

var ErrNotImplemented = errors.New("TODO W10: membership not implemented")
var ErrReplay = errors.New("stale incarnation or sequence")
var ErrIdentity = errors.New("advertised identity differs from authenticated transport")
var ErrCapacity = errors.New("peer watermark capacity reached")

type Announcement struct {
	ID          string `json:"id"`
	Address     string `json:"address"`
	Incarnation uint64 `json:"incarnation"`
	Sequence    uint64 `json:"sequence"`
}

type Peer struct {
	Announcement
	LastSeen time.Time `json:"-"`
}

type Table struct {
	mu               sync.RWMutex
	max              int
	ttl              time.Duration
	peers            map[string]Peer
	watermarks       map[string][2]uint64
	localIncarnation uint64
	statePath        string
}

func New(max int, ttl time.Duration) (*Table, error) {
	if max < 1 || max > 128 || ttl <= 0 {
		return nil, errors.New("max 1..128 and positive TTL required")
	}

	return &Table{
		max:        max,
		ttl:        ttl,
		peers:      make(map[string]Peer),
		watermarks: make(map[string][2]uint64),
	}, nil
}

// NewPersistent restores replay watermarks before accepting announcements.
func NewPersistent(max int, ttl time.Duration, statePath string) (*Table, error) {
	if statePath == "" {
		return nil, errors.New("membership state path is required")
	}

	path, err := prepareStateDirectory(statePath)
	if err != nil {
		return nil, err
	}

	table, err := New(max, ttl)
	if err != nil {
		return nil, err
	}

	var localIncarnation uint64
	saved, err := loadWatermarks(path, &localIncarnation)
	if err != nil {
		return nil, err
	}
	if err := table.Restore(saved); err != nil {
		return nil, err
	}

	if localIncarnation == ^uint64(0) {
		return nil, errors.New("local membership incarnation exhausted")
	}
	localIncarnation++

	if err := saveWatermarks(path, table.watermarks, localIncarnation); err != nil {
		return nil, err
	}

	table.localIncarnation = localIncarnation
	table.statePath = path
	return table, nil
}

func validAddress(address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Host == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Path != "" && u.Path != "/") {
		return false
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || host == "" || port == "" {
		return false
	}

	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

func (t *Table) Accept(a Announcement, authenticatedID string, now time.Time) error {
	if !config.ValidID(a.ID) || !config.ValidID(authenticatedID) ||
		a.ID != authenticatedID {
		return ErrIdentity
	}

	if !validAddress(a.Address) || a.Incarnation == 0 || a.Sequence == 0 {
		return errors.New("invalid membership announcement")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	previous, known := t.watermarks[a.ID]
	if known {
		if a.Incarnation < previous[0] ||
			(a.Incarnation == previous[0] && a.Sequence <= previous[1]) {
			return ErrReplay
		}
	} else if len(t.watermarks) >= 128 {
		return ErrCapacity
	}

	if _, active := t.peers[a.ID]; !active && len(t.peers) >= t.max {
		return ErrCapacity
	}

	nextWatermarks := make(map[string][2]uint64, len(t.watermarks)+1)
	for id, watermark := range t.watermarks {
		nextWatermarks[id] = watermark
	}
	nextWatermarks[a.ID] = [2]uint64{a.Incarnation, a.Sequence}

	// Persist replay protection before acknowledging the announcement.
	if t.statePath != "" {
		if err := saveWatermarks(t.statePath, nextWatermarks, t.localIncarnation); err != nil {
			return err
		}
	}

	t.watermarks = nextWatermarks
	t.peers[a.ID] = Peer{
		Announcement: a,
		LastSeen:     now,
	}

	return nil
}

func (t *Table) Expire(now time.Time) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var expired []string

	for id, peer := range t.peers {
		if now.Sub(peer.LastSeen) >= t.ttl {
			delete(t.peers, id)
			expired = append(expired, id)
		}
	}

	sort.Strings(expired)
	return expired
}

// ActiveSnapshot returns peers whose announcements have not expired.
func (t *Table) ActiveSnapshot(now time.Time) []Peer {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]Peer, 0, len(t.peers))
	for _, peer := range t.peers {
		if now.Sub(peer.LastSeen) < t.ttl {
			result = append(result, peer)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

func (t *Table) Snapshot() []Peer {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]Peer, 0, len(t.peers))
	for _, peer := range t.peers {
		result = append(result, peer)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

func (t *Table) Watermarks() map[string][2]uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string][2]uint64, len(t.watermarks))
	for id, watermark := range t.watermarks {
		result[id] = watermark
	}

	return result
}

func (t *Table) Restore(saved map[string][2]uint64) error {
	if len(saved) > 128 {
		return ErrCapacity
	}

	restored := make(map[string][2]uint64, len(saved))

	for id, watermark := range saved {
		if !config.ValidID(id) || watermark[0] == 0 || watermark[1] == 0 {
			return errors.New("invalid saved membership watermark")
		}
		restored[id] = watermark
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.peers) != 0 {
		return errors.New("cannot restore membership while peers are active")
	}

	t.watermarks = restored
	return nil
}

// Incarnation returns this node's persisted local incarnation.
func (t *Table) Incarnation() uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.localIncarnation
}
