# ADR-0001: Интеграция archmotif как поставщика архитектурных метрик в archlint

- Статус: Proposed (уточнён ADR-0002)
- ПРИМЕЧАНИЕ: механизм этого ADR (публичный export-пакет в форке archmotif +
  in-process Go-import) принят ADR-0002 как основной путь (Вариант B'). ADR-0002
  расширяет рамку (единая модель + тиринг метрик + стратегия миграции Python->Go)
  и добавляет fallback через GraphML/публичный CLI. Концепция adapter и mapping
  model.Graph -> словарь archmotif из этого ADR переиспользуется в ADR-0002 (Этап 1).

## Контекст

archlint строит доменную модель архитектуры (`internal/model.Graph`: компоненты
package/type/function/method + рёбра contains/import/calls/uses/embeds) для Go/Rust/TS
и проверяет её на нарушения. archmotif — отдельный движок, который умеет считать
графовые метрики качества архитектуры (modularity Q, motif redundancy, spectral gap,
local symmetry, cycle rank, instability) и детектировать аномалии поверх них.

Цель: archlint переиспользует расчётный аппарат archmotif вместо того, чтобы
реализовывать modularity/anomaly-детекцию заново. Подход: adapter-паттерн через
go-import (не CLI, не GraphML-файлы, не сетевой вызов).

## Ключевая находка (определяет всю архитектуру)

Форк archmotif УЖЕ имеет публичный вход `pkg/archmotifimport`:

- `archmotifimport.NewBuilder() *Builder` строит граф императивно;
- `type Graph = mgraph.Graph` — это ALIAS на `internal/graph.Graph`.

Расчётный аппарат (metrics/anomalies) лежит в `internal/` и наружу НЕ выведен. Go
запрещает чужому модулю импортировать `internal/`, НО `pkg/` того же модуля archmotif
звать свой `internal/` имеет право. Значит мост строится одним новым пакетом В ФОРКЕ —
`pkg/archmotifmetrics` — тонкой обёрткой. archlint импортирует только `pkg/*`, никакой
рефлексии, сериализации или GraphML.

Поток данных:

```
archlint model.Graph
   --[adapter: mapping]-->  archmotifimport.Builder
   --Build()-->             *archmotifimport.Graph (== *internal/graph.Graph)
   --ComputeMetrics()-->    archmotifmetrics.Metrics { Modularity, Anomalies, ... }
```

## Решение

### 1. Новый пакет в форке: `pkg/archmotifmetrics`

Тонкая обёртка над `internal/metrics` и `internal/anomalies`. EXPORTED-точки, которые
она оборачивает (сигнатуры точные):

`internal/metrics` (alias mgraph = internal/graph):
- `metrics.Run(g *mgraph.Graph, names []string) metrics.Result`
  names=nil/[] -> все зарегистрированные метрики; иначе выбор по Name().
- `metrics.Result{ Records []Record; Errors []MetricError; Ran []string }`
- `metrics.Record{ Metric string; Scope Scope; Target string; Value float64; Details map[string]any }`
  - `ScopeGraph` -> один Value на весь граф (modularity Q, spectral gap живут тут, Target=="").
  - `ScopeRegion`/`ScopeNode`/`ScopeEdge` -> Target адресует subject.
- Имена метрик (через init-регистрацию): `modularity`, `motif_redundancy`,
  `spectral_gap`, `local_symmetry`, `cycle_rank`, `instability_matrix`,
  `layer_mask`, `cycle_matrix`, `zero`.
- `metrics.Names() []string` — перечислить доступные.

`internal/anomalies`:
- `anomalies.Run(g *mgraph.Graph, records []metrics.Record, names []string) anomalies.Result`
  records берутся из `metrics.Result.Records` шага выше.
- `anomalies.Result{ Anomalies []Anomaly; Errors []DetectorError; Ran []string }`
- `anomalies.Anomaly{ Metric, Detector string; Score float64; Region Region; Reason Reason; SourceRecord SourceRecord }`
  - `Region{ Kind string; Members []string; PrimaryID string; Files []FileRef }`
  - `Reason{ Code, Message string; Details map[string]any }`
- `anomalies.Names() []string`.

Публичный контракт обёртки (предлагаемый):

```go
package archmotifmetrics

import (
    "context"
    "github.com/kgatilin/archmotif/pkg/archmotifimport"  // см. раздел про module path
)

// Metrics — плоский результат для потребителя. НЕ протекают internal-типы:
// обёртка перекладывает в собственные поля, чтобы archlint не зависел от
// internal/{metrics,anomalies}.
type Metrics struct {
    Modularity   float64           // Q из record modularity/ScopeGraph; NaN если не посчиталось
    SpectralGap  float64           // record spectral_gap/ScopeGraph
    Records      []Record          // перелож всех metrics.Record
    Anomalies    []Anomaly         // перелож всех anomalies.Anomaly
    Ran          []string          // какие метрики реально отработали
    Errors       []string          // per-metric/per-detector ошибки (не паника)
}

type Record struct { Metric, Scope, Target string; Value float64; Details map[string]any }
type Anomaly struct {
    Metric, Detector string
    Score            float64
    Code             string        // Reason.Code
    Message          string        // Reason.Message
    Members          []string      // Region.Members
    PrimaryID        string
}

// Опции выбора метрик/детекторов; nil -> всё.
type Options struct { Metrics, Detectors []string }

func ComputeMetrics(g *archmotifimport.Graph, opt Options) Metrics
func ComputeMetricsContext(ctx context.Context, g *archmotifimport.Graph, opt Options) Metrics
```

Внутри `ComputeMetrics`:
1. `res := metrics.Run(g, opt.Metrics)`
2. `anom := anomalies.Run(g, res.Records, opt.Detectors)`
3. Перелож в плоский `Metrics`; вытащить modularity/spectral_gap из ScopeGraph-записей;
   собрать Errors из обоих Result.

Важно: `*archmotifimport.Graph` и `*internal/graph.Graph` — один и тот же тип (alias),
поэтому передаётся в `metrics.Run` без приведения.

### 2. Адаптер в archlint: интерфейс `MetricsProvider` + две реализации

`internal/metrics` (новый пакет archlint) или подпакет существующего analyzer:

```go
type MetricsProvider interface {
    // Compute принимает доменную модель archlint и возвращает метрики.
    Compute(g model.Graph) (Metrics, error)
}
```

Реализация A — `archmotifProvider` (основная, go-import):
- mapping `model.Graph -> archmotifimport.Builder` (раздел 3);
- `archmotifmetrics.ComputeMetrics(built, opts)`;
- перелож в archlint-местный `Metrics`.

Реализация B — `nativeProvider` (FALLBACK):
- собственный расчёт archlint (degradation, lcom4, reach_srp в internal/mcp).
  Используется, когда archmotif недоступен или результат пуст/расошёлся.

Селектор (фабрика):
```go
func NewProvider(cfg Config) MetricsProvider // дефолт archmotif, fallback native
```
Триггер fallback: `archmotifProvider.Compute` вернул ошибку ЛИБО `len(Records)==0` /
modularity==NaN при непустом графе. `metrics.Run` ловит ошибки per-metric и НЕ паникует
(Compute по контракту чист), но матричные метрики на gonum — риск; обёртка в форке
оборачивает их `recover()` и кладёт в Errors, чтобы паника одной метрики не валила процесс.

### 3. Mapping model.Graph -> archmotifimport.Builder

Builder требует ИЕРАРХИЮ (AddType требует существующий packageID, AddMethod — parentTypeID).
archlint model.Node плоский (ID/Title/Entity), родитель выражен ребром `contains`. Поэтому
адаптер сначала реконструирует иерархию из рёбер, потом наполняет Builder в порядке
package -> type/function -> method -> field -> рёбра-зависимости.

Узлы (model.Node.Entity -> Builder):
- `package` -> `AddPackage(id, layer, aggregate)` (layer/aggregate можно пустые или из Title)
- `struct`/`type`/`trait`/`enum`/`component` -> `AddType(id, packageID, isInterface, role)`
  (isInterface=true для interface/trait; packageID — из contains-ребра)
- `function` -> `AddFunction(id, packageID)`
- `method` -> `AddMethod(id, parentTypeID)` (parentTypeID — из contains/receiver)
- `external*` (external/external_module/external_crate/external_contract) -> либо
  `AddPackage` как foreign-узел, либо пропуск (для modularity внешние обычно нужны как стоки зависимостей).

Рёбра (model.Edge.Type -> Builder):
- `contains` -> `AddContains(parentID, childID)` (ОБРАБОТАТЬ ПЕРВЫМ — даёт иерархию)
- `import` -> `AddDependency(from, to, DependencyDependsOn)`
- `calls` -> `AddDependency(from, to, DependencyCalls)`
- `uses` -> `AddDependency(from, to, DependencyUsesType)`
- `embeds` -> `AddDependency(from, to, DependencyEmbeds)` (или `AddImplements` если это
  satisfaction интерфейса — уточнить по семантике archlint embeds)

DependencyKind в `pkg/archmotifimport`: `DependencyDependsOn/Calls/CallsFrom/References/Embeds/Returns/UsesType`.

Грабли mapping:
- Builder.Add* возвращают error на дубль ID / отсутствующий parent. Адаптер ДОЛЖЕН
  идемпотентно дедуплицировать и пропускать рёбра на неизвестные узлы (а не падать).
- Порядок обязателен: все package -> все type/function -> method/field -> зависимости.
  Иначе AddType/AddMethod упадут на отсутствующем родителе.
- Узлы без contains-родителя (осиротевшие type/method) — либо синтетический package, либо drop.

### 4. Module path форка: require + replace (РЕКОМЕНДАЦИЯ)

Форк сохраняет `module github.com/kgatilin/archmotif` в go.mod (НЕ переименовывать).
В archlint go.mod:

```
require github.com/kgatilin/archmotif v0.0.0-<pseudo>
replace github.com/kgatilin/archmotif => <форк> <commit-or-branch>
```

Обоснование (trade-off):
- Вариант REPLACE (рекомендуется): import paths в коде archlint = `kgatilin/archmotif/pkg/*`,
  физически тянется форк. Upstream-sync форка ТРИВИАЛЕН — import paths внутри
  форка не трогаются, merge от upstream без конфликтов по путям. Цена: в go.mod две
  строки (require+replace). pkg/archmotifmetrics — единственный новый код в форке, конфликтовать
  с upstream почти не может.
- Вариант RENAME (`module <форк>`): go.mod archlint чище (один
  require), НО форк навсегда расходится с upstream в КАЖДОМ из ~45 internal-импортов
  -> upstream-sync становится болью (конфликт в каждом файле). Поэтому отвергнут.

Дефолт = REPLACE ради дешёвого sync. Если важен именно собственный import path в коде —
RENAME допустим, но тогда sync-цена осознанная.

## Последствия

Плюсы:
- Нулевое дублирование расчётного аппарата; archlint получает modularity/motif/anomalies даром.
- Граница чистая: archlint видит только `pkg/archmotifmetrics` + `pkg/archmotifimport`,
  internal-типы archmotif не протекают (обёртка перекладывает в плоские структуры).
- Fallback на native-метрики -> archlint не падает, если форк недоступен/разойдётся.

Минусы / риски:
- Mapping иерархии — главный источник багов (порядок, осиротевшие узлы, дубли). Покрыть тестами.
- Связь с форком: archlint зависит от стабильности `pkg/*` API форка. Контракт зафиксирован тут.
- replace-directive: `go get` archlint у третьих лиц без replace не соберётся.

## План реализации

Форк archmotif (ветка от main):
1. Создать `pkg/archmotifmetrics/metrics.go` — обёртку из раздела 1. Импортирует свои
   `internal/metrics`, `internal/anomalies`, `internal/graph`. Экспорт: `Metrics`, `Record`,
   `Anomaly`, `Options`, `ComputeMetrics`, `ComputeMetricsContext`.
2. Внутри: `metrics.Run(g, opt.Metrics)` -> `anomalies.Run(g, res.Records, opt.Detectors)` ->
   плоский `Metrics`. Вытащить modularity/spectral_gap из ScopeGraph-записей. Обернуть
   вызовы `recover()` на случай паники gonum-метрик, ошибки -> `Metrics.Errors`.
3. `pkg/archmotifmetrics/example_test.go` — построить мини-граф через archmotifimport.Builder,
   прогнать ComputeMetrics, проверить что modularity считается.
4. Коммит + push в форк.

archlint:
5. go.mod: `require github.com/kgatilin/archmotif` + `replace => <форк> <ref>`. `go mod tidy`.
6. Новый пакет (напр. `internal/archmetrics`): интерфейс `MetricsProvider`, тип `Metrics`,
   `archmotifProvider`, `nativeProvider`, фабрика `NewProvider`.
7. `archmotifProvider`: mapping `model.Graph -> archmotifimport.Builder` (раздел 3, строго
   порядок package->type/func->method->deps, идемпотентно, drop рёбер на unknown). Затем
   `archmotifmetrics.ComputeMetrics`.
8. `nativeProvider`: завернуть существующий расчёт archlint (degradation/lcom4/reach_srp).
9. Тесты mapping: дубли ID, осиротевшие узлы, fallback-триггер.
