# 符号表模块设计

## 结论

符号表模块在 Common AST（02）之上建立语义名称空间：构建作用域树、收集声明、分配稳定符号标识、把名称引用绑定到符号，并按各语言规则处理导入、可见性、遮蔽、重载、接收者与继承。模块只消费 common AST 的核心节点与语言扩展，不读取 Tree-sitter 节点，输出供统一 IR（04）、Call Graph（05）、类型层次（06）及后续所有模块复用的 `SymbolIndex`。

符号表先于统一 IR 构建：作用域与声明收集只需 common AST 的结构信息即可完成，而统一 IR 的语义规范化（符号绑定、类型关联）需要符号表结果作为输入。

模块坚持低误报底线：无法唯一解析的引用保留候选集合与失败原因，绝不为凑结果强行绑定到无依据符号。未解析、多候选、上游语法错误和暂不支持四种失败状态必须显式区分。

```text
03-symbol-table/
├── symbol.go       # 符号、符号种类与稳定 SymbolID 定义
├── scope.go        # 作用域、作用域种类与作用域树
├── reference.go    # 引用与绑定结果
├── index.go        # SymbolIndex 聚合与查询入口
├── builder.go      # 从 common AST 构建符号表的分派逻辑
├── resolve.go      # 名称解析与可见性规则
├── c.go / cpp.go / java.go / go.go / python.go   # 各语言可见性与解析规则
├── diagnostic.go   # 符号解析诊断
└── test/
    ├── unit/
    │   ├── fixture/{c,cpp,java,go,python}/{positive,negative,boundary,regression}
    │   ├── scope_test.go
    │   ├── declaration_test.go
    │   ├── resolve_test.go
    │   └── crossfile_test.go
    └── e2e/
        └── symbol_e2e_test.go
```

## 目标

- 在 common AST 之上建立项目、模块、包、命名空间、类型、函数和代码块作用域。
- 收集全部声明并分配稳定 `SymbolID`。
- 将名称引用绑定到唯一符号或候选符号集合。
- 处理导入、可见性、遮蔽、重载、接收者与继承对可见性的影响。
- 完成跨文件符号合并，保证同一逻辑符号在多文件中一致。
- 对无法唯一解析的引用保留候选与失败原因，不强行绑定。
- 保证相同输入产生确定性的符号、作用域与绑定结果。

## 非目标

- 不做完整类型推导与类型检查，只做名称解析所需的最小类型关联：记录声明处直接可见的类型引用（如 `int foo()` 的返回类型），对无显式类型标注的语言（如 Python）记录为未知，不做推导。符号种类（如 `SymbolType`）足以支撑 04 判定构造调用的基本场景；抽象类、接口、类型别名的可实例化性判定不在本模块，交由下游保守处理并保留候选。
- 不构建类型继承层次图，这是类型/类层次（06）的职责，本模块只提供继承对可见性的影响。
- 不构建 Call Graph、控制流或数据流。
- 不展开宏、不选择条件编译分支；宏与预处理来自 common AST 语言扩展。
- 不做模板与泛型的实例化求值，只记录声明与类型参数。

## 使用场景

- 查询某个名称在指定位置解析到的符号或候选集合。
- 枚举某个类型、函数或作用域内的全部声明。
- 为统一 IR 提供引用到符号的绑定，使 IR 节点可携带符号身份。
- 为类型层次提供类型声明与成员归属信息。
- 从符号或引用回溯到 common AST 节点与源码位置。

## 模块边界

### 符号表负责

- 构建作用域树并确定每个声明与引用所属作用域。
- 收集声明、生成稳定 `SymbolID`、维护符号到 AST 节点的映射。
- 按语言规则解析名称引用，输出唯一目标、候选集合或失败原因。
- 处理导入与可见性对解析的影响。
- 记录声明中直接书写的原始继承与实现关系（谁 `extends`/`implements`/嵌入谁），供可见性解析与下游使用；这是原始声明信息，不等于 06 构建的完整类型层次图与覆盖分析。
- 跨文件合并同一逻辑符号（如同一包、同一命名空间的成员）。

### 符号表不负责

- 语义规范化、desugar 与 IR 构建。
- 类型继承层次图构建与方法覆盖分析。
- 调用目标推断与虚分派解析。
- 宏展开与条件编译。

## 核心数据结构

