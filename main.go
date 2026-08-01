// Dmitry Zhdanov — Cyber Portfolio backend
// Pure Go standard library (no external dependencies) — easy to build anywhere,
// including minimal Docker images without network access at build time.
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Embedded static assets (built into the binary)
// ---------------------------------------------------------------------------

//go:embed index.html
var indexHTML []byte

//go:embed admin.html
var adminHTML []byte

//go:embed style.css
var styleCSS []byte

//go:embed script.js
var scriptJS []byte

//go:embed admin.js
var adminJS []byte

//go:embed projects.seed.json
var projectsSeed []byte

// 100 сгенерированных демо-лендингов карточек, имена вида s-<hash>.html —
// это и есть "скрытые пути": не последовательные и не угадываемые.
//
//go:embed s-*.html
var landingFiles embed.FS

// ---------------------------------------------------------------------------
// Data models
// ---------------------------------------------------------------------------

type Project struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Niche       string   `json:"niche"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
	Tech        []string `json:"tech"`
	Theme       string   `json:"theme"`
	DemoURL     string   `json:"demo_url"`
	Image       string   `json:"image"`
	Secure      bool     `json:"secure"`
}

type Profile struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Email    string `json:"email"`
	Telegram string `json:"telegram"`
	Lead     string `json:"lead"`
}

type Lead struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Contact   string `json:"contact"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type AdminAuth struct {
	Salt         string `json:"salt"`
	PasswordHash string `json:"password_hash"`
}

// ---------------------------------------------------------------------------
// Data directory / file paths (flat, same level as the binary)
// ---------------------------------------------------------------------------

var dataDir = "."

func dataPath(name string) string { return filepath.Join(dataDir, name) }

const (
	fileProjects = "data_projects.json"
	fileProfile  = "data_profile.json"
	fileLeads    = "data_leads.json"
	fileAdmin    = "data_admin.json"
)

// ---------------------------------------------------------------------------
// In-memory store guarded by a mutex, persisted to disk on every mutation
// ---------------------------------------------------------------------------

type Store struct {
	mu       sync.Mutex
	Projects []Project
	Profile  Profile
	Leads    []Lead
	Admin    AdminAuth

	tokens map[string]time.Time
}

var store = &Store{tokens: map[string]time.Time{}}

func loadJSONFile(path string, v interface{}) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func saveJSONFile(path string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func hashPassword(password, salt string) string {
	h := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(h[:])
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// extremely unlikely; fall back to time-based value
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

func (s *Store) init() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Projects
	if err := loadJSONFile(dataPath(fileProjects), &s.Projects); err != nil {
		var seed []Project
		if err := json.Unmarshal(projectsSeed, &seed); err != nil {
			log.Fatalf("failed to parse embedded seed projects: %v", err)
		}
		s.Projects = seed
		if err := saveJSONFile(dataPath(fileProjects), s.Projects); err != nil {
			log.Printf("warning: could not persist seed projects: %v", err)
		}
	}

	// Profile
	if err := loadJSONFile(dataPath(fileProfile), &s.Profile); err != nil {
		s.Profile = Profile{
			Name:     "Дмитрий Жданов",
			Role:     "Full-stack разработчик",
			Email:    "contact@example.com",
			Telegram: "@your_telegram",
			Lead:     "Более 4 лет создаю сайты, Telegram-ботов, веб- и мобильные приложения.",
		}
		saveJSONFile(dataPath(fileProfile), s.Profile)
	}

	// Leads
	if err := loadJSONFile(dataPath(fileLeads), &s.Leads); err != nil {
		s.Leads = []Lead{}
		saveJSONFile(dataPath(fileLeads), s.Leads)
	}

	// Admin auth
	if err := loadJSONFile(dataPath(fileAdmin), &s.Admin); err != nil {
		pass := os.Getenv("ADMIN_PASSWORD")
		if pass == "" {
			pass = "changeme123"
			log.Println("=====================================================================")
			log.Println("ADMIN_PASSWORD env var not set. Using default password: changeme123")
			log.Println("Please set ADMIN_PASSWORD and change it via the admin panel ASAP.")
			log.Println("=====================================================================")
		}
		salt := randomHex(16)
		s.Admin = AdminAuth{Salt: salt, PasswordHash: hashPassword(pass, salt)}
		saveJSONFile(dataPath(fileAdmin), s.Admin)
	}
}

// ---------------------------------------------------------------------------
// Auth helpers
// ---------------------------------------------------------------------------

func (s *Store) newToken() string {
	t := randomHex(24)
	s.tokens[t] = time.Now().Add(24 * time.Hour)
	return t
}

func (s *Store) validToken(token string) bool {
	exp, ok := s.tokens[token]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.tokens, token)
		return false
	}
	return true
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		store.mu.Lock()
		ok := token != "" && store.validToken(token)
		store.mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// ---------------------------------------------------------------------------
