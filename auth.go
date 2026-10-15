package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ---------- auth: single admin, session cookies + API token ----------

type Auth struct {
	store    *Store
	mu       sync.Mutex
	sessions map[string]time.Time // token -> expiry
	logins   map[string][]time.Time
}

func NewAuth(s *Store) *Auth {
	return &Auth{store: s, sessions: map[string]time.Time{}, logins: map[string][]time.Time{}}
}

func hashPassword(pass, saltB64 string) string {
	salt, _ := base64.StdEncoding.DecodeString(saltB64)
	key, err := pbkdf2.Key(sha256.New, pass, salt, 210000, 32)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func (a *Auth) SetPassword(pass string) error {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	saltB64 := base64.StdEncoding.EncodeToString(salt)
	h := hashPassword(pass, saltB64)
	return a.store.UpdateSettings(func(s *Settings) {
		s.PassSalt = saltB64
		s.PassHash = h
	})
}

func (a *Auth) CheckPassword(pass string) bool {
	st := a.store.GetSettings()
	if st == nil || st.PassHash == "" {
		return false
	}
	h := hashPassword(pass, st.PassSalt)
	return subtle.ConstantTimeCompare([]byte(h), []byte(st.PassHash)) == 1
}

func (a *Auth) RegenerateToken() error {
	tok := randStr(40)
	return a.store.UpdateSettings(func(s *Settings) { s.APIToken = tok })
}

func (a *Auth) APIToken() string {
	st := a.store.GetSettings()
	if st == nil {
		return ""
	}
	return st.APIToken
}

func (a *Auth) CheckToken(tok string) bool {
	want := a.APIToken()
	if want == "" || tok == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(tok), []byte(want)) == 1
}

// ---------- sessions ----------

const sessionCookie = "ih_session"

func (a *Auth) NewSession(w http.ResponseWriter, r *http.Request) {
	tok := randHex(32)
	a.mu.Lock()
	a.sessions[tok] = time.Now().Add(7 * 24 * time.Hour)
	a.mu.Unlock()
	secure := r.TLS != nil
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		MaxAge:   7 * 24 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

func (a *Auth) DropSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
}

func (a *Auth) HasSession(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	exp, ok := a.sessions[c.Value]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(a.sessions, c.Value)
		return false
	}
	return true
}

// LoginAllowed: simple per-IP login rate limit (10 attempts / 10 min).
func (a *Auth) LoginAllowed(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	var recent []time.Time
	for _, t := range a.logins[ip] {
		if now.Sub(t) < 10*time.Minute {
			recent = append(recent, t)
		}
	}
	a.logins[ip] = recent
	return len(recent) < 10
}

func (a *Auth) LoginAttempt(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logins[ip] = append(a.logins[ip], time.Now())
}

// identity: "admin" if session or API token valid, else "guest".
func (a *Auth) identity(r *http.Request) string {
	if a.HasSession(r) {
		return "admin"
	}
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") && a.CheckToken(strings.TrimPrefix(h, "Bearer ")) {
		return "admin"
	}
	if t := r.Header.Get("X-Api-Token"); t != "" && a.CheckToken(t) {
		return "admin"
	}
	return "guest"
}