```go
// SymbolID 是符号在当前分析任务中的稳定标识，可作为下游持久引用。
type SymbolID uint64

// SymbolKind 表示符号的种类，用于区分类型、函数、变量、字段、参数等。
type SymbolKind uint8

const (
    // SymbolPackage 表示包或模块级命名空间符号。
    SymbolPackage SymbolKind = iota + 1
    // SymbolNamespace 表示命名空间符号。
    SymbolNamespace
    // SymbolType 表示类型符号，包括类、结构体、接口、枚举和别名。
    SymbolType
    // SymbolFunction 表示自由函数或方法符号。
    SymbolFunction
    // SymbolField 表示类型的字段或成员变量符号。
    SymbolField
    // SymbolVariable 表示局部或全局变量符号。
    SymbolVariable
    // SymbolConst 表示常量符号。
    SymbolConst
    // SymbolParameter 表示函数或方法的参数符号。
    SymbolParameter
    // SymbolEnumMember 表示枚举成员符号。
    SymbolEnumMember
)

// Symbol 表示一个具名声明及其语义属性。
type Symbol struct {
    // ID 是该符号的稳定标识。
    ID SymbolID
    // Kind 是该符号的种类。
    Kind SymbolKind
    // Name 是该符号在源码中的名称。
    Name string
    // DeclNode 是声明该符号的 common AST 节点标识。
    DeclNode NodeID
    // Scope 是该符号所属的作用域标识。
    Scope ScopeID
    // TypeRef 是与该符号关联的类型引用节点，缺省时为空。
    TypeRef NodeID
    // Owner 是该符号所属的类型或容器符号，自由符号时为空。
    Owner SymbolID
    // Visibility 是该符号的可见性级别。
    Visibility Visibility
}

// ScopeID 是作用域在当前分析任务中的稳定标识。
type ScopeID uint64

// ScopeKind 表示作用域的种类。
type ScopeKind uint8

// Scope 表示一个名称可见性区域及其嵌套关系。
type Scope struct {
    // ID 是该作用域的稳定标识。
    ID ScopeID
    // Kind 是该作用域的种类，例如包、类型、函数或代码块。
    Kind ScopeKind
    // Parent 是父作用域标识，根作用域时为空。
    Parent ScopeID
    // Node 是产生该作用域的 common AST 节点标识。
    Node NodeID
    // Symbols 是在该作用域直接声明的符号标识集合。
    Symbols []SymbolID
}

// ResolveStatus 表示一次名称解析的结果状态，必须显式区分而非用零值混淆。
type ResolveStatus uint8

const (
    // ResolveResolved 表示解析到唯一目标符号。
    ResolveResolved ResolveStatus = iota + 1
    // ResolveAmbiguous 表示存在多个候选符号，无法唯一确定。
    ResolveAmbiguous
    // ResolveUnresolved 表示未找到任何候选符号。
    ResolveUnresolved
    // ResolveUpstreamError 表示上游语法或 AST 错误导致无法解析。
    ResolveUpstreamError
    // ResolveUnsupported 表示当前暂不支持该解析场景。
    ResolveUnsupported
)

// Reference 表示一次名称引用及其绑定结果。
type Reference struct {
    // Node 是产生该引用的 common AST 节点标识。
    Node NodeID
    // Name 是被引用的名称。
    Name string
    // Status 是该引用的解析状态。
    Status ResolveStatus
    // Targets 是解析得到的目标符号集合，唯一解析时长度为一。
    Targets []SymbolID
    // Reason 是解析失败或多候选时的简要原因。
    Reason string
}

// SymbolIndex 聚合一个项目的作用域、符号与引用，供下游查询。
type SymbolIndex interface {
    // Symbol 返回指定标识的符号，不存在时返回 false。
    Symbol(id SymbolID) (Symbol, bool)
    // Scope 返回指定标识的作用域，不存在时返回 false。
    Scope(id ScopeID) (Scope, bool)
    // ResolveAt 返回在指定引用节点处的名称解析结果。
    ResolveAt(node NodeID) (Reference, bool)
    // SymbolsInScope 返回指定作用域内直接声明的符号。
    SymbolsInScope(id ScopeID) []Symbol
}
```

## 对外接口

```go
// Builder 定义从 common AST 构建符号表的能力。
type Builder interface {
    // Build 消费一组同项目源文件的 common AST，构建并返回统一符号表与诊断。
    Build(ctx context.Context, units []ast.CommonAST) (SymbolIndex, []Diagnostic, error)
}
```

接口约束：

- `Build` 接收整个项目的 AST 单元集合，以完成跨文件合并与解析。
- 单个引用解析失败通过 `Reference.Status` 表达，不作为 Go `error` 返回。
- Go `error` 仅用于系统性失败，例如无有效 AST 或内部不变量破坏。
- 相同输入必须产生确定性的 `SymbolID`、`ScopeID` 分配与解析结果。
- 解析顺序与候选集合排序稳定，不依赖 map 顺序。
- 顶层入口 `BuildSymbolTable` 转发到 `Builder`，语义与批量调用等价。

