package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"task-queue/internal/config"
	"task-queue/internal/mocks"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestHandler_ServeHTTP(t *testing.T) {
	mc := minimock.NewController(t)
	// Настраиваем конфигурацию
	cfg := &config.Config{
		Priorities: config.PrioritiesConfig{
			Low:    1,
			Medium: 2,
			High:   3,
		},
	}

	mockQueue := mocks.NewITaskQueueMock(mc)
	handler := NewHandler(mockQueue, cfg, zap.L())

	tests := []struct {
		name           string
		method         string
		path           string
		body           interface{}
		expectedStatus int
		expectedBody   string
		setupMock      func()
	}{
		{
			name:           "Successful POST /tasks",
			method:         http.MethodPost,
			path:           "/tasks",
			body:           TaskRequest{Payload: "Test task", Priority: 2, ExecuteAt: time.Now().Add(5 * time.Second)},
			expectedStatus: http.StatusCreated,
			expectedBody:   "{\"status\":\"task added\"}\n",
			setupMock: func() {
				mockQueue.AddTaskMock.Return(nil)
			},
		},
		{
			name:           "Invalid JSON body",
			method:         http.MethodPost,
			path:           "/tasks",
			body:           "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid request body\n",
			setupMock:      func() {},
		},
		{
			name:           "Empty payload",
			method:         http.MethodPost,
			path:           "/tasks",
			body:           TaskRequest{Payload: "", Priority: 2},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Payload is required\n",
			setupMock:      func() {},
		},
		{
			name:           "Invalid priority (too low)",
			method:         http.MethodPost,
			path:           "/tasks",
			body:           TaskRequest{Payload: "Test task", Priority: 0},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid priority\n",
			setupMock:      func() {},
		},
		{
			name:           "Invalid priority (too high)",
			method:         http.MethodPost,
			path:           "/tasks",
			body:           TaskRequest{Payload: "Test task", Priority: 4},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid priority\n",
			setupMock:      func() {},
		},
		{
			name:           "AddTask error",
			method:         http.MethodPost,
			path:           "/tasks",
			body:           TaskRequest{Payload: "Test task", Priority: 2},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to add task\n",
			setupMock: func() {
				mockQueue.AddTaskMock.
					Return(errors.New("failed to add task"))
			},
		},
		{
			name:           "Unsupported path",
			method:         http.MethodPost,
			path:           "/unknown",
			body:           nil,
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Not found\n",
			setupMock:      func() {},
		},
		{
			name:           "Unsupported method",
			method:         http.MethodGet,
			path:           "/tasks",
			body:           nil,
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Not found\n",
			setupMock:      func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
				if tt.name == "Invalid JSON body" {
					body = []byte(tt.body.(string))
				}
			}

			req, err := http.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, "Unexpected status code")
			assert.Equal(t, tt.expectedBody, rr.Body.String(), "Unexpected response body")
		})
	}
}
