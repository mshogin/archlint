package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// ISP usage-subset (golden id:isp) — путь B (param-typed, БЕЗ go/types).
//
// ИДЕЯ: клиентский метод/функция с параметром интерфейсного типа `Use(p I)`, который
// ВНУТРИ ТЕЛА вызывает СТРОГОЕ ПОДМНОЖЕСТВО методов I (p.Foo(), но не все методы I),
// — кандидат на «жирный интерфейс глазами клиента»: клиенту нужен уже интерфейс.
//
// СОУНДНОСТЬ через ДВА синтаксических guard'а (иначе ложно-стреляет на кондуитах):
//   - GUARD1 (i-не-форвард): p используется ТОЛЬКО как receiver вызова, никогда не в
//     value-позиции (не аргумент helper(p), не присвоение полю, не return, не p.(T)).
//     Форвард -> NO-VERDICT: значение утекает наружу, сузить тип нельзя. Факт —
//     ForwardedParams (AST-walk анализатора).
//   - GUARD2 (клиент-не-контракт): сигнатура клиентского МЕТОДА не диктуется
//     интерфейсом, который он сам реализует (implements-ребро с тем же именем метода).
//     Контракт-связан -> SUPPRESS: клиент не волен сузить param.
//
// КЛАССИФИКАЦИЯ: СВОЙ интерфейс (в графе) + клиент -> isp-usage-subset (ERROR-кандидат,
// реальный запах, сужаем). ВНЕШНИЙ интерфейс (io.* и пр., вне графа) при узком
// использовании -> isp-external-narrow (WARNING: чужой контракт, не наша вина).
//
// НАПРАВЛЕНИЕ over-approx: при ЛЮБОМ сомнении (использован метод вне известного
// множества I, неоднозначный резолв, форвард) — ВОЗДЕРЖАНИЕ (no-verdict), а не ложный
// ERROR. Критерий горнила соундности = 0 false-ERROR-fire, поэтому числитель здесь
// сознательно консервативен на ERROR-стороне.
//
// SEVERITY: Kind'ы НЕ зарегистрированы в severity_class как ERROR до прохождения
// горнила соундности (self-прогон 0 false-fire). До промоции оба — не блокирующие.
const (
	// KindISPUsageSubset — СВОЙ интерфейс, узкий клиент, не форвард, не контракт.
	// ERROR-кандидат (промоция в severity_class — после горнила соундности).
	KindISPUsageSubset = "isp-usage-subset"
	// KindISPExternalNarrow — ВНЕШНИЙ интерфейс (io.* и пр.), узкое использование.
	// Всегда WARNING (чужой контракт), никогда не ERROR.
	KindISPExternalNarrow = "isp-external-narrow"
)

// knownWideInterfaces — курируемое множество ШИРОКИХ внешних (stdlib) интерфейсов
// с известным множеством методов. Нужно, чтобы посчитать «строгое подмножество» для
// ВНЕШНЕГО интерфейса (метод-сет stdlib мы не парсим). Вне таблицы внешний интерфейс
// -> no-verdict (не можем доказать сужение). Намеренно мал и явен; только WARNING.
var knownWideInterfaces = map[string][]string{
	"io.ReadWriteCloser": {"Read", "Write", "Close"},
	"io.ReadWriter":      {"Read", "Write"},
	"io.ReadCloser":      {"Read", "Close"},
	"io.WriteCloser":     {"Write", "Close"},
	"io.ReadWriteSeeker": {"Read", "Write", "Seek"},
}

// ispClient — нормализованный вход для анализа одного клиента (метод или функция).
type ispClient struct {
	QName     string // полное имя клиента (qname)
	Pkg       string // пакет клиента
	RecvType  string // тип-получатель (для метода); "" для функции
	Name      string // имя метода/функции (для guard2)
	Params    []model.FieldInfo
	Calls     []model.CallInfo
	Forwarded []string // ForwardedParams
}

// ispVerdict — результат анализа одного (клиент, param-интерфейс).
type ispVerdict struct {
	Client string   // qname клиента
	Param  string   // имя параметра
	Iface  string   // имя интерфейса
	Used   []string // отсортированные использованные методы (в множестве I)
	Total  int      // |methods(I)|
	Own    bool     // свой интерфейс (в графе)
	Kind   string   // KindISPUsageSubset | KindISPExternalNarrow
}

