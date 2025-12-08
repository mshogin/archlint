# Task Templates

Шаблоны для создания задач с использованием Markdown + PlantUML (C4 + UML).

## Доступные шаблоны

### 1. task-template.md (Полный)
**Когда использовать:**
- Большие задачи (L/XL)
- Архитектурные изменения
- Новые модули/компоненты
- Требуется детальное проектирование

**Содержит:**
- C4 диаграммы (Context, Container, Component)
- UML диаграммы (Class, Sequence, Activity)
- Детальные требования
- План реализации по фазам
- Управление рисками
- Стратегия тестирования

### 2. task-template-simple.md (Упрощенный)
**Когда использовать:**
- Небольшие задачи (S/M)
- Рефакторинг
- Багфиксы
- Добавление функциональности

**Содержит:**
- Базовые C4/UML диаграммы
- Основные требования
- Критерии приемки
- Простой план реализации

## Как использовать

### Создание новой задачи

1. Выберите подходящий шаблон
2. Скопируйте в `tasks/todo/`
3. Переименуйте: `XXXX-task-name.md`
   - XXXX = 4-значный приоритет (0001-9999)
   - Меньше число = выше приоритет

```bash
# Пример создания задачи
cp templates/task-template.md tasks/todo/0042-implement-feature-x.md
```

4. Заполните метаданные
5. Замените все `[Placeholder]` на реальные значения
6. Обновите/удалите неиспользуемые секции

### Работа с задачей

**Todo -> InProgress:**
```bash
mv tasks/todo/0042-implement-feature-x.md tasks/inprogress/
```

**InProgress -> Done:**
```bash
mv tasks/inprogress/0042-implement-feature-x.md tasks/done/
```

## Секции шаблона

### PlantUML диаграммы

#### C4 Diagrams (Архитектура)

**Level 1: System Context**
- Показывает систему в контексте окружения
- Внешние пользователи и системы
- Граничные взаимодействия

```plantuml
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Context.puml
```

**Level 2: Container**
- Контейнеры внутри системы
- Приложения, БД, сервисы
- Технологии

```plantuml
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Container.puml
```

**Level 3: Component**
- Компоненты внутри контейнера
- Модули, пакеты
- Зависимости

```plantuml
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Component.puml
```

#### UML Diagrams (Детали реализации)

**Class Diagram**
- Структуры данных
- Интерфейсы
- Отношения между типами

```plantuml
class MyClass {
  +field: type
  --
  +method(): returnType
}
```

**Sequence Diagram**
- Взаимодействие компонентов во времени
- Порядок вызовов
- Жизненный цикл объектов

```plantuml
Actor -> Component: message
activate Component
Component -> Database: query
deactivate Component
```

**Activity Diagram**
- Процессы и алгоритмы
- Условные переходы
- Параллельное выполнение

```plantuml
start
:Action;
if (condition?) then (yes)
  :Step A;
else (no)
  :Step B;
endif
stop
```

### Markdown секции

**Overview**
- Краткое описание проблемы и решения
- Метрики успеха

**Requirements**
- Функциональные требования (FR)
- Нефункциональные требования (NFR)

**Acceptance Criteria**
- Конкретные проверяемые условия
- Чеклисты для проверки

**Implementation Plan**
- Пошаговый план реализации
- Список файлов
- Требования к тестам

**Dependencies**
- Связи с другими задачами
- Внешние зависимости

**Risks & Mitigations**
- Потенциальные риски
- Планы по снижению рисков

## Примеры из реальных проектов

### Пример 1: Большая задача (aitrader)
```
tasks/todo/0005-01-strategy-config.puml
- Рефакторинг архитектуры стратегий
- Использует PlantUML с пакетами и зависимостями
```

### Пример 2: Описательная задача (aitrader)
```
tasks/todo/0004-backtest.md
- Описание архитектуры Trade Engine
- Markdown с ASCII диаграммами
- Детальный план реализации
```

