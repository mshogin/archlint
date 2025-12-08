# Название продукта/проекта/фичи

## Проблема 
<описание проблемы>

## Решение
<описание решения>

## Бизнес анализ
### Бизнес процесс
<ссылка на BPMN диаграмму>

### Бизнес требования
<список требований>

### Бизнес правила
<список правил>

## Системный анализ
### Функциональные требования
<списко функциональных требований>

### C4 Контекст
```markdown
@startuml
!theme toy
!include <C4/C4_Context>

title <ADRname>: C4 Context

System_Boundary(arch_system, "Архитектура"){
    Person(analyst, "Аналитик")
    System(arch,"Архитектурный комитет")
    System(git, "Git", "wbjobs/common/arch")
    Lay_D(analyst, git)
    Lay_R(analyst, arch)
}

Person(lead, "Техлид")

Lay_L(lead,arch_system)

Rel(lead,git,"Сохраняет и читает ADR")
Rel(analyst,git,"Сохраняет и читает ADR")
Rel(arch,git,"Валидирует решение")

SHOW_LEGEND()
@enduml
```


### C4 Контейнеры
```plantuml
@startuml
!theme toy
!include <C4/C4_Container>

title <ADRname>: C4 Containers

System_Boundary(System_Boundary, "System_Boundary"){
    Person(Person, "Person", "Внутренний пользователь")
    Container(Container, "Container", "Базовый контейнер")
    Lay_U(Person, Container)
    ContainerDb(ContainerDb, "ContainerDb", "База данных")
    Lay_R(Container, ContainerDb)
    ContainerQueue(ContainerQueue, "ContainerQueue", "Очередь")
    System(system, "system", "Система")
    Rel(Person, Container, "Обращается")

}

Container_Boundary(Container_Boundary, "Container_Boundary"){
    Person_Ext(Person_Ext, "Person_Ext", "Внешний пользователь")
    Container_Ext(Container_Ext, "Container_Ext", "Базовый контейнер (внешний)")
    ContainerDb_Ext(ContainerDb_Ext, "ContainerDb_Ext", "База данных (внешняя)")
    ContainerQueue_Ext(ContainerQueue_Ext, "ContainerQueue_Ext", "Очередь (внешняя)")
    System_Ext(system_ext, "system_ext", "Система внешняя")
}

SHOW_LEGEND()

@enduml
```


## Диаграмма последовательности
```plantuml
@startuml
!theme toy
skinparam stereotypePosition bottom
skinparam Maxmessagesize 400
skinparam sequenceMessageAlign center

title <ADRname>: Sequence

Actor "Техлид" as lead

participant "git" as git << wbjobs/common/arch >> #00aaff

Actor "Аналитик" as analyst

participant "Встреча 'Арх.Комитет'" as arch #00aaff

participant "Band" as band << Архитектурный комитет >> #00aaff

'boundary "Boundary" as boundary1 << Класс-Разграничитель >>
'используется для классов, отделяющих внутреннюю структуру системы от внешней среды (экранная форма, пользовательский интерфейс, устройство ввода-вывода). Объект со стереотипом < отличается от, привычного нам, класса <<эИнтерфейс>>, который по большей части предназначен для вызова методов класса, с которым он связан. Объект boundary показывает именно экранную форму, которая принимает и передает данные обработчику

'control "Control" as control1 << Класс-контроллер >>
'активный элемент, который используются для выполнения некоторых операций над объектами (программный компонент, модуль, обработчик)

'entity "Entity" as entity1 << Класс-сущность >>
'обычно применяется для обозначения классов, которые хранят некую информацию о бизнес-объектах (соответствует таблице или элементу БД)

'database "Database" as database1 << БД >>

'collections "Collections" as collections1 << Группа объектов >>

'queue "Queue" as queue1 << Очередь сообщений >>

group "Техлид готовит черновик" "Дополнительное пояснение"
    == Подготовка ==
    lead -> git: Создает свою ветку от master
    lead -> git: Сохраняет черновик ADR
    note right
        Образец - '000x-ADR-template':
        * ADR
        * Context
        * Containers
        * Sequence
        * При необходимости - другие диаграммы (state, ...)
    end note


== Merge request ==
    lead -> git: Делает MR из своей ветки в master
    note right: Указывает аналитика в качестве reviewer

== Согласование с аналитиком ==
    lead -> analyst: Сообщает о готовности черновика ADR
    note right: Название git-ветки
    analyst -> band: Создает тред для обсуждения ADR
    analyst -> git: Читает черновик ADR
    alt Есть замечания?
        analyst -> git: Создает новую ветку от master
        analyst -> git: Предлагает изменения в своей ветке
        analyst -> git: Делает MR из своей ветки в master
        note right
            Указывает техлида
            в качестве reviewer

            При необходимости
            обсуждают ADR в Band
        end note
    end
end

group "Техлид согласовывает решение на архитектурном комитете"
== Подготовка ==
lead -> arch: Сообщает о желании презентовать ADR
arch -> arch: Включает ADR в повестку\nодной из регулярных встреч
== Презентация ==
lead -> arch: Показывает ADR и рассказывает о ней
lead <-- arch: Дают замечания
lead -> git: Новая версия ADR\nс учетом замечаний
end

/'
== Пример разных сообщений и пометок ==
group Пример разных сообщений и пометок
    lead -> git: Раз
    note right
        Пояснение
        на несколько
        строк
    end note
    git -> analyst: Два
    alt Все хорошо?
    git <-- analyst: Три. Ответное сообщение\nпунктиром
    git --> lead : Четыре
    else Все плохо?
        git <-- analyst !!: Не три
        git --> lead !!: И не четыре
    end
end
'/

@enduml
```
