package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"opensporttrack/internal/tracking"
)

type Handler struct {
	manager *tracking.Manager
	ctx     context.Context
	log     *slog.Logger
}

func NewHandler(ctx context.Context, manager *tracking.Manager, logger *slog.Logger) http.Handler {
	h := &Handler{manager: manager, ctx: ctx, log: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /api/v1/activities", h.createActivity)
	mux.HandleFunc("GET /api/v1/activities/{id}", h.getActivity)
	mux.HandleFunc("POST /api/v1/activities/{id}/samples", h.addSample)
	mux.HandleFunc("PUT /api/v1/activities/{id}/route", h.setRoute)
	mux.HandleFunc("GET /api/v1/activities/{id}/live", h.live)
	return mux
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "expected a single JSON object", http.StatusBadRequest)
		return false
	}
	return true
}

func (h *Handler) createActivity(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Sport string `json:"sport"`
	}
	if !decodeJSON(w, r, &request, 64<<10) {
		return
	}
	activity, err := h.manager.Create(request.Sport)
	if err != nil {
		if errors.Is(err, tracking.ErrClosed) {
			http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	h.log.Info("activity created", "activity_id", activity.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(activity)
}

func (h *Handler) getActivity(w http.ResponseWriter, r *http.Request) {
	activity, err := h.manager.GetActivity(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, tracking.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if errors.Is(err, tracking.ErrClosed) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(activity)
}

func (h *Handler) addSample(w http.ResponseWriter, r *http.Request) {
	var sample tracking.Sample
	if !decodeJSON(w, r, &sample, 64<<10) {
		return
	}
	err := h.manager.AddSample(r.PathValue("id"), sample)
	switch {
	case errors.Is(err, tracking.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, tracking.ErrInvalidSample):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, tracking.ErrClosed):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) setRoute(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Positions []tracking.Position `json:"positions"`
	}
	if !decodeJSON(w, r, &request, 8<<20) {
		return
	}
	err := h.manager.SetRoute(r.PathValue("id"), request.Positions)
	switch {
	case errors.Is(err, tracking.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, tracking.ErrInvalidRoute):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, tracking.ErrClosed):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) live(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	snapshot, messages, unsubscribe, err := h.manager.Subscribe(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer unsubscribe()
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		h.log.Warn("websocket upgrade failed", "error", err)
		return
	}
	defer conn.CloseNow()
	// Consume control frames so a write-only stream still notices client disconnects.
	readCtx := conn.CloseRead(h.ctx)
	write := func(message tracking.Message) error {
		ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
		defer cancel()
		return wsjson.Write(ctx, conn, message)
	}
	if err := write(snapshot); err != nil {
		return
	}
	for {
		select {
		case msg, ok := <-messages:
			if !ok || write(msg) != nil {
				return
			}
		case <-readCtx.Done():
			return
		case <-h.ctx.Done():
			return
		}
	}
}
