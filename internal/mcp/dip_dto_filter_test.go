package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

// WARNING-проверка DIP DTO-фильтра (precision): behavioral(C) различается от DTO структурно.
// Сохраняет DIP-сигнал (больной abstract->concrete-С-ПОВЕДЕНИЕМ -> fire), убирает FP (DTO).
func TestDIP_DTOFilter_BehavioralVsDTO(t *testing.T) {
	dir := t.TempDir()
	code := `package sample

// Behavioral: метод с control-flow И вызовом -> поведенческая деталь.
type Worker struct{ n int }

func (w *Worker) Process(items []int) int {
	total := 0
	for _, x := range items {
		total += compute(x)
	}
	return total
}

func compute(x int) int { return x * 2 }

// DTO: только поля + аксессоры (нет control-flow, нет вызовов) -> словарь абстракции.
type UserDTO struct {
	Name string
	Age  int
}

func (u *UserDTO) GetName() string { return u.Name }
func (u *UserDTO) GetAge() int     { return u.Age }
`
	if err := os.WriteFile(filepath.Join(dir, "s.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	if _, err := a.Analyze(dir); err != nil {
		t.Fatal(err)
	}

	workerID := findTypeID(t, a, "Worker")
	dtoID := findTypeID(t, a, "UserDTO")

	// W2 не молчит: behavioral concrete -> isBehavioralType true (DIP-кандидат сохраняется).
	if !isBehavioralType(a, workerID) {
		t.Error("Worker (control-flow + вызов) должен быть behavioral -> DIP-сигнал не теряем")
	}

	// precision: DTO (только аксессоры) -> false (FP убран, ссылка на DTO не DIP-дефект).
	if isBehavioralType(a, dtoID) {
		t.Error("UserDTO (только поля+аксессоры) НЕ behavioral -> не DIP-дефект (словарь абстракции)")
	}
}
