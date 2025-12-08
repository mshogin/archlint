package tracer

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Trace представляет трассировку выполнения теста.
type Trace struct {
	TestName  string    `json:"test_name"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Calls     []Call    `json:"calls"`
	mu        sync.Mutex
}

// Call представляет вызов функции.
type Call struct {
	Event     string    `json:"event"`
	Function  string    `json:"function"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
	Depth     int       `json:"depth"`
}

var (
	currentTrace *Trace
	traceMu      sync.Mutex
	callDepth    int
)

// StartTrace начинает новую трассировку.
func StartTrace(testName string) *Trace {
	traceMu.Lock()
	defer traceMu.Unlock()

	currentTrace = &Trace{
		TestName:  testName,
		StartTime: time.Now(),
		Calls:     []Call{},
	}
	callDepth = 0

	return currentTrace
}

// Enter регистрирует вход в функцию.
func Enter(fn string) {
	traceMu.Lock()
	defer traceMu.Unlock()

	if currentTrace == nil {
		return
	}

	currentTrace.mu.Lock()
	defer currentTrace.mu.Unlock()

	currentTrace.Calls = append(currentTrace.Calls, Call{
		Event:     "enter",
		Function:  fn,
		Timestamp: time.Now(),
		Depth:     callDepth,
	})

	callDepth++
}

// Exit регистрирует выход из функции (deprecated, используйте ExitSuccess/ExitError).
func Exit(fn string, err error) {
	if err != nil {
		ExitError(fn, err)
	} else {
		ExitSuccess(fn)
	}
}

// ExitSuccess регистрирует успешный выход из функции.
func ExitSuccess(fn string) {
	traceMu.Lock()
	defer traceMu.Unlock()

	if currentTrace == nil {
		return
	}

	callDepth--
	if callDepth < 0 {
		callDepth = 0
	}

	currentTrace.mu.Lock()
	defer currentTrace.mu.Unlock()

	currentTrace.Calls = append(currentTrace.Calls, Call{
		Event:     "exit_success",
		Function:  fn,
		Timestamp: time.Now(),
		Depth:     callDepth,
	})
}

// ExitError регистрирует выход из функции с ошибкой.
func ExitError(fn string, err error) {
	traceMu.Lock()
	defer traceMu.Unlock()

	if currentTrace == nil {
		return
	}

	callDepth--
	if callDepth < 0 {
		callDepth = 0
	}

	currentTrace.mu.Lock()
	defer currentTrace.mu.Unlock()

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	currentTrace.Calls = append(currentTrace.Calls, Call{
		Event:     "exit_error",
		Function:  fn,
		Timestamp: time.Now(),
		Error:     errMsg,
		Depth:     callDepth,
	})
}

// Save сохраняет трассировку в файл.
func (t *Trace) Save(filename string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.EndTime = time.Now()

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка сериализации трассировки: %w", err)
	}

	if err := os.WriteFile(filename, data, 0o600); err != nil {
		return fmt.Errorf("ошибка записи файла трассировки: %w", err)
	}

	return nil
}

// StopTrace останавливает текущую трассировку.
func StopTrace() *Trace {
	traceMu.Lock()
	defer traceMu.Unlock()

	trace := currentTrace
	currentTrace = nil
	callDepth = 0

	return trace
}
