package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"success": false, "error": msg})
}

func (a *App) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.auth.identity(r) != "admin" {
			errJSON(w, http.StatusUnauthorized, "需要管理员登录")
			return
		}
		next(w, r)
	}
}

// detectImageType sniffs magic bytes; returns extension + mime, or "" if not an allowed image.
func detectImageType(b []byte) (ext, mime string) {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "jpg", "image/jpeg"
	case len(b) >= 8 && b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4E && b[3] == 0x47 &&
		b[4] == 0x0D && b[5] == 0x0A && b[6] == 0x1A && b[7] == 0x0A:
		return "png", "image/png"
	case len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a"):
		return "gif", "image/gif"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "webp", "image/webp"
	case len(b) >= 12 && string(b[4:8]) == "ftyp" && (string(b[8:12]) == "avif" || string(b[8:12]) == "avis"):
		return "avif", "image/avif"
	case len(b) >= 2 && string(b[:2]) == "BM":
		return "bmp", "image/bmp"
	case len(b) >= 4 && b[0] == 0 && b[1] == 0 && b[2] == 1 && b[3] == 0:
		return "ico", "image/x-icon"
	}
	return "", ""
}

func sanitizeOrig(name string) string {
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if len(name) > 120 {
		name = name[:120]
	}
	if name == "" || name == "." {
		return "image"
	}
	return name
}

// ---------- /api/me ----------

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"success": true,
		"data": map[string]any{
			"admin":              a.auth.identity(r) == "admin",
			"guest_pin_required": a.cfg.GuestPIN != "",
			"limits": map[string]any{
				"max_file_mb":    a.cfg.MaxFileMB,
				"guest_daily":    a.cfg.GuestDaily,
				"guest_quota_gb": float64(a.cfg.GuestQuotaB) / 1073741824.0,
				"total_quota_gb": float64(a.cfg.TotalQuotaB) / 1073741824.0,
			},
		},
	})
}

// ---------- auth endpoints ----------

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := a.clientIP(r)
	if !a.auth.LoginAllowed(ip) {
		errJSON(w, http.StatusTooManyRequests, "登录尝试过于频繁，请稍后再试")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		errJSON(w, 400, "请求格式错误")
		return
	}
	a.auth.LoginAttempt(ip)
	if !a.auth.CheckPassword(body.Password) {
		errJSON(w, http.StatusUnauthorized, "密码不正确")
		return
	}
	a.auth.NewSession(w, r)
	writeJSON(w, 200, map[string]any{"success": true})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.auth.DropSession(w, r)
	writeJSON(w, 200, map[string]any{"success": true})
}

// ---------- upload ----------

type uploadResult struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Name     string `json:"name"`
	Orig     string `json:"orig"`
	Size     int64  `json:"size"`
	Markdown string `json:"markdown"`
	HTML     string `json:"html"`
	BBCode   string `json:"bbcode"`
}

