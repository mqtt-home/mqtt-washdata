package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/mqtt-home/mqtt-washdata/dryer"
	"github.com/philipparndt/go-logger"
	loggerchi "github.com/philipparndt/go-logger/chi"
)

const webRoot = "./web/dist"

// WebServer serves the REST API, SSE live stream and the static React app.
type WebServer struct {
	manager *dryer.Manager
	router  *chi.Mux

	mu         sync.RWMutex
	sseClients map[string]chan string
}

func NewWebServer(manager *dryer.Manager) *WebServer {
	ws := &WebServer{
		manager:    manager,
		router:     chi.NewRouter(),
		sseClients: make(map[string]chan string),
	}
	ws.setupRoutes()
	return ws
}

func (ws *WebServer) setupRoutes() {
	ws.router.Use(middleware.Recoverer)
	ws.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	ws.router.Route("/api", func(r chi.Router) {
		// Health probes are polled by orchestrators — keep them out of the log.
		r.Get("/health", ws.healthCheck)
		r.Get("/livez", ws.liveness)
		r.Group(func(r chi.Router) {
			r.Use(loggerchi.Logger())
			r.Get("/status", ws.getStatus)
			r.Get("/runs", ws.getRuns)
			r.Get("/runs/{id}", ws.getRun)
			r.Post("/runs/{id}/label", ws.labelRun)
			r.Delete("/runs/{id}", ws.deleteRun)
			r.Get("/programs", ws.getPrograms)
			r.Get("/export", ws.exportData)
			r.Post("/import", ws.importData)
			r.Get("/events", ws.handleSSE)
		})
	})

	// Serve the React SPA, falling back to index.html for client-side routes.
	fileServer := http.FileServer(http.Dir(webRoot))
	ws.router.Handle("/*", func() http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			path := webRoot + r.URL.Path
			if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
			http.ServeFile(w, r, webRoot+"/index.html")
		}
	}())
}

func (ws *WebServer) Start(port int) error {
	addr := ":" + strconv.Itoa(port)
	logger.Info("Starting web server", "addr", addr)
	return http.ListenAndServe(addr, ws.router)
}

// --- REST handlers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (ws *WebServer) healthCheck(w http.ResponseWriter, _ *http.Request) {
	ws.mu.RLock()
	clients := len(ws.sseClients)
	ws.mu.RUnlock()
	writeJSON(w, map[string]any{
		"status":      "ok",
		"goroutines":  runtime.NumGoroutine(),
		"runs":        len(ws.manager.Runs()),
		"programs":    len(ws.manager.Programs()),
		"sse_clients": clients,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (ws *WebServer) liveness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (ws *WebServer) getStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, ws.manager.Live())
}

// runSummary is a run without its (potentially large) sample array.
type runSummary struct {
	*dryer.Run
	Samples any `json:"samples,omitempty"`
}

func (ws *WebServer) getRuns(w http.ResponseWriter, _ *http.Request) {
	runs := ws.manager.Runs()
	out := make([]runSummary, len(runs))
	for i, r := range runs {
		out[i] = runSummary{Run: r}
	}
	writeJSON(w, out)
}

func (ws *WebServer) getRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	run, ok := ws.manager.Run(id)
	if !ok {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	writeJSON(w, run)
}

func (ws *WebServer) labelRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Program string `json:"program"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	run, err := ws.manager.LabelRun(id, body.Program)
	if errors.Is(err, dryer.ErrRunNotFound) {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, run)
}

func (ws *WebServer) deleteRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := ws.manager.DeleteRun(id)
	if errors.Is(err, dryer.ErrRunNotFound) {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (ws *WebServer) getPrograms(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, ws.manager.Programs())
}

// exportPayload is the portable snapshot of an instance's learned data: all
// runs including their samples and labels. Programs are not exported — they
// are re-derived from the runs on import.
type exportPayload struct {
	Version    int          `json:"version"`
	ExportedAt time.Time    `json:"exportedAt"`
	Runs       []*dryer.Run `json:"runs"`
}

func (ws *WebServer) exportData(w http.ResponseWriter, _ *http.Request) {
	payload := exportPayload{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Runs:       ws.manager.Runs(),
	}
	name := "washdata-export-" + time.Now().UTC().Format("20060102-150405") + ".json"
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	writeJSON(w, payload)
}

func (ws *WebServer) importData(w http.ResponseWriter, r *http.Request) {
	var payload exportPayload
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<20)).Decode(&payload); err != nil {
		http.Error(w, "invalid export file: "+err.Error(), http.StatusBadRequest)
		return
	}
	if payload.Version != 1 {
		http.Error(w, fmt.Sprintf("unsupported export version %d", payload.Version), http.StatusBadRequest)
		return
	}
	imported, skipped, err := ws.manager.ImportRuns(payload.Runs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]int{"imported": imported, "skipped": skipped})
}

// --- SSE ---

// BroadcastStatus pushes a live status update to all connected SSE clients.
func (ws *WebServer) BroadcastStatus(ls dryer.LiveStatus) {
	data, err := json.Marshal(ls)
	if err != nil {
		logger.Error("Failed to marshal live status for SSE", "error", err)
		return
	}
	msg := string(data)

	ws.mu.RLock()
	defer ws.mu.RUnlock()
	for _, ch := range ws.sseClients {
		select {
		case ch <- msg:
		default: // drop if the client is slow
		}
	}
}

func (ws *WebServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	clientID := fmt.Sprintf("%d", time.Now().UnixNano())
	ch := make(chan string, 10)

	ws.mu.Lock()
	ws.sseClients[clientID] = ch
	ws.mu.Unlock()
	logger.Debug("SSE client connected", "client", clientID)

	defer func() {
		ws.mu.Lock()
		delete(ws.sseClients, clientID)
		close(ch)
		ws.mu.Unlock()
		logger.Debug("SSE client disconnected", "client", clientID)
	}()

	// Send the current status immediately.
	if data, err := json.Marshal(ws.manager.Live()); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-keepAlive.C:
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