// ComputeISPUsageSubset прогоняет ISP usage-subset по всем клиентам графа.
// Детерминирован: клиенты и методы сортируются перед эмиссией.
func ComputeISPUsageSubset(graph *model.Graph, a *analyzer.GoAnalyzer) []Violation {
	if a == nil {
		return nil
	}

	implementsByType := buildImplementsIndex(graph)
	ifaceMethods := buildOwnInterfaceMethods(a)

	clients := collectISPClients(a)

	var out []Violation

	for _, c := range clients {
		for _, v := range analyzeISPClient(c, ifaceMethods, implementsByType) {
			out = append(out, ispViolation(v))
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}

		return out[i].Target < out[j].Target
	})

	return out
}

// buildImplementsIndex: clientTypeID -> множество ifaceID, которые тип реализует.
func buildImplementsIndex(graph *model.Graph) map[string]map[string]bool {
	idx := make(map[string]map[string]bool)

	if graph == nil {
		return idx
	}

	for _, e := range graph.Edges {
		if e.Type != model.EdgeImplements {
			continue
		}

		if idx[e.From] == nil {
			idx[e.From] = make(map[string]bool)
		}

		idx[e.From][e.To] = true
	}

	return idx
}

// ownInterface — свой интерфейс графа: id + множество имён методов (из MethodSigs).
type ownInterface struct {
	ID      string
	Name    string
	Pkg     string
	Methods map[string]bool
}

// buildOwnInterfaceMethods индексирует свои интерфейсы по (pkg,name).
func buildOwnInterfaceMethods(a *analyzer.GoAnalyzer) map[string]ownInterface {
	res := make(map[string]ownInterface)

	for id, t := range a.AllTypes() {
		if t.Kind != "interface" || len(t.MethodSigs) == 0 {
			continue
		}

		methods := make(map[string]bool, len(t.MethodSigs))
		for _, ms := range t.MethodSigs {
			methods[ms.Name] = true
		}

		res[t.Package+"."+t.Name] = ownInterface{ID: id, Name: t.Name, Pkg: t.Package, Methods: methods}
	}

	return res
}

// collectISPClients нормализует методы и функции в общий список клиентов.
func collectISPClients(a *analyzer.GoAnalyzer) []ispClient {
	var clients []ispClient

	for id, m := range a.AllMethods() {
		if len(m.NamedParams) == 0 {
			continue
		}

		clients = append(clients, ispClient{
			QName: id, Pkg: m.Package, RecvType: m.Receiver, Name: m.Name,
			Params: m.NamedParams, Calls: m.Calls, Forwarded: m.ForwardedParams,
		})
	}

	for id, f := range a.AllFunctions() {
		if len(f.NamedParams) == 0 {
			continue
		}

		clients = append(clients, ispClient{
			QName: id, Pkg: f.Package, RecvType: "", Name: f.Name,
			Params: f.NamedParams, Calls: f.Calls, Forwarded: f.ForwardedParams,
		})
	}

	sort.Slice(clients, func(i, j int) bool { return clients[i].QName < clients[j].QName })

	return clients
}

// analyzeISPClient проверяет КАЖДЫЙ параметр клиента на ISP usage-subset.
func analyzeISPClient(
	c ispClient,
	ifaceMethods map[string]ownInterface,
	implementsByType map[string]map[string]bool,
) []ispVerdict {
	var verdicts []ispVerdict

	for _, p := range c.Params {
		v, ok := analyzeISPParam(c, p, ifaceMethods, implementsByType)
		if ok {
			verdicts = append(verdicts, v)
		}
	}

	return verdicts
}

