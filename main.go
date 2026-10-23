package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ---------- config ----------

type Config struct {
	Bind         string // listen address, e.g. 127.0.0.1; empty = all interfaces
	Port         string
	DataDir      string
	BaseURL      string // e.g. https://img.example.com ; empty = derive from request
	MaxFileMB    int64
	GuestDaily   int
	GuestHourly  int
	GuestQuotaB  int64
	TotalQuotaB  int64
	GuestPIN     string // empty = off
	TrustProxy   bool
	AdminPassEnv string
}

func envStr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
func envInt(k string, def int) int {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
func envGB(k string, defGB int64) int64 {
	return int64(envInt(k, int(defGB))) * 1024 * 1024 * 1024
}

func loadConfig() Config {
	return Config{
		Bind:         envStr("IMG_BIND", ""),
		Port:         envStr("IMG_PORT", "8080"),
		DataDir:      envStr("IMG_DATA_DIR", "./data"),
		BaseURL:      strings.TrimRight(envStr("IMG_BASE_URL", ""), "/"),
		MaxFileMB:    int64(envInt("IMG_MAX_FILE_MB", 10)),
		GuestDaily:   envInt("IMG_GUEST_DAILY", 20),
		GuestHourly:  envInt("IMG_GUEST_HOURLY", 60),
		GuestQuotaB:  envGB("IMG_GUEST_QUOTA_GB", 2),
		TotalQuotaB:  envGB("IMG_TOTAL_QUOTA_GB", 10),
		GuestPIN:     envStr("IMG_GUEST_PIN", ""),
		TrustProxy:   envStr("IMG_TRUST_PROXY", "1") != "0",
		AdminPassEnv: os.Getenv("IMG_ADMIN_PASSWORD"),
	}
}

// ---------- app ----------

type App struct {
	cfg     Config
	store   *Store
	auth    *Auth
	started time.Time
}

func (a *App) uploadsDir() string { return filepath.Join(a.cfg.DataDir, "uploads") }

func (a *App) clientIP(r *http.Request) string {
	if a.cfg.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if xr := r.Header.Get("X-Real-IP"); xr != "" {
			return xr
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *App) baseURL(r *http.Request) string {
	if a.cfg.BaseURL != "" {
		return a.cfg.BaseURL
	}
	proto := "http"
	if r.TLS != nil {
		proto = "https"
	} else if a.cfg.TrustProxy {
		if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
			proto = p
		}
	}
	return proto + "://" + r.Host
}

//go:embed web/*
var webFS embed.FS

func main() {
	log.SetFlags(log.LstdFlags)
	cfg := loadConfig()
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "uploads"), 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	store, err := NewStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	auth := NewAuth(store)
	app := &App{cfg: cfg, store: store, auth: auth, started: time.Now()}

	if store.FirstRun() {
		pass := cfg.AdminPassEnv
		generated := false
		if pass == "" {
			pass = randStr(16)
			generated = true
		}
		if err := auth.SetPassword(pass); err != nil {
			log.Fatalf("set password: %v", err)
		}
		if err := auth.RegenerateToken(); err != nil {
			log.Fatalf("token: %v", err)
		}
		if generated {
			log.Printf("==================================================")
			log.Printf(" FIRST RUN: admin password generated: %s", pass)
			log.Printf(" Log in with it and change it in the admin panel.")
			log.Printf("==================================================")
		} else {
			log.Printf("first run: admin password taken from IMG_ADMIN_PASSWORD")
		}
	}

	mux := http.NewServeMux()

	// static web UI (no-cache so UI updates take effect immediately)
	sub, _ := fs.Sub(webFS, "web")
	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	}))

	// public image files
	mux.HandleFunc("GET /i/", app.serveImage)

	// api
	mux.HandleFunc("GET /api/me", app.handleMe)
	mux.HandleFunc("POST /api/login", app.handleLogin)
	mux.HandleFunc("POST /api/logout", app.handleLogout)
	mux.HandleFunc("POST /api/upload", app.handleUploadSimple)
	mux.HandleFunc("POST /api/v1/upload", app.handleUploadLsky) // PicGo lskypro plugin format
	mux.HandleFunc("GET /api/images", app.requireAdmin(app.handleImages))
	mux.HandleFunc("DELETE /api/images/{id}", app.requireAdmin(app.handleDelete))
	mux.HandleFunc("GET /api/stats", app.requireAdmin(app.handleStats))
	mux.HandleFunc("POST /api/admin/token", app.requireAdmin(app.handleTokenRegen))
	mux.HandleFunc("POST /api/admin/password", app.requireAdmin(app.handlePassword))
	mux.HandleFunc("POST /api/admin/purge-guests", app.requireAdmin(app.handlePurgeGuests))

	addr := cfg.Bind + ":" + cfg.Port
	log.Printf("image-host listening on %s (data: %s)", addr, cfg.DataDir)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

var _ = fmt.Sprintf
