package sample

import "github.com/mshogin/archlint/pkg/tracer"

// Calculator представляет простой калькулятор
type Calculator struct {
	memory int
}

// NewCalculator создает новый калькулятор
func NewCalculator() *Calculator {
	tracer.Enter("sample.NewCalculator")
	tracer.ExitSuccess("sample.NewCalculator")
	return &Calculator{memory: 0}
}

// Add складывает два числа
func Add(a, b int) int {
	tracer.Enter("sample.Add")
	tracer.ExitSuccess("sample.Add")
	return a + b
}

// Multiply умножает два числа
func Multiply(a, b int) int {
	tracer.Enter("sample.Multiply")
	tracer.ExitSuccess("sample.Multiply")
	return a * b
}

// AddToMemory добавляет значение в память
func (c *Calculator) AddToMemory(value int) {
	tracer.Enter("sample.Calculator.AddToMemory")
	c.memory = Add(c.memory, value)
	tracer.ExitSuccess("sample.Calculator.AddToMemory")
}

// GetMemory возвращает значение из памяти
func (c *Calculator) GetMemory() int {
	tracer.Enter("sample.Calculator.GetMemory")
	tracer.ExitSuccess("sample.Calculator.GetMemory")
	return c.memory
}

// Calculate выполняет вычисление с использованием памяти
func (c *Calculator) Calculate(a, b int) int {
	tracer.Enter("sample.Calculator.Calculate")
	sum := Add(a, b)
	product := Multiply(sum, 2)
	c.AddToMemory(product)
	tracer.ExitSuccess("sample.Calculator.Calculate")
	return c.GetMemory()
}
