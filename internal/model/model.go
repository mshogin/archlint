// Package model содержит структуры данных для представления архитектурного графа.
package model

// Entity kind constants.
const (
	EntityPackage   = "package"
	EntityStruct    = "struct"
	EntityInterface = "interface"
	EntityFunction  = "function"
	EntityMethod    = "method"
	EntityField     = "field"
	EntityExternal  = "external"
)

// Edge type constants.
const (
	EdgeContains   = "contains"
	EdgeImport     = "import"
	EdgeCalls      = "calls"
	EdgeUses       = "uses"
	EdgeEmbeds     = "embeds"
	EdgeFieldRead  = "field_read"
	EdgeFieldWrite = "field_write"
	// EdgeImplements — concrete type -> interface (method-set сатисфакция с
	// embeds-промоушеном). Материализуется в Фазе 1 (ADR-0002); нужен DIP/dead-code.
	EdgeImplements = "implements"
	// EdgeReturns — функция/метод -> тип в сигнатуре ВОЗВРАТА (type-flow, Фаза 1).
	EdgeReturns = "returns"
	// EdgeReferences — функция/метод используется как ЗНАЧЕНИЕ (callback), Фаза 1.
	EdgeReferences = "references"
)

// Type-kind значения для Node.Attrs["kind"] (ось абстрактности для DIP).
const (
	KindInterface = "interface"
	KindConcrete  = "concrete"
)

// Graph представляет архитектурный граф.
type Graph struct {
	Nodes []Node `yaml:"components"`
	Edges []Edge `yaml:"links"`
	// Attrs — граф-уровневые атрибуты (как nx.DiGraph.graph): entry_point, stats для
	// callgraph-формата. In-memory (не часть archlint-схемы YAML). Аддитивно.
	Attrs map[string]any `yaml:"-" json:"graphAttrs,omitempty"`
}

// Node представляет узел графа (компонент).
// Attrs/μ — property-graph мешок атрибутов (ADR-0002 Этап 1). Для type-узлов
// несёт "kind"=interface|concrete (нужен DIP). omitempty -> старые потребители не ломаются.
type Node struct {
	ID     string         `yaml:"id"`
	Title  string         `yaml:"title"`
	Entity string         `yaml:"entity"`
	Attrs  map[string]any `yaml:"attrs,omitempty"`
}

// Edge представляет ребро графа (связь между компонентами).
// Attrs/μ — property-graph мешок атрибутов ребра (ADR-0002 Этап 1).
type Edge struct {
	From   string         `yaml:"from"`
	To     string         `yaml:"to"`
	Method string         `yaml:"method,omitempty"`
	Type   string         `yaml:"type,omitempty"`
	Attrs  map[string]any `yaml:"attrs,omitempty"`
}

// TypeInfo содержит информацию о типе (struct/interface).
type TypeInfo struct {
	Name       string
	Package    string
	Kind       string
	File       string
	Line       int
	Fields     []FieldInfo
	Embeds     []string
	Implements []string
	// MethodSigs — методы, объявленные В ИНТЕРФЕЙСЕ (Kind=="interface"), с ПОЛНОЙ
	// сигнатурой (имя + param/return type-refs). Имена -> method-set implements;
	// param/return -> usesType/returns ОТ интерфейса (DIP: абстракция ссылается на
	// конкрет в сигнатуре своего метода). Для struct пусто. Фундаментальный факт
	// для DIP и будущего signature-точного implements.
	MethodSigs []InterfaceMethodSig
}

// InterfaceMethodSig — сигнатура одного метода интерфейса (без тела).
type InterfaceMethodSig struct {
	Name    string
	Params  []FieldInfo
	Results []FieldInfo
}

// FieldInfo содержит информацию о поле структуры.
type FieldInfo struct {
	Name     string
	TypeName string
	TypePkg  string
}

// FunctionInfo содержит информацию о функции.
type FunctionInfo struct {
	Name    string
	Package string
	File    string
	Line    int
	Calls   []CallInfo
	// Params/Results — type-refs из СИГНАТУРЫ (Фаза 1): usesType покрывает param-типы,
	// returns — типы возврата. FieldInfo.Name опционален (для type-ref не важен).
	Params  []FieldInfo
	Results []FieldInfo
	// Refs — функция/метод использован как ЗНАЧЕНИЕ (callback, Фаза 1). Резолв-фильтр
	// в билдере оставит только реальные функции -> references-ребро.
	Refs []CallInfo
	// ForwardedParams — ОТСОРТИРОВАННОЕ множество имён параметров, появляющихся в
	// VALUE-позиции в теле (аргумент чужого вызова helper(p), присвоение полю, return,
	// type-assert) — т.е. НЕ только как receiver вызова p.Foo(). Синтаксический факт
	// (AST-walk, over-approx по имени), фундамент ISP-guard1: если интерфейсный
	// параметр форвардится, ISP воздерживается (no-verdict) — сужать «свой» интерфейс
	// нельзя, если значение утекает наружу.
	ForwardedParams []string
	// NamedParams — ИМЕНОВАННЫЕ параметры (имя+type-ref), по одному на каждое имя
	// (`a, b I` -> два элемента). Отдельно от Params (без имён, для usesType):
	// ISP-числителю нужен маппинг имя_параметра -> интерфейсный тип. Аддитивно,
	// существующих потребителей Params не затрагивает.
	NamedParams []FieldInfo
}

// MethodInfo содержит информацию о методе.
type MethodInfo struct {
	Name        string
	Receiver    string
	Package     string
	File        string
	Line        int
	Calls       []CallInfo
	FieldAccess []FieldAccessInfo
	// Params/Results — type-refs из СИГНАТУРЫ метода (Фаза 1): usesType/returns.
	// Ключ DIP: у интерфейса нет тела -> без param-типов DIP молча пропустит
	// param-нарушения (самый частый вектор).
	Params  []FieldInfo
	Results []FieldInfo
	// Refs — функция/метод как ЗНАЧЕНИЕ (callback, Фаза 1) -> references-ребро.
	Refs []CallInfo
	// ForwardedParams — см. FunctionInfo.ForwardedParams. Имена параметров метода в
	// value-позиции (фундамент ISP-guard1).
	ForwardedParams []string
	// NamedParams — см. FunctionInfo.NamedParams. Именованные параметры метода
	// (имя+тип) для ISP-числителя.
	NamedParams []FieldInfo
	// HasControlFlow — в теле метода есть управляющая логика (if/for/range/switch/select).
	// Структурный признак ПОВЕДЕНИЯ (не аксессор) для DIP DTO-фильтра: behavioral(C) =
	// ∃ метод C с control-flow ИЛИ внешними вызовами; DTO = только аксессоры (нет ни того, ни
	// другого) -> ссылка на DTO не DIP-дефект (словарь абстракции, не поведенческая деталь).
	HasControlFlow bool
}

// FieldAccessInfo contains information about a field access within a method.
type FieldAccessInfo struct {
	// FieldName is the bare field name (e.g. "Name").
	FieldName string
	// IsWrite is true when the field is on the LHS of an assignment,
	// increment/decrement, or its address is taken.
	IsWrite bool
	// Line is the source line of the access.
	Line int
}

// CallInfo содержит информацию о вызове.
type CallInfo struct {
	Target      string
	IsMethod    bool
	Receiver    string
	Line        int
	IsGoroutine bool
	IsDeferred  bool
}
