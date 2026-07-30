package sample

// Exported — prod entry (exported => в R). Реальный мёртвый prod-символ ниже.
func Exported() { used() }

func used() {}

// deadProd — НЕ exported, НЕ вызван ниоткуда: реальный prod dead-code (должен ловиться).
func deadProd() {}
