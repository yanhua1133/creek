# 类型/类层次模块设计

## 结论

类型/类层次模块在符号表（03）之上建立类型之间的继承、实现、嵌入和成员归属关系，形成类型层次图。模块为 C++ 多继承与模板、Java 类与接口继承、Go 结构体嵌入与接口满足、Python 多继承与 MRO 分别实现规则，并记录字段与方法在层次中的定义与覆盖来源。输出的 `TypeHierarchy` 供 Call Graph 精化虚调用、供后续分析判定类型关系。

模块坚持低误报：无法唯一确定的父类型或实现关系保留候选集合，不强行绑定。类型层次不反向依赖 Call Graph，仅消费符号表提供的类型声明与成员信息。

```text
06-type-hierarchy/
├── hierarchy.go    # 类型节点、关系边与稳定标识定义
├── member.go       # 成员归属与覆盖来源
├── build.go        # 从符号表构建层次的分派逻辑
├── c.go / cpp.go / java.go / go.go / python.go   # 各语言继承与满足规则
├── diagnostic.go   # 层次构建诊断
└── test/
    ├── unit/
    │   ├── fixture/{c,cpp,java,go,python}/{positive,negative,boundary,regression}
    │   ├── inherit_test.go
    │   ├── implement_test.go
    │   └── member_test.go
    └── e2e/
        └── hierarchy_e2e_test.go
```

## 目标

- 收集所有类型、接口、类和结构体声明并建立类型节点。
- 建立继承、实现、嵌入与成员归属关系。
- 记录字段、方法在层次中的定义与覆盖来源。
- 按语言规则处理多继承、接口满足与方法解析顺序。
- 对无法唯一确定的关系保留候选集合。
- 保证相同输入产生确定性的层次图。

## 非目标

- 不做完整类型推导与泛型实例化求值，只记录类型参数与约束。
- 不构建调用图或数据流。
- 不做名称解析，类型声明与成员来自符号表。
- 不展开宏或条件编译。

## 使用场景

- 查询某个类型的父类型、子类型与实现的接口。
- 查询某个方法在层次中的定义者与覆盖者。
- 为 Call Graph 提供虚调用候选精化依据。
- 从类型节点回溯到符号与源码位置。

## 模块边界

### 类型层次负责

- 建立类型节点并分配稳定标识。
- 建立继承、实现、嵌入关系边。
- 计算成员归属与覆盖来源。
- 按语言规则处理多继承与接口满足。

### 类型层次不负责

- 类型推导、泛型实例化与类型检查。
- 调用目标解析与虚分派执行。
- 名称解析与作用域构建。

## 核心数据结构

```go
// TypeNodeID 是类型节点在层次图中的稳定标识。
type TypeNodeID uint64

// RelationKind 表示类型之间关系的种类。
type RelationKind uint8

const (
    // RelationInherit 表示类继承关系。
    RelationInherit RelationKind = iota + 1
    // RelationImplement 表示接口实现关系。
    RelationImplement
    // RelationEmbed 表示结构体或类嵌入关系。
    RelationEmbed
)

// TypeNode 表示类型层次中的一个类型。
type TypeNode struct {
    // ID 是该类型节点的稳定标识。
    ID TypeNodeID
    // Symbol 是该类型对应的符号标识。
    Symbol SymbolID
    // Members 是该类型直接声明的成员符号标识集合。
    Members []SymbolID
}

// TypeRelation 表示两个类型之间的一条关系边。
type TypeRelation struct {
    // From 是关系起点类型节点标识。
    From TypeNodeID
    // To 是关系终点类型节点标识。
    To TypeNodeID
    // Kind 是该关系的种类。
    Kind RelationKind
    // Confident 标记该关系是否唯一确定，非唯一时为候选。
    Confident bool
}

// MemberOrigin 表示一个成员在层次中的定义与覆盖来源。
type MemberOrigin struct {
    // Member 是成员名称。
    Member string
    // DefiningType 是定义该成员的类型节点标识。
    DefiningType TypeNodeID
    // OverriddenFrom 是被覆盖的上层定义类型节点集合，无覆盖时为空。
    OverriddenFrom []TypeNodeID
}

// TypeHierarchy 聚合类型节点、关系与成员来源，供下游查询。
type TypeHierarchy interface {
    // Node 返回指定标识的类型节点，不存在时返回 false。
    Node(id TypeNodeID) (TypeNode, bool)
    // Supertypes 返回指定类型的直接父类型与实现的接口。
    Supertypes(id TypeNodeID) []TypeRelation
    // Subtypes 返回指定类型的直接子类型与实现者。
    Subtypes(id TypeNodeID) []TypeRelation
    // ResolveMember 返回指定类型上某成员的定义与覆盖来源。
    ResolveMember(id TypeNodeID, member string) (MemberOrigin, bool)
}
```

