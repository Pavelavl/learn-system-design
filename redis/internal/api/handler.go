package api

import (
	"encoding/json"
	"net/http"
	"time"

	"task-queue/internal/config"
	"task-queue/internal/queue"

	"go.uber.org/zap"
)

// Handler управляет HTTP-ручками
type Handler struct {
	queue  queue.ITaskQueue
	cfg    *config.Config
	logger *zap.Logger
}

// NewHandler создаёт новый HTTP-обработчик
func NewHandler(queue queue.ITaskQueue, cfg *config.Config, logger *zap.Logger) *Handler {
	return &Handler{queue: queue, cfg: cfg, logger: logger}
}

// ServeHTTP настраивает маршруты
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if r.URL.Path == "/tasks" {
			h.addTask(w, r)
			return
		}
	}

	h.logger.Warn("Not found",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path))
	http.Error(w, "Not found", http.StatusNotFound)
}

// TaskRequest представляет запрос для добавления задачи
type TaskRequest struct {
	Payload   string    `json:"payload"`
	Priority  int       `json:"priority"`
	ExecuteAt time.Time `json:"execute_at"`
}

// addTask обрабатывает POST /tasks
func (h *Handler) addTask(w http.ResponseWriter, r *http.Request) {
	var req TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid request body",
			zap.String("remote_addr", r.RemoteAddr),
			zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Валидация
	if req.Payload == "" {
		h.logger.Warn("Payload is required",
			zap.String("remote_addr", r.RemoteAddr))
		http.Error(w, "Payload is required", http.StatusBadRequest)
		return
	}
	if req.Priority < h.cfg.Priorities.Low || req.Priority > h.cfg.Priorities.High {
		h.logger.Warn("Invalid priority",
			zap.Int("priority", req.Priority),
			zap.String("remote_addr", r.RemoteAddr))
		http.Error(w, "Invalid priority", http.StatusBadRequest)
		return
	}

	// Добавляем задачу
	err := h.queue.AddTask(r.Context(), req.Payload, req.Priority, req.ExecuteAt)
	if err != nil {
		h.logger.Error("Failed to add task",
			zap.String("payload", req.Payload),
			zap.Int("priority", req.Priority),
			zap.String("remote_addr", r.RemoteAddr),
			zap.Error(err))
		http.Error(w, "Failed to add task", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Task creation request processed",
		zap.String("payload", req.Payload),
		zap.Int("priority", req.Priority),
		zap.String("remote_addr", r.RemoteAddr))

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "task added"})
}
