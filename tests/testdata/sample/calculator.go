package sample

// Calculator представляет простой калькулятор
type Calculator struct {
	memory int
}

// NewCalculator создает новый калькулятор
func NewCalculator() *Calculator {
	return &Calculator{memory: 0}
}

// Add складывает два числа
func Add(a, b int) int {
	return a + b
}

// Multiply умножает два числа
func Multiply(a, b int) int {
	return a * b
}

// AddToMemory добавляет значение в память
func (c *Calculator) AddToMemory(value int) {
	c.memory = Add(c.memory, value)
}

// GetMemory возвращает значение из памяти
func (c *Calculator) GetMemory() int {
	return c.memory
}

// Calculate выполняет вычисление с использованием памяти
func (c *Calculator) Calculate(a, b int) int {
	sum := Add(a, b)
	product := Multiply(sum, 2)
	c.AddToMemory(product)

	return c.GetMemory()
}