## 对外接口

```go
// Builder 定义从符号表构建类型层次的能力。
type Builder interface {
    // Build 消费项目符号表，构建并返回类型层次图与诊断。
    Build(ctx context.Context, symbols SymbolIndex) (TypeHierarchy, []Diagnostic, error)
}
```

接口约束：

- 单个关系无法唯一确定通过 `TypeRelation.Confident` 表达，不作为 Go `error` 返回。
- Go `error` 仅用于系统性失败。
- 相同输入产生确定性的标识分配与关系排序。
- 顶层入口 `BuildTypeHierarchy` 转发到 `Builder`。

## 处理流程

1. 从符号表收集类型声明，建立类型节点并分配标识。
2. 依据符号信息建立继承、实现、嵌入关系边。
3. 计算成员归属，识别覆盖来源。
4. 按语言规则处理多继承、接口满足与方法解析顺序。
5. 无法唯一确定的关系标记为候选并说明依据。
6. 输出类型层次图与诊断。

### 语言规则

- C：无类、继承与虚方法，仅为结构体、联合体和枚举建立类型节点并记录成员归属，不建立继承或实现关系边，也不参与虚调用精化；此时类型层次退化为类型与成员的索引。
- C++：处理多继承与虚继承，模板类记录类型参数；成员覆盖按虚函数规则识别。
- Java：处理单类继承与多接口实现；泛型记录类型参数与边界；接口默认方法记为定义来源。
- Go：结构体嵌入形成成员提升；接口满足按方法集结构化判定，无显式声明，满足关系依据方法集匹配给出。
- Python：多继承按 C3 线性化计算 MRO，覆盖来源沿 MRO 确定；动态基类无法静态确定时保留候选。

## 错误处理

- 无法唯一确定的父类型或实现关系标记为候选，不强行绑定。
- 循环继承必须检测并记录诊断，不进入无限递归。
- 符号缺失导致无法建立关系时记录诊断，不中止其余构建。
- 内部不变量破坏且无法恢复时返回致命错误。

## 性能与资源限制

- 类型节点与关系使用稳定标识关联，避免难以回收的引用环。
- 成员归属按需计算并缓存，避免重复遍历层次。
- 构建过程接受上下文取消信号。
- 对超深继承链与超宽多继承设置可配置上限。
- 内置计时与 profile 埋点并可输出层次中间摘要，默认关闭。
- 性能基准在获得真实 fixture 后确定。

## 安全考虑

- 所有输入符号信息均视为不可信数据。
- 不执行、编译或加载被分析项目代码。
- 对层次规模爆炸与循环继承设置资源边界。
- 诊断不包含与当前分析无关的环境敏感信息。

## 测试设计

- 单元测试覆盖五种语言的继承、实现、嵌入与成员覆盖。
- 正例：继承与实现关系被正确建立，覆盖来源正确。
- 负例：不产生无依据的继承或实现关系。
- 边界：循环继承、深层继承链、菱形继承与空类型。
- 变体：等价语义的不同声明形式层次一致。
- 回归：每个已修复层次缺陷对应独立 fixture。
- 确定性：相同输入产生一致的层次图。
- 语言专项：C++ 多继承与虚继承、Java 接口默认方法、Go 嵌入与接口满足、Python C3 MRO。

类型/类层次是分析流水线的**中间步骤**，不产出漏洞报告，因此不适用端到端误报率阈值（见 AGENTS.md 第 8 条），以**正确性**口径验收：继承、实现关系与覆盖来源必须与 golden fixture 精确一致，系统失败必须为零，不得产生错误结构。正报为正确产出的层次结构，负例为不产生无依据的错误关系或错误覆盖来源。最低标准为五种语言均有正报，任何结构错误都是必须修复的缺陷，目标是零错误。

## 验收标准

- 五种语言均能建立继承、实现与成员关系。
- 多继承、接口满足与 MRO 规则正确且通过专项测试。
- 无法确定的关系保留候选，循环继承被检测。
- 五种语言均有有效正报，继承、实现关系与覆盖来源与 golden fixture 精确一致，系统失败为零，无错误结构（零错误）。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确类型层次职责与不反向依赖 Call Graph 的边界。
- ✅ 完成类型节点、关系边与成员来源的数据结构设计。
- ✅ 完成层次构建流程与各语言继承、满足规则设计。
- ✅ 设计单元测试、语言专项测试与误报口径。
- ❌ 创建 `06-type-hierarchy/` 包结构。
- ❌ 创建五种语言测试 fixture 与失败测试。
- ❌ 实现类型节点、关系与成员归属构建。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计五种语言的正报、误报与性能基线。