// doUpload contains the shared logic for both upload endpoints.
// Returns (results, httpCode, errMsg).
func (a *App) doUpload(w http.ResponseWriter, r *http.Request, lsky bool) ([]uploadResult, int, string) {
	ident := a.auth.identity(r)
	ip := a.clientIP(r)
	maxFile := a.cfg.MaxFileMB * 1024 * 1024

	// overall body cap: allow up to 6 files of max size + overhead
	maxBody := maxFile*6 + 1024*1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, 400, "上传解析失败（文件过大或格式错误）"
	}

	// collect files from fields "file" and "image"
	var files []struct {
		name string
		open func() (io.ReadCloser, error)
		size int64
	}
	if r.MultipartForm != nil {
		for _, field := range []string{"file", "image"} {
			for _, fh := range r.MultipartForm.File[field] {
				fh := fh
				files = append(files, struct {
					name string
					open func() (io.ReadCloser, error)
					size int64
				}{fh.Filename, func() (io.ReadCloser, error) { return fh.Open() }, fh.Size})
			}
		}
	}
	if len(files) == 0 {
		return nil, 400, "没有收到文件"
	}
	if lsky && len(files) > 1 {
		files = files[:1]
	}

	// guest restrictions
	if ident == "guest" {
		if a.cfg.GuestPIN != "" {
			pinOK := false
			if v := r.FormValue("pin"); v != "" && v == a.cfg.GuestPIN {
				pinOK = true
			}
			if c, err := r.Cookie("ih_guest_pin"); err == nil && c.Value == a.cfg.GuestPIN {
				pinOK = true
			}
			if !pinOK {
				return nil, 403, "游客上传需要口令"
			}
			// remember pin in a cookie for 30 days for convenience
			http.SetCookie(w, &http.Cookie{Name: "ih_guest_pin", Value: a.cfg.GuestPIN, Path: "/", MaxAge: 30 * 24 * 3600, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		}
		day, hour := a.store.GuestCount(ip)
		if day+len(files) > a.cfg.GuestDaily {
			return nil, 429, fmt.Sprintf("游客每日最多上传 %d 张", a.cfg.GuestDaily)
		}
		if hour+len(files) > a.cfg.GuestHourly {
			return nil, 429, "游客上传过于频繁，请稍后再试"
		}
	}

	totalBytes, guestBytes, _, _ := a.store.Totals()
	var results []uploadResult
	var savedBytes int64
	for _, f := range files {
		if f.size > maxFile {
			return results, 413, fmt.Sprintf("单个文件不能超过 %d MB", a.cfg.MaxFileMB)
		}
		if totalBytes+savedBytes+f.size > a.cfg.TotalQuotaB {
			return results, 507, "存储空间已满"
		}
		if ident == "guest" && guestBytes+savedBytes+f.size > a.cfg.GuestQuotaB {
			return results, 507, "游客存储空间已满"
		}
		rc, err := f.open()
		if err != nil {
			return results, 500, "读取上传文件失败"
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxFile+1))
		rc.Close()
		if err != nil || int64(len(data)) > maxFile {
			return results, 413, "文件读取失败或过大"
		}
		ext, _ := detectImageType(data)
		if ext == "" {
			return results, 415, "只允许上传图片文件（jpg/png/gif/webp/avif/bmp/ico）"
		}
		// save under uploads/YYYY/MM/DD/<rand>.<ext>
		now := time.Now()
		relDir := fmt.Sprintf("%04d/%02d/%02d", now.Year(), int(now.Month()), now.Day())
		var fname string
		for {
			fname = randStr(16) + "." + ext
			if _, err := os.Stat(filepath.Join(a.uploadsDir(), relDir, fname)); os.IsNotExist(err) {
				break
			}
		}
		absDir := filepath.Join(a.uploadsDir(), relDir)
		if err := os.MkdirAll(absDir, 0o755); err != nil {
			return results, 500, "服务器存储错误"
		}
		if err := os.WriteFile(filepath.Join(absDir, fname), data, 0o644); err != nil {
			return results, 500, "服务器存储错误"
		}
		rel := relDir + "/" + fname
		img := Image{
			ID:    randHex(8),
			File:  rel,
			Orig:  sanitizeOrig(f.name),
			Size:  int64(len(data)),
			Ext:   ext,
			Owner: ident,
			IP:    ip,
			TS:    now.Unix(),
		}
		if err := a.store.AddImage(img); err != nil {
			return results, 500, "服务器存储错误"
		}
		savedBytes += img.Size
		url := a.baseURL(r) + "/i/" + rel
		results = append(results, uploadResult{
			ID: img.ID, URL: url, Name: fname, Orig: img.Orig, Size: img.Size,
			Markdown: "![" + img.Orig + "](" + url + ")",
			HTML:     "<img src=\"" + url + "\" alt=\"" + img.Orig + "\">",
			BBCode:   "[img]" + url + "[/img]",
		})
	}
	if ident == "guest" {
		a.store.GuestInc(ip, len(results))
	}
	return results, 200, ""
}

func (a *App) handleUploadSimple(w http.ResponseWriter, r *http.Request) {
	results, code, msg := a.doUpload(w, r, false)
	if code != 200 {
		errJSON(w, code, msg)
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "data": results})
}

// Lsky Pro API format (PicGo lskypro plugin):
// POST /api/v1/upload, Authorization: Bearer <token>, multipart field "image"
// -> {"status":true,"message":"success","data":{"links":{"url":...,"html":...,"bbcode":...,"markdown":...,"thumbnail_url":...}}}
func (a *App) handleUploadLsky(w http.ResponseWriter, r *http.Request) {
	if a.auth.identity(r) != "admin" {
		writeJSON(w, 401, map[string]any{"status": false, "message": "unauthorized: invalid token"})
		return
	}
	results, code, msg := a.doUpload(w, r, true)
	if code != 200 || len(results) == 0 {
		writeJSON(w, code, map[string]any{"status": false, "message": msg})
		return
	}
	res := results[0]
	writeJSON(w, 200, map[string]any{
		"status":  true,
		"message": "success",
		"data": map[string]any{
			"links": map[string]any{
				"url":           res.URL,
				"html":          res.HTML,
				"bbcode":        res.BBCode,
				"markdown":      res.Markdown,
				"thumbnail_url": res.URL,
			},
		},
	})
}

// ---------- gallery / admin ----------

