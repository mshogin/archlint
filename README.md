# archlint

Инструмент для построения архитектурных графов из исходного кода на Go.

archlint позволяет автоматически извлекать и визуализировать архитектуру программных систем в виде двух типов графов:
- **Структурный граф** — статический анализ кода, показывает все компоненты и связи
- **Граф поведения** — динамический анализ через трассировку, показывает реальные потоки выполнения

## Возможности

- ✅ Построение структурных графов из Go кода
- ✅ Генерация графов поведения из трассировок тестов
- ✅ Экспорт в формат DocHub YAML
- ✅ Автоматическая генерация PlantUML sequence диаграмм
- ✅ Поддержка wildcards для группировки компонентов

## Установка

### Из исходников

```bash
git clone https://github.com/mshogin/archlint
cd archlint
make install
```

Это установит `archlint` в `$GOPATH/bin`.

### Сборка

```bash
make build
```

Бинарник будет создан в `bin/archlint`.

## Использование

### 1. Построение структурного графа

Анализирует исходный код и строит граф всех компонентов (пакеты, типы, функции, методы) и их зависимостей.

```bash
archlint collect . -o architecture.yaml
```

**Пример вывода:**
```
Анализ кода: . (язык: go)
Найдено компонентов: 95
  - package: 5
  - struct: 23
  - function: 30
  - method: 21
  - external: 15
Найдено связей: 129
✓ Граф сохранен в architecture.yaml
```

**Структура графа:**
```yaml
components:
  cmd/archlint:
    title: main
    entity: package
  cmd/archlint.main:
    title: main
    entity: function
  internal/analyzer.GoAnalyzer:
    title: GoAnalyzer
    entity: struct

links:
  cmd/archlint:
    - to: cmd/archlint.main
      type: contains
  cmd/archlint.main:
    - to: internal/analyzer.NewGoAnalyzer
      type: calls

contexts:
  cmd:
    title: cmd
    location: Architecture/cmd
    components:
      - cmd/archlint
      - cmd/archlint.main
```

### 2. Построение графа поведения

Генерирует контексты из трассировок тестов, показывая реальные потоки выполнения.

**Шаг 1:** Добавьте трассировку в ваши тесты:

```go
import "github.com/mshogin/archlint/pkg/tracer"

func TestProcessOrder(t *testing.T) {
    trace := tracer.StartTrace("TestProcessOrder")
    defer func() {
        trace.Save("traces/test_process_order.json")
    }()

    // Трассируемая функция
    tracer.Enter("OrderService.ProcessOrder")
    result, err := service.ProcessOrder(order)
    tracer.Exit("OrderService.ProcessOrder", err)

    // assertions...
}
```

**Шаг 2:** Запустите тесты:

```bash
go test -v ./...
```

**Шаг 3:** Генерируйте контексты:

```bash
archlint trace ./traces -o contexts.yaml
```

**Результат:**
- `contexts.yaml` — контексты для DocHub
- `*.puml` — PlantUML sequence диаграммы для каждого теста

### 3. Использование Makefile

```bash
# Показать справку
make help

# Собрать проект
make build

# Построить граф для самого archlint
make collect

# Форматирование кода
make fmt

# Запустить тесты
make test

# Очистить сгенерированные файлы
make clean
```

## Структура проекта

```
archlint/
├── cmd/
│   └── archlint/          # CLI приложение
│       ├── main.go        # Точка входа
│       ├── collect.go     # Команда collect
│       └── trace.go       # Команда trace
├── internal/
│   ├── model/             # Модель графа
│   │   └── model.go       # Graph, Node, Edge, DocHub
│   └── analyzer/          # Анализаторы кода
│       └── go.go          # GoAnalyzer (AST парсинг)
├── pkg/
│   └── tracer/            # Библиотека трассировки
│       ├── trace.go       # Сбор трассировок
│       └── context_generator.go  # Генератор контекстов
├── go.mod
├── Makefile
└── README.md
```

## Примеры

### Анализ собственного проекта

archlint использует сам себя в качестве примера:

```bash
make collect
```

Результат: `graph/architecture.yaml` с полным графом проекта.

### Интеграция с DocHub

Сгенерированные YAML файлы совместимы с [DocHub](https://dochub.info/):

```yaml
# dochub.yaml
contexts:
  $imports:
    - architecture.yaml
    - contexts.yaml
```

## Формат данных

### Структурный граф

- **Nodes (components)**: компоненты системы
  - `package` — Go пакеты
  - `struct` — структуры
  - `interface` — интерфейсы
  - `function` — функции
  - `method` — методы
  - `external` — внешние зависимости

- **Edges (links)**: связи между компонентами
  - `contains` — вхождение (пакет содержит тип)
  - `calls` — вызов функции/метода
  - `uses` — использование типа в поле
  - `embeds` — встраивание типа
  - `import` — импорт пакета

### Граф поведения

- **Trace**: трассировка выполнения теста
  - `test_name` — имя теста
  - `calls` — последовательность вызовов
    - `event`: "enter" | "exit_success" | "exit_error"
    - `function` — имя функции
    - `depth` — уровень вложенности

## Связь с aiarch

archlint содержит только функционал построения графов из проекта [aiarch](https://github.com/mshogin/aiarch).

**Что НЕ входит в archlint:**
- Валидация графов
- Метрики качества (fan-out, coupling, etc.)
- Проверка архитектурных правил

Для валидации и метрик используйте [aiarch](https://github.com/mshogin/aiarch).

## Лицензия

MIT

## Контакты

- GitHub: https://github.com/mshogin/archlint
- Связанный проект: https://github.com/mshogin/aiarch