// Handlers — static files
// ---------------------------------------------------------------------------

// handleLandingFile serves one of the 100 generated demo landing pages.
// Only names matching the embedded "s-*.html" pattern are ever readable;
// anything else (or any attempt at path traversal) returns 404.
func handleLandingFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !strings.HasPrefix(name, "s-") || !strings.HasSuffix(name, ".html") || strings.ContainsAny(name, "/\\") {
		http.NotFound(w, r)
		return
	}
	data, err := landingFiles.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}

func serveStatic(content []byte, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(content)
	}
}

// ---------------------------------------------------------------------------
// Handlers — projects
// ---------------------------------------------------------------------------

func handleProjectsList(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	defer store.mu.Unlock()
	out := make([]Project, len(store.Projects))
	copy(out, store.Projects)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	writeJSON(w, http.StatusOK, out)
}

func handleProjectsCreate(w http.ResponseWriter, r *http.Request) {
	var p Project
	if err := readJSON(r, &p); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if p.Title == "" {
		http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	maxID := 0
	for _, existing := range store.Projects {
		if existing.ID > maxID {
			maxID = existing.ID
		}
	}
	p.ID = maxID + 1
	if p.Theme == "" {
		p.Theme = "cyber-green"
	}
	store.Projects = append(store.Projects, p)
	saveJSONFile(dataPath(fileProjects), store.Projects)
	writeJSON(w, http.StatusCreated, p)
}

func handleProjectByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		store.mu.Lock()
		defer store.mu.Unlock()
		for _, p := range store.Projects {
			if p.ID == id {
				writeJSON(w, http.StatusOK, p)
				return
			}
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)

	case http.MethodPut:
		var upd Project
		if err := readJSON(r, &upd); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}
		store.mu.Lock()
		defer store.mu.Unlock()
		for i, p := range store.Projects {
			if p.ID == id {
				upd.ID = id
				if upd.Theme == "" {
					upd.Theme = p.Theme
				}
				store.Projects[i] = upd
				saveJSONFile(dataPath(fileProjects), store.Projects)
				writeJSON(w, http.StatusOK, upd)
				return
			}
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)

	case http.MethodDelete:
		store.mu.Lock()
		defer store.mu.Unlock()
		for i, p := range store.Projects {
			if p.ID == id {
				store.Projects = append(store.Projects[:i], store.Projects[i+1:]...)
				saveJSONFile(dataPath(fileProjects), store.Projects)
				writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
				return
			}
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// ---------------------------------------------------------------------------
// Handlers — profile
// ---------------------------------------------------------------------------

func handleProfileGet(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	defer store.mu.Unlock()
	writeJSON(w, http.StatusOK, store.Profile)
}

func handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	var p Profile
	if err := readJSON(r, &p); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.Profile = p
	saveJSONFile(dataPath(fileProfile), store.Profile)
	writeJSON(w, http.StatusOK, store.Profile)
}

// ---------------------------------------------------------------------------
// Handlers — contact / leads
// ---------------------------------------------------------------------------

