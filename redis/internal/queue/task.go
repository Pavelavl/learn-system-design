package queue

import "time"

// Task представляет задачу в очереди
type Task struct {
	ID        string    `json:"id"`
	Payload   string    `json:"payload"`
	Priority  int       `json:"priority"`
	ExecuteAt time.Time `json:"execute_at"`
	Attempts  int       `json:"attempts"` // Количество попыток выполнения
}