// analyzeISPParam — ядро решения для одного (клиент, параметр). ok=false -> no-verdict.
func analyzeISPParam(
	c ispClient,
	p model.FieldInfo,
	ifaceMethods map[string]ownInterface,
	implementsByType map[string]map[string]bool,
) (ispVerdict, bool) {
	// GUARD1: параметр форвардится в value-позицию -> воздержание.
	if contains(c.Forwarded, p.Name) {
		return ispVerdict{}, false
	}

	methodSet, own, ifaceName := resolveParamInterface(c.Pkg, p, ifaceMethods)
	if methodSet == nil {
		return ispVerdict{}, false // не интерфейс / неизвестный внешний -> no-verdict
	}

	used := usedMethodsOn(c.Calls, p.Name)
	if len(used) == 0 {
		return ispVerdict{}, false // не использует методы param -> не ISP
	}

	// Любой использованный метод ВНЕ известного множества I -> модель неполна
	// (embeds/мис-атрибуция) -> воздержание (безопасная сторона, не ложный ERROR).
	for u := range used {
		if !methodSet[u] {
			return ispVerdict{}, false
		}
	}

	usedSorted := sortedKeys(used)

	// СТРОГОЕ подмножество: используется > 0 и < |methods(I)|. Использует все -> не ISP.
	if len(usedSorted) >= len(methodSet) {
		return ispVerdict{}, false
	}

	// GUARD2: клиентский МЕТОД реализует интерфейс с тем же именем -> контракт-связан
	// -> SUPPRESS. Для функций (RecvType=="") guard2 не применим.
	if c.RecvType != "" && clientIsContractBound(c, implementsByType, ifaceMethods) {
		return ispVerdict{}, false
	}

	kind := KindISPUsageSubset
	if !own {
		kind = KindISPExternalNarrow
	}

	return ispVerdict{
		Client: c.QName, Param: p.Name, Iface: ifaceName,
		Used: usedSorted, Total: len(methodSet), Own: own, Kind: kind,
	}, true
}

// resolveParamInterface возвращает множество методов интерфейса параметра, признак
// «свой» и имя интерфейса. nil -> не интерфейс / неизвестный внешний.
func resolveParamInterface(
	clientPkg string,
	p model.FieldInfo,
	ifaceMethods map[string]ownInterface,
) (methods map[string]bool, own bool, name string) {
	if p.TypePkg != "" {
		// pkg-qualified: либо известный широкий внешний (курируемая таблица), либо
		// no-verdict (своих кросс-пакетных по короткому имени пакета не резолвим).
		// TypeName уже квалифицирован ("io.ReadWriteCloser"), TypePkg — короткое имя.
		key := p.TypeName
		if ms, ok := knownWideInterfaces[key]; ok {
			set := make(map[string]bool, len(ms))
			for _, m := range ms {
				set[m] = true
			}

			return set, false, key
		}

		return nil, false, ""
	}

	// Неквалифицированный тип параметра = тип ИЗ ПАКЕТА КЛИЕНТА (Go-правило, dot-import
	// игнорируем -> безопасное воздержание). ПРЯМОЙ keyed lookup по (clientPkg.TypeName):
	// детерминирован (нет map-iteration) и пакето-корректен (одноимённый интерфейс из
	// ЧУЖОГО пакета не подставит неверный |methods(I)| -> исключён ложный ERROR и
	// недетерминизм снапшота, обязательный для дельта-инварианта).
	if oi, ok := ifaceMethods[clientPkg+"."+p.TypeName]; ok {
		return oi.Methods, true, oi.Name
	}

	return nil, false, ""
}

// usedMethodsOn собирает имена методов, вызванных на receiver==paramName.
func usedMethodsOn(calls []model.CallInfo, paramName string) map[string]bool {
	used := make(map[string]bool)

	for _, c := range calls {
		if !c.IsMethod || c.Receiver != paramName {
			continue
		}

		name := strings.TrimPrefix(c.Target, paramName+".")
		if name == "" || strings.Contains(name, ".") {
			continue // цепочка p.X.Y или пустое -> не прямой вызов метода I
		}

		used[name] = true
	}

	return used
}

// clientIsContractBound — реализует ли тип клиента интерфейс, в котором есть метод с
// именем клиентского метода (сигнатура диктуется контрактом).
func clientIsContractBound(
	c ispClient,
	implementsByType map[string]map[string]bool,
	ifaceMethods map[string]ownInterface,
) bool {
	clientTypeID := c.Pkg + "." + c.RecvType

	ifaceIDs := implementsByType[clientTypeID]
	if len(ifaceIDs) == 0 {
		return false
	}

	// id -> множество методов: переиндексируем ifaceMethods по ID.
	for _, oi := range ifaceMethods {
		if ifaceIDs[oi.ID] && oi.Methods[c.Name] {
			return true
		}
	}

	return false
}

// ispViolation формирует Violation из вердикта.
func ispViolation(v ispVerdict) Violation {
	scope := "external"
	if v.Own {
		scope = "own"
	}

	return Violation{
		Kind: v.Kind,
		Message: fmt.Sprintf(
			"client %s uses %d of %d methods of %s interface %s (via param %s: %s) — narrow it",
			v.Client, len(v.Used), v.Total, scope, v.Iface, v.Param, strings.Join(v.Used, ","),
		),
		Target: v.Client,
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}

	return false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}
