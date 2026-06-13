package mcp

import (
	"encoding/json"
	"testing"

	"github.com/mshogin/archlint/internal/archlintcfg"
)

// helpers ---------------------------------------------------------------------

func cycleViol(target string, members string) Violation {
	// Message детерминирован (detectCycles сортирует членов) -> идентичность SCC.
	return Violation{Kind: "circular-dependency", Target: target, Message: "Circular dependency detected (SCC size 2): " + members}
}

func deadViol(qname string) Violation {
	return Violation{Kind: "dead-code", Target: qname, Message: "dead code: " + qname + " недостижим от entry points R"}
}

func layerViol(from, to string) Violation {
	return Violation{Kind: "layer-violation", Target: from, Message: "Forbidden dependency: " + from + " (app) -> " + to + " (infra)"}
}

// ЯДРО ГОРНИЛА: baseline на коде -> повторная дельта ТОГО ЖЕ кода ПУСТА.
func TestDeltaGate_IdempotentEmptyDelta(t *testing.T) {
	current := []Violation{
		cycleViol("a", "a <-> b"),
		cycleViol("b", "a <-> b"), // тот же цикл, другой пакет -> та же идентичность
		deadViol("internal/x.Foo"),
		layerViol("internal/app", "internal/infra"),
	}
	base := BuildBaseline(current)

	d := Delta(current, base)
	if len(d.New) != 0 {
		t.Fatalf("ожидалась ПУСТАЯ дельта на неизменном коде, получено NEW=%d: %+v", len(d.New), d.New)
	}
	if len(d.Existing) != len(current) {
		t.Errorf("все текущие должны быть Existing: got %d/%d", len(d.Existing), len(current))
	}
}

// +1 ДЕТЕРМИНИЗМ: два baseline одного кода БАЙТ-идентичны.
func TestDeltaGate_DeterministicSnapshot(t *testing.T) {
	// Порядок входа НАМЕРЕННО разный -> снимок обязан совпасть байт-в-байт.
	v1 := []Violation{
		layerViol("internal/app", "internal/infra"),
		deadViol("internal/z.Bar"),
		cycleViol("b", "a <-> b"),
		deadViol("internal/x.Foo"),
		cycleViol("a", "a <-> b"),
	}
	v2 := []Violation{
		deadViol("internal/x.Foo"),
		cycleViol("a", "a <-> b"),
		deadViol("internal/z.Bar"),
		cycleViol("b", "a <-> b"),
		layerViol("internal/app", "internal/infra"),
	}

	b1, err := json.MarshalIndent(BuildBaseline(v1), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b2, err := json.MarshalIndent(BuildBaseline(v2), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Fatalf("снимки НЕ байт-идентичны:\n--- 1 ---\n%s\n--- 2 ---\n%s", b1, b2)
	}
}

// +2 NO-BASELINE -> NO-BLOCK: nil baseline -> ERROR-class деградирует в telemetry.
func TestDeltaGate_NoBaselineNoBlock(t *testing.T) {
	for _, v := range []Violation{cycleViol("a", "a <-> b"), deadViol("p.Foo"), layerViol("a", "b")} {
		if lvl := EffectiveLevel(v, nil, nil); lvl != archlintcfg.LevelTelemetry {
			t.Errorf("%s без baseline: ожидался Telemetry (no-block), получен %v", v.Kind, lvl)
		}
	}
}

// +3 POSITIVE CONTROL: 1 новый дефект -> дельта = ровно он; откат -> пуста.
func TestDeltaGate_PositiveControl(t *testing.T) {
	baselineViols := []Violation{cycleViol("a", "a <-> b")}
	base := BuildBaseline(baselineViols)

	withNewDead := append([]Violation{}, baselineViols...)
	withNewDead = append(withNewDead, deadViol("internal/x.Leaked"))

	d := Delta(withNewDead, base)
	if len(d.New) != 1 || d.New[0].Target != "internal/x.Leaked" {
		t.Fatalf("ожидался ровно 1 NEW (internal/x.Leaked), получено: %+v", d.New)
	}

	// откат (вернули исходный код) -> дельта пуста.
	if d := Delta(baselineViols, base); len(d.New) != 0 {
		t.Errorf("после отката дельта должна быть пуста, получено NEW=%d", len(d.New))
	}
}

// +4 RENAME-CASE: переименование -> ожидаемо ложный-NEW (документируем, НЕ баг).
func TestDeltaGate_RenameIsFalseNew(t *testing.T) {
	base := BuildBaseline([]Violation{deadViol("internal/x.OldName")})
	// тот же мёртвый код, переименован -> строгий qname-key изменился -> NEW.
	d := Delta([]Violation{deadViol("internal/x.NewName")}, base)
	if len(d.New) != 1 {
		t.Fatalf("rename должен дать ложный-NEW (fail-safe, irritation), получено NEW=%d", len(d.New))
	}
}

// +5 ПО КЛАССАМ + 4 исходных голдена ТЗ через EffectiveLevel.
func TestDeltaGate_GateLevels(t *testing.T) {
	base := BuildBaseline([]Violation{
		cycleViol("a", "a <-> b"),
		deadViol("internal/x.Existing"),
	})

	cases := []struct {
		name string
		v    Violation
		base *Baseline
		want archlintcfg.Level
	}{
		// Голден 1: новый цикл vs baseline -> ERROR-block.
		{"new-cycle-blocks", cycleViol("c", "c <-> d"), base, archlintcfg.LevelTaboo},
		// Голден 2: существующий цикл (в baseline) -> telemetry no-block.
		{"existing-cycle-audit", cycleViol("a", "a <-> b"), base, archlintcfg.LevelTelemetry},
		// Голден 3: новый dead -> block.
		{"new-dead-blocks", deadViol("internal/x.New"), base, archlintcfg.LevelTaboo},
		// существующий dead -> telemetry.
		{"existing-dead-audit", deadViol("internal/x.Existing"), base, archlintcfg.LevelTelemetry},
		// Голден 4: no baseline -> audit fallback.
		{"no-baseline-audit", cycleViol("c", "c <-> d"), nil, archlintcfg.LevelTelemetry},
		// новый layer back-edge -> block.
		{"new-layer-blocks", layerViol("internal/app", "internal/infra"), base, archlintcfg.LevelTaboo},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveLevel(tc.v, nil, tc.base); got != tc.want {
				t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// не-ERROR-class НИКОГДА не блокирует через дельта-гейт (Ось-1: магнитуды не гейт).
func TestDeltaGate_NonErrorClassNeverBlocks(t *testing.T) {
	base := BuildBaseline(nil)
	warn := Violation{Kind: "high-efferent-coupling", Target: "p", Message: "coupling"}
	if lvl := EffectiveLevel(warn, &archlintcfg.Config{}, base); lvl == archlintcfg.LevelTaboo {
		t.Errorf("WARNING-магнитуда не должна блокировать дельта-гейтом, получен %v", lvl)
	}
	// и не должна попадать в baseline.
	if len(BuildBaseline([]Violation{warn}).Patterns) != 0 {
		t.Errorf("не-ERROR-class не должен попадать в baseline-снимок")
	}
}