## 处理流程

1. 遍历每个 common AST，建立作用域树并把声明登记到所属作用域。
2. 为每个声明分配稳定 `SymbolID`，记录种类、名称、所属作用域与声明节点。
3. 跨文件合并同一逻辑命名空间的符号，例如同包、同命名空间成员。
4. 解析导入并建立可见性视图。
5. 遍历名称引用，按语言规则在作用域链与可见性视图中查找候选。
6. 对唯一候选标记为已解析；多候选标记为歧义并保留候选；无候选标记为未解析。
7. 记录每次解析的状态与失败原因，输出 `SymbolIndex` 与诊断。

### 语言规则

- C/C++：函数重载按签名区分候选；C++ 命名空间、`using` 声明与 ADL 影响可见性；C 无重载但有 tag 名字空间。
- Java：包与 import 决定可见性；继承引入父类成员；方法重载按签名保留候选；内部类与静态导入单独处理。
- Go：包级可见性由标识符首字母大小写决定；方法按接收者归属；接口方法集用于后续满足性判断；点导入与别名导入分别处理。
- Python：LEGB 作用域链解析；`global` 与 `nonlocal` 改变绑定作用域；动态属性无法静态确定时保留未解析而非臆造。

## 错误处理

- 引用无候选返回 `ResolveUnresolved`，多候选返回 `ResolveAmbiguous`，均保留原因。
- 上游 AST 携带未映射节点或错误时返回 `ResolveUpstreamError`，不中止其余解析。
- 暂不支持的解析场景返回 `ResolveUnsupported`，显式记录待改进。
- 跨文件合并冲突（同名不同定义）必须保留全部定义并标注冲突，不静默丢弃。
- 内部不变量破坏且无法恢复时返回致命错误。

## 性能与资源限制

- 作用域与符号使用稳定标识关联，避免形成难以回收的对象引用环。
- 解析按需查询作用域链，不为每个引用复制作用域内容。
- 构建过程接受上下文取消信号。
- 对超深作用域嵌套和超大符号表设置可配置上限。
- 内置计时与 profile 埋点并可输出符号表中间摘要，默认关闭。
- 性能基准在获得真实 fixture 后确定。

## 安全考虑

- 所有输入 AST 与名称均视为不可信数据。
- 不执行、编译或加载被分析项目代码。
- 对符号爆炸与超深作用域设置资源边界，防止无界资源消耗。
- 诊断不包含与当前分析无关的环境敏感信息。

## 测试设计

- 单元测试覆盖五种语言的作用域构建、声明收集、单文件解析与跨文件解析。
- 正例：应解析的引用被正确绑定到唯一符号。
- 负例：不应产生无依据绑定，动态或不可解析引用保持未解析。
- 边界：空作用域、深层嵌套、遮蔽、匿名作用域与循环导入。
- 变体：等价语义的不同声明形式解析结果一致。
- 回归：每个已修复解析缺陷对应独立 fixture。
- 确定性：相同输入重复运行产生一致的符号与绑定。
- 语言专项：C/C++ 重载解析、Java 继承可见性、Go 接收者归属与首字母可见性、Python LEGB 与 `global`/`nonlocal`。

符号表是分析流水线的**中间步骤**，不产出漏洞报告，因此不适用端到端误报率阈值（见 AGENTS.md 第 8 条），以**正确性**口径验收：符号绑定与解析结果必须与 golden fixture 精确一致，系统失败必须为零，不得产生错误结构。正报为正确产出的绑定与解析结构，负例为不产生无依据的错误绑定或错误解析失败。最低标准为五种语言均有正报，任何结构错误都是必须修复的缺陷，目标是零错误。

## 验收标准

- 五种语言均能在 common AST 之上构建作用域树并收集声明。
- 名称解析正确区分已解析、歧义、未解析、上游错误与暂不支持五种状态。
- 跨文件符号合并正确，冲突显式保留。
- C/C++ 重载、Java 继承、Go 接收者与接口、Python LEGB 规则均有专项测试通过。
- 五种语言均有有效正报，符号绑定与解析结果与 golden fixture 精确一致，系统失败为零，无错误结构（零错误）。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确符号表职责与在 common AST、统一 IR 之间的位置。
- ✅ 完成作用域、符号、引用与解析状态的数据结构设计。
- ✅ 完成名称解析流程与各语言可见性规则设计。
- ✅ 设计单元测试、语言专项测试与误报口径。
- ❌ 创建 `03-symbol-table/` 包结构。
- ❌ 创建五种语言测试 fixture 与失败测试。
- ❌ 实现作用域构建、符号收集与名称解析。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计五种语言的正报、误报与性能基线。
