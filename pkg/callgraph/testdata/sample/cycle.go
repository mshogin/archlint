package sample

// CycleA -> CycleB -> CycleC -> CycleA (цикл).

// CycleA вызывает CycleB.
func CycleA() {
	CycleB()
}

// CycleB вызывает CycleC.
func CycleB() {
	CycleC()
}

// CycleC вызывает CycleA (создает цикл).
func CycleC() {
	CycleA()
}
