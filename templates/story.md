# Story: [Название Story]

**Metadata:**
- Epic: [Ссылка на Epic]
- Priority: [High/Medium/Low]
- Status: [Todo/InProgress/Done]
- Effort: [XS/S/M/L/XL]

---

## Use Case
As a [роль пользователя]
I want to [действие]
So that [цель/выгода]

---

## Описание
[Детальное описание use case]

### Предусловия
- [Предусловие 1]
- [Предусловие 2]

### Основной сценарий
1. [Шаг 1]
2. [Шаг 2]
3. [Шаг 3]
4. [Результат]

### Альтернативные сценарии
**Сценарий A:** [название]
- [Описание альтернативного пути]

---

## Решение
[Техническое описание решения]

### Компоненты
- [Компонент 1]: [что делает]
- [Компонент 2]: [что делает]

### Изменения в коде
- [Файл 1]: [изменения]
- [Файл 2]: [изменения]

---

## Acceptance Criteria
- [ ] AC1: [Конкретный проверяемый критерий]
- [ ] AC2: [Конкретный проверяемый критерий]
- [ ] AC3: [Конкретный проверяемый критерий]

---

## Tasks
- [Task 1 - XXXX-task-name.md]
- [Task 2 - YYYY-task-name.md]

---

## Примеры для archlint

**Story: Обнаружение циклических зависимостей**

Use Case:
As a Go developer
I want to detect circular dependencies in my codebase
So that I can maintain clean architecture

Решение:
- Реализовать Tarjan's algorithm в pkg/analyzer/graph/cycles.go
- Добавить CLI команду: archlint analyze --cycles
- Вывести список всех найденных циклов

Acceptance Criteria:
- AC1: Обнаруживает простые циклы (A->B->A)
- AC2: Обнаруживает сложные циклы (A->B->C->A)
- AC3: CLI выводит понятный отчет