func (a *App) handleImages(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	owner := r.URL.Query().Get("owner") // all|admin|guest
	sortBy := r.URL.Query().Get("sort") // new|old|size_desc|size_asc|name
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 60
	}
	all := a.store.ListImages()
	var filtered []Image
	for _, im := range all {
		if owner == "admin" || owner == "guest" {
			if im.Owner != owner {
				continue
			}
		}
		if q != "" && !strings.Contains(strings.ToLower(im.Orig), q) && !strings.Contains(strings.ToLower(im.File), q) {
			continue
		}
		filtered = append(filtered, im)
	}
	switch sortBy {
	case "old":
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
	case "size_desc":
		sortDesc(filtered, func(x, y Image) bool { return x.Size > y.Size })
	case "size_asc":
		sortDesc(filtered, func(x, y Image) bool { return x.Size < y.Size })
	case "name":
		sortDesc(filtered, func(x, y Image) bool { return strings.ToLower(x.Orig) < strings.ToLower(y.Orig) })
	}
	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	base := a.baseURL(r)
	items := make([]map[string]any, 0, end-start)
	for _, im := range filtered[start:end] {
		url := base + "/i/" + im.File
		items = append(items, map[string]any{
			"id": im.ID, "url": url, "file": im.File, "orig": im.Orig,
			"size": im.Size, "ext": im.Ext, "owner": im.Owner, "ip": im.IP, "ts": im.TS,
			"markdown": "![" + im.Orig + "](" + url + ")",
		})
	}
	writeJSON(w, 200, map[string]any{
		"success": true,
		"data":    map[string]any{"total": total, "page": page, "page_size": pageSize, "items": items},
	})
}

func sortDesc(s []Image, less func(a, b Image) bool) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && less(s[j], s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	im, ok, err := a.store.RemoveImage(id)
	if err != nil {
		errJSON(w, 500, "删除失败")
		return
	}
	if !ok {
		errJSON(w, 404, "图片不存在")
		return
	}
	_ = os.Remove(filepath.Join(a.uploadsDir(), im.File))
	writeJSON(w, 200, map[string]any{"success": true})
}

func (a *App) handleStats(w http.ResponseWriter, r *http.Request) {
	tb, gb, tc, gc := a.store.Totals()
	writeJSON(w, 200, map[string]any{
		"success": true,
		"data": map[string]any{
			"total_bytes": tb, "guest_bytes": gb,
			"total_count": tc, "guest_count": gc,
			"today_count": a.store.TodayCount(),
			"total_quota": a.cfg.TotalQuotaB,
			"guest_quota": a.cfg.GuestQuotaB,
			"uptime_sec":  int64(time.Since(a.started).Seconds()),
			"api_token":   a.auth.APIToken(),
			"base_url":    a.baseURL(r),
		},
	})
}

func (a *App) handleTokenRegen(w http.ResponseWriter, r *http.Request) {
	if err := a.auth.RegenerateToken(); err != nil {
		errJSON(w, 500, "生成失败")
		return
	}
	writeJSON(w, 200, map[string]any{"success": true, "data": map[string]any{"token": a.auth.APIToken()}})
}

func (a *App) handlePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		errJSON(w, 400, "请求格式错误")
		return
	}
	if len(body.New) < 8 {
		errJSON(w, 400, "新密码至少 8 位")
		return
	}
	if !a.auth.CheckPassword(body.Old) {
		errJSON(w, http.StatusUnauthorized, "当前密码不正确")
		return
	}
	if err := a.auth.SetPassword(body.New); err != nil {
		errJSON(w, 500, "保存失败")
		return
	}
	writeJSON(w, 200, map[string]any{"success": true})
}

func (a *App) handlePurgeGuests(w http.ResponseWriter, r *http.Request) {
	removed, err := a.store.RemoveByOwner("guest")
	if err != nil {
		errJSON(w, 500, "清理失败")
		return
	}
	for _, im := range removed {
		_ = os.Remove(filepath.Join(a.uploadsDir(), im.File))
	}
	writeJSON(w, 200, map[string]any{"success": true, "data": map[string]any{"removed": len(removed)}})
}

// ---------- image serving ----------

func (a *App) serveImage(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/i/")
	clean := path.Clean("/" + rel)
	if strings.Contains(clean, "..") {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(a.uploadsDir(), clean)
	if !strings.HasPrefix(full, a.uploadsDir()) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	// content type by extension (files were validated on upload)
	switch strings.ToLower(filepath.Ext(full)) {
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	case ".avif":
		w.Header().Set("Content-Type", "image/avif")
	case ".bmp":
		w.Header().Set("Content-Type", "image/bmp")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	default:
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}