func handleContactSubmit(w http.ResponseWriter, r *http.Request) {
	var l Lead
	if err := readJSON(r, &l); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if l.Name == "" || l.Contact == "" || l.Message == "" {
		http.Error(w, `{"error":"name, contact and message are required"}`, http.StatusBadRequest)
		return
	}
	// very small anti-abuse guard
	if len(l.Message) > 4000 || len(l.Name) > 200 || len(l.Contact) > 200 {
		http.Error(w, `{"error":"payload too large"}`, http.StatusBadRequest)
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	maxID := 0
	for _, existing := range store.Leads {
		if existing.ID > maxID {
			maxID = existing.ID
		}
	}
	l.ID = maxID + 1
	l.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	store.Leads = append(store.Leads, l)
	saveJSONFile(dataPath(fileLeads), store.Leads)
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func handleLeadsList(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	defer store.mu.Unlock()
	writeJSON(w, http.StatusOK, store.Leads)
}

// ---------------------------------------------------------------------------
// Handlers — auth
// ---------------------------------------------------------------------------

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	candidate := hashPassword(body.Password, store.Admin.Salt)
	if subtle.ConstantTimeCompare([]byte(candidate), []byte(store.Admin.PasswordHash)) != 1 {
		http.Error(w, `{"error":"invalid password"}`, http.StatusUnauthorized)
		return
	}
	token := store.newToken()
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func handleAdminPing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if len(body.NewPassword) < 6 {
		http.Error(w, `{"error":"new password too short"}`, http.StatusBadRequest)
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	candidate := hashPassword(body.OldPassword, store.Admin.Salt)
	if subtle.ConstantTimeCompare([]byte(candidate), []byte(store.Admin.PasswordHash)) != 1 {
		http.Error(w, `{"error":"old password incorrect"}`, http.StatusUnauthorized)
		return
	}
	salt := randomHex(16)
	store.Admin = AdminAuth{Salt: salt, PasswordHash: hashPassword(body.NewPassword, salt)}
	saveJSONFile(dataPath(fileAdmin), store.Admin)
	// invalidate existing sessions
	store.tokens = map[string]time.Time{}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------------------
// Simple request logger + basic security headers
// ---------------------------------------------------------------------------

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	if v := os.Getenv("DATA_DIR"); v != "" {
		dataDir = v
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("cannot create data dir %s: %v", dataDir, err)
	}
	store.init()

	mux := http.NewServeMux()

	// Static pages
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		serveStatic(indexHTML, "text/html; charset=utf-8")(w, r)
	})
	mux.HandleFunc("GET /admin.html", serveStatic(adminHTML, "text/html; charset=utf-8"))
	mux.HandleFunc("GET /style.css", serveStatic(styleCSS, "text/css; charset=utf-8"))
	mux.HandleFunc("GET /script.js", serveStatic(scriptJS, "application/javascript; charset=utf-8"))
	mux.HandleFunc("GET /admin.js", serveStatic(adminJS, "application/javascript; charset=utf-8"))

	// Демо-лендинги карточек: /s-<hash>.html — "скрытые пути" (не последовательные,
	// не индексируются, каждый уже содержит meta robots=noindex внутри самого файла).
	mux.HandleFunc("GET /{name}", handleLandingFile)

	// Public API
	mux.HandleFunc("GET /api/projects", handleProjectsList)
	mux.HandleFunc("GET /api/projects/{id}", handleProjectByID)
	mux.HandleFunc("GET /api/profile", handleProfileGet)
	mux.HandleFunc("POST /api/contact", handleContactSubmit)
	mux.HandleFunc("POST /api/login", handleLogin)

	// Protected API
	mux.HandleFunc("POST /api/projects", requireAuth(handleProjectsCreate))
	mux.HandleFunc("PUT /api/projects/{id}", requireAuth(handleProjectByID))
	mux.HandleFunc("DELETE /api/projects/{id}", requireAuth(handleProjectByID))
	mux.HandleFunc("PUT /api/profile", requireAuth(handleProfileUpdate))
	mux.HandleFunc("GET /api/leads", requireAuth(handleLeadsList))
	mux.HandleFunc("GET /api/admin/ping", requireAuth(handleAdminPing))
	mux.HandleFunc("POST /api/admin/password", requireAuth(handlePasswordChange))

	// health check for Railway
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	handler := withLogging(withSecurityHeaders(mux))

	log.Printf("Server starting on :%s (data dir: %s)", port, dataDir)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
