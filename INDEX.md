# ArchLint - INDEX

Обновлено: 2026-02-10

---

## Описание

Инструмент для построения архитектурных графов из исходного кода Go. Автоматически извлекает и визуализирует архитектуру программных систем через структурные графы (статический анализ) и поведенческие графы (динамический анализ через трассировку). Экспортирует в DocHub YAML и PlantUML sequence диаграммы.

## Структура проекта

| Путь | Описание |
|------|----------|
| cmd/archlint/ | CLI приложение archlint (точка входа + команды collect/trace) |
| cmd/tracelint/ | Инструмент проверки трассировок (linter для трейсов) |
| internal/cli/ | Реализация команд CLI (root, collect, trace, validate) |
| internal/analyzer/ | Анализаторы кода (Go AST парсинг) |
| internal/linter/ | Лог-линтеры (tracelint реализация) |
| internal/model/ | Модель графа (Graph, Node, Edge, DocHub структуры) |
| pkg/tracer/ | Библиотека трассировки для инструментирования кода |
| tests/ | Интеграционные тесты (fullcycle_test.go) |
| tests/testdata/ | Тестовые данные |
| specs/ | Спецификации (todo/inprogress/done) |
| templates/ | Шаблоны (ADR, project-handover, specifications, system-audit) |
| validator/ | Python валидаторы для структурных и поведенческих графов |
| arch/ | Сгенерированные архитектурные графы (architecture.yaml) |
| bin/ | Бинарники archlint и tracelint |

---

## Ключевые файлы

| Файл | Описание |
|------|----------|
| README.md | Основная документация на русском (с примерами использования и API) |
| README.en.md | Английская версия документации |
| VALIDATION.md | Спецификация формата валидации графов (rules, contexts, components) |
| VALIDATION.en.md | Английская версия VALIDATION.md |
| Makefile | Команды разработки (build, collect, trace, fmt, test, lint, clean, implement) |
| go.mod | Зависимости: cobra, golang.org/x/tools, yaml.v3 |
| go.sum | Контрольные суммы зависимостей |
| .archlint.yaml | Конфиг самого archlint (используется для собственного анализа) |
| .golangci.yml | Конфиг golangci-lint проверок |
| .claude/ | Контекст для Claude Code (contribution.md для /make implement) |

---

## Основные команды

```
make help       - Показать справку
make build      - Собрать проект
make collect    - Построить структурный граф для archlint
make test       - Запустить тесты
make lint       - Проверить код (golangci-lint + tracelint)
make fmt        - Форматировать код
make clean      - Очистить сгенерированные файлы
make implement  - Реализовать спецификацию (Claude Code)
```

---

## Модули и зависимости

**Точка входа:** cmd/archlint/main.go
**Go версия:** 1.25.1
**Зависимости:**
- github.com/spf13/cobra - CLI фреймворк
- golang.org/x/tools - Работа с Go AST и анализом кода
- gopkg.in/yaml.v3 - YAML сериализация (DocHub формат)

---

## Workflow спецификаций

1. Спецификация создается в specs/todo/
2. Claude Code берет в работу через `make implement`
3. Переносит в specs/inprogress/ и коммитит
4. Реализует описанные изменения
5. Перемещает в specs/done/ после завершения

---

## Связанные проекты

- **aiarch** (https://github.com/mshogin/aiarch) - Валидация графов и метрики архитектуры (fan-out, coupling, и т.д.)
- **dochub** - Интеграция для визуализации и документирования архитектуры
