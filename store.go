package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ---------- persisted data ----------

type Settings struct {
	PassSalt  string `json:"pass_salt"`
	PassHash  string `json:"pass_hash"`
	APIToken  string `json:"api_token"`
	CreatedAt int64  `json:"created_at"`
}

type Image struct {
	ID    string `json:"id"`
	File  string `json:"file"` // relative path under uploads/, e.g. 2026/10/14/abc.png
	Orig  string `json:"orig"` // original filename as uploaded (display only)
	Size  int64  `json:"size"`
	Ext   string `json:"ext"`
	Owner string `json:"owner"` // "admin" | "guest"
	IP    string `json:"ip"`
	TS    int64  `json:"ts"` // unix seconds
}

type indexFile struct {
	Images []Image `json:"images"`
}

// counters: { "2026-10-14": {"1.2.3.4": 3}, "2026-10-14T15": {"1.2.3.4": 2} }
type counterFile map[string]map[string]int

type Store struct {
	dir string
	mu  sync.Mutex

	settings *Settings
	index    *indexFile
	counters counterFile
}

func NewStore(dir string) (*Store, error) {
	s := &Store{dir: dir, index: &indexFile{}, counters: counterFile{}}
	if b, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
		var st Settings
		if err := json.Unmarshal(b, &st); err == nil {
			s.settings = &st
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "index.json")); err == nil {
		_ = json.Unmarshal(b, s.index)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "counters.json")); err == nil {
		_ = json.Unmarshal(b, &s.counters)
	}
	return s, nil
}

func (s *Store) FirstRun() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings == nil || s.settings.PassHash == ""
}

func writeJSONAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) saveSettings() error {
	if s.settings == nil {
		return fmt.Errorf("no settings")
	}
	return writeJSONAtomic(filepath.Join(s.dir, "settings.json"), s.settings)
}
func (s *Store) saveIndex() error {
	return writeJSONAtomic(filepath.Join(s.dir, "index.json"), s.index)
}
func (s *Store) saveCounters() error {
	return writeJSONAtomic(filepath.Join(s.dir, "counters.json"), s.counters)
}

func (s *Store) GetSettings() *Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *Store) UpdateSettings(fn func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settings == nil {
		s.settings = &Settings{CreatedAt: time.Now().Unix()}
	}
	fn(s.settings)
	return s.saveSettings()
}

// ---------- images ----------

func (s *Store) AddImage(img Image) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.index.Images = append(s.index.Images, img)
	return s.saveIndex()
}

func (s *Store) ListImages() []Image {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Image, len(s.index.Images))
	copy(out, s.index.Images)
	sort.Slice(out, func(i, j int) bool { return out[i].TS > out[j].TS })
	return out
}

func (s *Store) RemoveImage(id string) (Image, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, im := range s.index.Images {
		if im.ID == id {
			s.index.Images = append(s.index.Images[:i], s.index.Images[i+1:]...)
			if err := s.saveIndex(); err != nil {
				return Image{}, false, err
			}
			return im, true, nil
		}
	}
	return Image{}, false, nil
}

// RemoveByOwner removes every image of the given owner; returns removed entries.
func (s *Store) RemoveByOwner(owner string) ([]Image, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed, kept []Image
	for _, im := range s.index.Images {
		if im.Owner == owner {
			removed = append(removed, im)
		} else {
			kept = append(kept, im)
		}
	}
	s.index.Images = kept
	if err := s.saveIndex(); err != nil {
		return nil, err
	}
	return removed, nil
}

func (s *Store) Totals() (totalBytes, guestBytes int64, totalCount, guestCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, im := range s.index.Images {
		totalBytes += im.Size
		totalCount++
		if im.Owner == "guest" {
			guestBytes += im.Size
			guestCount++
		}
	}
	return
}

func (s *Store) TodayCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	start := time.Now().Truncate(24 * time.Hour).Unix()
	n := 0
	for _, im := range s.index.Images {
		if im.TS >= start {
			n++
		}
	}
	return n
}

// ---------- guest counters ----------

func dayKey() string  { return time.Now().Format("2006-01-02") }
func hourKey() string { return time.Now().Format("2006-01-02T15") }

// GuestCount returns today's and this hour's upload count for an IP (no increment).
func (s *Store) GuestCount(ip string) (day, hour int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.counters[dayKey()]; m != nil {
		day = m[ip]
	}
	if m := s.counters[hourKey()]; m != nil {
		hour = m[ip]
	}
	return
}

func (s *Store) GuestInc(ip string, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dk, hk := dayKey(), hourKey()
	// prune keys older than 2 days / 2 hours lazily
	for k := range s.counters {
		if len(k) == 10 && k < time.Now().AddDate(0, 0, -2).Format("2006-01-02") {
			delete(s.counters, k)
		}
		if len(k) == 13 && k < time.Now().Add(-2*time.Hour).Format("2006-01-02T15") {
			delete(s.counters, k)
		}
	}
	if s.counters[dk] == nil {
		s.counters[dk] = map[string]int{}
	}
	if s.counters[hk] == nil {
		s.counters[hk] = map[string]int{}
	}
	s.counters[dk][ip] += n
	s.counters[hk][ip] += n
	_ = s.saveCounters()
}

// ---------- random ----------

const randAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func randStr(n int) string {
	b := make([]byte, n)
	max := big.NewInt(int64(len(randAlphabet)))
	for i := range b {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err)
		}
		b[i] = randAlphabet[v.Int64()]
	}
	return string(b)
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	const hexd = "0123456789abcdef"
	out := make([]byte, n*2)
	for i, v := range b {
		out[i*2] = hexd[v>>4]
		out[i*2+1] = hexd[v&0xf]
	}
	return string(out)
}