## Рекомендации

### Диаграммы
1. **Начинайте с C4**: контекст -> контейнеры -> компоненты
2. **Используйте UML для деталей**: классы, последовательности
3. **Держите диаграммы простыми**: фокус на главном
4. **Обновляйте диаграммы**: по мере реализации

### Требования
1. **Будьте конкретны**: избегайте неоднозначности
2. **Измеримые критерии**: как проверить выполнение
3. **Приоритизация**: не все требования равнозначны

### План реализации
1. **Разбивайте на шаги**: каждый шаг - 1-4 часа работы
2. **Указывайте файлы**: где будут изменения
3. **Тесты обязательны**: для каждого шага

### Общие советы
1. **Шаблон - это основа**: адаптируйте под задачу
2. **Удаляйте лишнее**: ненужные секции можно убрать
3. **Добавляйте нужное**: можно добавить специфичные секции
4. **Используйте ссылки**: на ADR, документацию, код

## Инструменты

### Просмотр PlantUML

**Online:**
- http://www.plantuml.com/plantuml/

**VS Code:**
```bash
# Установить расширение
code --install-extension jebbs.plantuml
```

**CLI:**
```bash
# Установить PlantUML
brew install plantuml

# Генерация PNG
plantuml tasks/todo/0042-task.md

# Генерация SVG
plantuml -tsvg tasks/todo/0042-task.md
```

### Проверка Markdown
```bash
# markdownlint
npm install -g markdownlint-cli
markdownlint tasks/**/*.md
```

## Конвенции именования

### Файлы задач
```
PPPP-short-description.md
```
- `PPPP` = 4-значный приоритет (0001-9999)
- `short-description` = краткое описание через дефисы
- Расширение: `.md`

### Приоритеты
- `0001-0099`: Критические задачи
- `0100-0499`: Высокий приоритет
- `0500-0899`: Средний приоритет
- `0900-9999`: Низкий приоритет

### Подзадачи
```
PPPP-XX-subtask-name.md
```
- `PPPP` = приоритет родительской задачи
- `XX` = номер подзадачи (01-99)

**Пример:**
```
0005-walk-forward-optimization.md      (родительская)
0005-01-strategy-config.md             (подзадача 1)
0005-02-tax-implementation.md          (подзадача 2)
0005-03-genetic-algorithm.md           (подзадача 3)
```

## Интеграция с Git

### Коммиты
```bash
# Создание задачи
git add tasks/todo/0042-feature-x.md
git commit -m "Add task 0042: Implement feature X"

# Начало работы
git mv tasks/todo/0042-feature-x.md tasks/inprogress/
git commit -m "Start task 0042"

# Завершение задачи
git mv tasks/inprogress/0042-feature-x.md tasks/done/
git commit -m "Complete task 0042"
```

### Ветки
```bash
# Создать ветку для задачи
git checkout -b task-0042-feature-x

# Упомянуть задачу в коммитах
git commit -m "task-0042: Add initial implementation"
```

## FAQ

**Q: Когда использовать .md vs .puml?**
A: Используйте .md (этот шаблон) для всех задач. PlantUML встроен в markdown. Чистый .puml используйте только для standalone диаграмм.

**Q: Нужно ли заполнять все секции?**
A: Нет. Удаляйте неиспользуемые секции. Для маленьких задач используйте simple шаблон.

**Q: Как обновлять диаграммы?**
A: Обновляйте PlantUML код прямо в .md файле. Пересоздавайте изображения при необходимости.

**Q: Можно ли добавлять свои секции?**
A: Да! Шаблон - это отправная точка. Адаптируйте под свои нужды.

**Q: Где хранить большие диаграммы?**
A: Если диаграмма очень большая, вынесите в отдельный .puml файл и ссылайтесь на него:
```markdown
## Architecture
См. [диаграмму архитектуры](./diagrams/0042-architecture.puml)
```
