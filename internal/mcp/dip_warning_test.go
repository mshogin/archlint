package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// WARNING-проверка соундности STRICT-DIP (dip-abstraction-to-detail), шаблон как у SRP.
//
// STRICT-DIP fire = ребро abstract(interface) --uses/returns--> concrete(свой). Метрика
// СТРУКТУРНО НЕ различает concrete-с-поведением (истинный DIP-дефект) и concrete-DTO
// (legal FP: словарь абстракции, не поведенческая деталь) -> доказывает НАДМНОЖЕСТВО
// Def_DIP (term ⊇ Def_DIP) -> WARNING, не ERROR. Это ровно DTO-FP, под который вопрос
// квалифицирован как WARNING.
//
// Критерий WARNING (НЕ 0-false-fire на здоровом — это ERROR-ворота; FP легален):
//   - W1/W2 0-FALSE-SILENCE: больной abstract->concrete-с-поведением ОБЯЗАН fire;
//   - не always-fire: abstraction->abstraction (правильный DI) НЕ fire;
//   - W3 НАПРАВЛЕННОСТЬ: улучшение (concrete-зависимость заменена на интерфейс) -> метрика НЕ растёт;
//   - DTO-кейс: abstract->concrete-DTO -> fire = LEGAL FP (precision<1, whitelist держит на гейте;
//     это НЕ инверсия и НЕ false-silence -> демотация НЕ требуется).
func TestDIP_WarningSoundness(t *testing.T) {
	// W1/W2 0-FALSE-SILENCE: интерфейс зависит от своего concrete (с поведением) -> fire.
	sick := dipGraph(
		[]model.Node{kindNode("p.Service", model.KindInterface), kindNode("p.Worker", model.KindConcrete)},
		[]model.Edge{{From: "p.Service", To: "p.Worker", Type: model.EdgeUses}},
	)

	sickN := len(DetectDIP(sick))
	if sickN == 0 {
		t.Fatal("FALSE-SILENCE: abstract->concrete-с-поведением не сработал — WARNING обязан fire на viol_DIP=1")
	}

	// не always-fire: правильный DI (abstraction->abstraction) -> НЕ fire.
	healthy := dipGraph(
		[]model.Node{kindNode("p.Service", model.KindInterface), kindNode("p.Repo", model.KindInterface)},
		[]model.Edge{{From: "p.Service", To: "p.Repo", Type: model.EdgeUses}},
	)

	improvedN := len(DetectDIP(healthy))
	if improvedN != 0 {
		t.Errorf("always-fire: abstraction->abstraction сработал (%d) — правильный DI не должен fire", improvedN)
	}

	// W3 НАПРАВЛЕННОСТЬ: улучшение DIP (concrete-зависимость -> интерфейс) -> метрика НЕ растёт.
	if improvedN >= sickN {
		t.Errorf("W3-ИНВЕРСИЯ: улучшение (concrete->interface) не снизило метрику: sick=%d, improved=%d", sickN, improvedN)
	}

	// DTO-кейс = LEGAL FP: abstract->concrete-DTO структурно = abstract->concrete -> fire,
	// но это FP (DTO = словарь абстракции, не деталь). Для WARNING легально: whitelist на гейте
	// подавляет, метрику НЕ демотируем (НЕ false-silence и НЕ инверсия).
	dto := dipGraph(
		[]model.Node{kindNode("p.Reader", model.KindInterface), kindNode("p.DTO", model.KindConcrete)},
		[]model.Edge{{From: "p.Reader", To: "p.DTO", Type: model.EdgeReturns}},
	)

	if vs := DetectDIP(dto); len(vs) == 0 {
		t.Error("ожидался STRICT-fire на abstract->concrete-DTO (метрика не различает DTO) — иначе модель сломана")
	} else {
		t.Logf("DTO fire=%d = LEGAL FP (precision<1): whitelist держит, демотация НЕ требуется", len(vs))
	}
}
