package main

import "testing"

// Golden калибровки health v3: значения откалиброваны замером на спектре репо (self + open-source Go).
// Закрепляет, что НОРМАЛИЗОВАННАЯ формула даёт ОСМЫСЛЕННЫЙ health — не обнуляется на объёме WARN
// (дефект линейной v2), не выдаёт ложно-идеальный. Если калибровка/формула изменится — тест краснеет.
func TestComputeHealth_Calibration(t *testing.T) {
	cases := []struct {
		name                    string
		errs, warns, loc, wantH int
	}{
		{"чистый малый репо (godotenv: 0 warns)", 0, 0, 1155, 100},
		{"good (color: density 4.8)", 0, 6, 1261, 85},
		{"good (charmbracelet-log: density 3.4)", 0, 12, 3487, 88},
		{"плотный WARN НЕ обнуляется (env: density 15.3)", 0, 52, 3406, 74}, // линейная v2 дала бы 0
		{"self (ERROR 9 + WARN density 4.4)", 9, 134, 30192, 41},
		{"ERROR-доминанта (twirp: 423 errs)", 423, 861, 68863, 0},
	}

	for _, c := range cases {
		h, _ := computeHealth(c.errs, c.warns, c.loc)
		if h != c.wantH {
			t.Errorf("%s: health=%d, want %d", c.name, h, c.wantH)
		}
	}
}

// СВОЙСТВО (ядро решения дефекта v2): WARN-слой НИКОГДА не обнуляет health (обнуляет только ERROR).
// Любой объём WARN при 0 ERROR -> health >= 100 - warnMax (асимптота насыщения), строго > 0.
func TestComputeHealth_WarnNeverZeroes(t *testing.T) {
	// Экстремальная WARN-плотность, 0 ERROR: штраф насыщается на warnMax, не обнуляет.
	h, _ := computeHealth(0, 1_000_000, 1000)

	floor := 100 - int(warnMax)
	if h < floor-1 {
		t.Errorf("WARN-штраф превысил warnMax=%v: health=%d < %d (формула должна насыщаться)", warnMax, h, floor)
	}
	if h <= 0 {
		t.Errorf("WARN обнулил health (%d) — обнулять должен ТОЛЬКО ERROR, WARN насыщается", h)
	}
}

// density монотонна и нормализует по размеру: тот же WARN-объём на большем репо -> меньше density
// -> выше health (плотность, не объём). Два репо с равными warns, разный LOC.
func TestComputeHealth_DensityNormalizes(t *testing.T) {
	small, _ := computeHealth(0, 50, 2000)  // density 25
	large, _ := computeHealth(0, 50, 50000) // density 1

	if large <= small {
		t.Errorf("тот же WARN-объём на большем репо должен давать ВЫШЕ health (меньше плотность): small=%d large=%d", small, large)
	}
}
