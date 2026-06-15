package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

// WARNING-проверка соундности SRP->decomposability (srp-lack-of-cohesion, LCOM4>=2).
// Зеркало ERROR self-проверки, но КРИТЕРИЙ ДРУГОЙ (WARNING — сигнал, не блок):
//   - НЕ требуем 0-false-fire на здоровом (это ERROR-ворота, к WARNING НЕ применимо; FP легален).
//   - W1/W2 0-FALSE-SILENCE: на курируемом БОЛЬНОМ синтет-эталоне (God-class с >=2 непересекающимися
//     группами методов, разложимость по конструкции) метрика ОБЯЗАНА fire (LCOM4>=2).
//   - W3 НАПРАВЛЕННОСТЬ (нет инверсии знака): легальное УЛУЧШЕНИЕ принципа (разбиение God-class на
//     когезивные типы) НЕ повышает метрику — осколки LCOM4=1, max НЕ растёт относительно God.
//
// NB по соундности: реализация LCOM4 (lcom4.go) строит граф ТОЛЬКО по взаимным вызовам методов,
// без shared-field рёбер (TODO в lcom4.go) — расходится с декларацией карточки "поле ИЛИ вызов".
// Для WARNING это легально (FP precision<1: когезивный-через-поля тип может ложно сработать),
// но decomposability доказывается по подграфу ВЫЗОВОВ, не по полному LCOM4.

// lcomFires воспроизводит решающее правило srp-lack-of-cohesion (metrics.go: LCOM>=2 -> fire).
func lcomFires(t *testing.T, code, typeName string) (fired bool, lcom int) {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "x.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, typeName)
	res := ComputeLCOM4(a, typeID, graph)

	return res.LCOM >= 2, res.LCOM
}

// БОЛЬНОЙ синтет-эталон: God-class, 2 непересекающиеся группы вызовов (viol_SRP=1 по конструкции).
const srpSickGod = `package sick

type God struct{}

func (g *God) A() { g.B() }
func (g *God) B() {}
func (g *God) C() { g.D() }
func (g *God) D() {}
`

// ЗДОРОВЫЙ эталон: когезивный тип, все методы в одной компоненте (разложимость=1).
const srpHealthyCohesive = `package healthy

type Cohesive struct{}

func (c *Cohesive) A() { c.B() }
func (c *Cohesive) B() { c.C() }
func (c *Cohesive) C() {}
`

// W3-улучшение: God разбит на 2 когезивных типа (каждый LCOM4=1).
const srpSplitShard1 = `package shard

type Shard1 struct{}

func (s *Shard1) A() { s.B() }
func (s *Shard1) B() {}
`

const srpSplitShard2 = `package shard

type Shard2 struct{}

func (s *Shard2) C() { s.D() }
func (s *Shard2) D() {}
`

func TestSRPDecomposability_WarningSoundness(t *testing.T) {
	// W1/W2 0-FALSE-SILENCE: больной God-class ОБЯЗАН fire.
	firedSick, lcomSick := lcomFires(t, srpSickGod, "God")
	if !firedSick {
		t.Fatalf("FALSE-SILENCE: больной God-class (LCOM4=%d) не сработал — WARNING обязан fire на viol_SRP=1", lcomSick)
	}

	if lcomSick < 2 {
		t.Fatalf("больной эталон должен иметь LCOM4>=2, got %d", lcomSick)
	}

	// Не always-fire: когезивный тип НЕ срабатывает (метрика различает, не тривиальна).
	firedHealthy, lcomHealthy := lcomFires(t, srpHealthyCohesive, "Cohesive")
	if firedHealthy {
		t.Errorf("когезивный тип (LCOM4=%d) сработал — метрика не должна быть always-fire", lcomHealthy)
	}

	// W3 НАПРАВЛЕННОСТЬ (нет инверсии): разбиение God на когезивные осколки -> LCOM4=1, метрика НЕ растёт.
	_, lcomShard1 := lcomFires(t, srpSplitShard1, "Shard1")
	_, lcomShard2 := lcomFires(t, srpSplitShard2, "Shard2")

	maxShard := lcomShard1
	if lcomShard2 > maxShard {
		maxShard = lcomShard2
	}

	if maxShard >= lcomSick {
		t.Errorf("W3-ИНВЕРСИЯ: улучшение (разбиение) не снизило метрику: God LCOM4=%d, max осколок LCOM4=%d", lcomSick, maxShard)
	}

	if lcomShard1 != 1 || lcomShard2 != 1 {
		t.Errorf("осколки должны быть когезивны (LCOM4=1), got %d, %d", lcomShard1, lcomShard2)
	}
}
