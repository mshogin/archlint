package sample

import "testing"

// usedTestHelper — объявлен в _test.go, ВЫЗВАН из TestFoo (как killDaemon@archai).
func usedTestHelper() {}

// unusedTestHelper — объявлен в _test.go, НЕ вызван: «dead test code».
// До фикса оба ложно-метились dead-code (unreachable из prod-R). После — пропуск.
func unusedTestHelper() {}

func TestFoo(t *testing.T) { usedTestHelper() }
