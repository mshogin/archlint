package sample

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/pkg/tracer"
)

func TestCalculateWithTrace(t *testing.T) {
	// Создаем директорию для трейсов
	traceDir := "traces"
	if err := os.MkdirAll(traceDir, 0755); err != nil {
		t.Fatalf("Failed to create trace dir: %v", err)
	}

	// Начинаем трассировку
	trace := tracer.StartTrace("TestCalculateWithTrace")
	defer func() {
		trace = tracer.StopTrace()
		if trace != nil {
			traceFile := filepath.Join(traceDir, "test_calculate.json")
			if err := trace.Save(traceFile); err != nil {
				t.Fatalf("Failed to save trace: %v", err)
			}
		}
	}()

	// Вызываем функции с трассировкой
	tracer.Enter("sample.NewCalculator")
	calc := NewCalculator()
	tracer.ExitSuccess("sample.NewCalculator")

	tracer.Enter("sample.Calculator.Calculate")
	result := calc.Calculate(5, 3)
	tracer.ExitSuccess("sample.Calculator.Calculate")

	if result != 16 {
		t.Errorf("Expected 16, got %d", result)
	}
}
