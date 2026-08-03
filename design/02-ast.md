# Common AST 模块设计

## 结论

Common AST 模块把 Parser 输出的、语言相关的 Tree-sitter 具体语法树（CST）转换为跨语言统一命名的抽象语法树。模块为 C、C++、Java、Go 和 Python 各实现一个独立的 CST 到 AST 转换器（每语言一个转换文件），产出一套核心节点集合加受控语言扩展的统一语法树，供符号表（03）、统一 IR（04）及所有后续模块复用。

模块遵循一条根本原则：**AST 层按语法形态保真，语义归一化交给统一 IR**。源码里出现的不同语法写法映射为不同的 AST 节点（例如 `for`、`while`、`do-while`、`foreach` 是四种独立节点），Common AST 忠实记录、不做等价变换；把多种循环归一为单一规范循环、把成员访问 `.` 与 `->` 归一等语义级归一化，全部留给 04 统一 IR。

节点集合采用**核心节点加语言扩展**的分层结构：跨五语言语义一致的通用结构进入核心节点，无法统一或语言特有的语义通过 `LanguageExtensions` 挂到最近的核心节点，不污染公共字段。核心节点偏细粒度但必须满足单一映射、正交、跨语言语义一致三条不冲突约束。

测试严格拆分为单元测试与 E2E 测试，单元测试使用项目自有最小 fixture，并额外包含跨语言等价性测试：同一语义用五种语言各写 fixture，断言核心 AST 结构一致，以此验证跨语言统一是否成立。

```text
02-ast/
├── ast.go          # 核心节点类型、NodeKind 枚举与稳定 NodeID 定义
├── node.go         # Node 只读访问接口与公共字段
├── extension.go    # 语言扩展承载结构
├── converter.go    # Converter 接口与按语言分派
├── c.go            # C 的 CST 到 common AST 转换器
├── cpp.go          # C++ 的 CST 到 common AST 转换器
├── java.go         # Java 的 CST 到 common AST 转换器
├── go.go           # Go 的 CST 到 common AST 转换器
├── python.go       # Python 的 CST 到 common AST 转换器
├── diagnostic.go   # 转换诊断
└── test/
    ├── unit/
    │   ├── fixture/
    │   │   ├── c/          # positive/negative/boundary/regression
    │   │   ├── cpp/
    │   │   ├── java/
    │   │   ├── go/
    │   │   ├── python/
    │   │   └── cross/      # 跨语言等价性 fixture，按语义分组
    │   ├── converter_test.go
    │   ├── node_test.go
    │   ├── extension_test.go
    │   └── cross_language_test.go
    └── e2e/
        └── ast_e2e_test.go
```

## 目标

- 为 C、C++、Java、Go 和 Python 各提供一个独立的 CST 到 common AST 转换器。
- 输出跨语言统一命名的核心节点集合，屏蔽 Tree-sitter grammar 的命名与结构差异。
- 忠实保留源码语法形态：语法不同即使语义等价也映射为不同节点，不在本层做归一。
- 为每个 AST 节点分配自己的稳定 NodeID，并保留到 CST 节点与源码区间的双向映射。
- 语言特有语义通过受控 `LanguageExtensions` 完整保留，不丢弃、不污染公共字段。
- 对无法映射的语法生成带源码位置的诊断，输出尽可能完整的部分 AST。
- 为符号表提供可直接遍历的声明、作用域边界与名称引用节点。
- 保证相同输入产生确定性的 AST 结构与节点顺序。

## 非目标

- 不做语义归一化：多种循环、多种分支、成员访问运算符差异等归一交给 04 统一 IR。
- 不建立作用域、不做名称解析、不做类型求值或类型关联，这些属于符号表与 IR。
- 不做 desugar、不展开宏、不选择条件编译分支。
- 不解析 import path、classpath、include path 或 Go module 依赖。
- 不直接依赖或向下游暴露 Tree-sitter 节点生命周期。
- 不承担 Call Graph、数据流或任何分析职责。

下表说明 AST 保真与 IR 归一的分工，帮助界定本模块边界：

| 源码语法形态 | Common AST（本模块，保真） | 统一 IR（04，归一） |
|---|---|---|
| `for(;;)`、`while`、`do-while`、`foreach` | `ForStmt`、`WhileStmt`、`DoWhileStmt`、`ForEachStmt` 四种独立节点 | 归一为单一规范循环 |
| `switch`、`match` | `SwitchStmt`、`MatchStmt` | 归一为规范分支 |
| `a.b`、`a->b`、`a::b` | `MemberAccess` 带运算符字段 | 归一为统一成员访问 |
| 复合赋值 `a += b` | `AssignExpr` 带复合运算符 | 展开为 `a = a + b` |

## 使用场景

- 把某个源文件的 CST 转换为统一 AST，供符号表收集声明与作用域。
- 为统一 IR 转换器提供语言无关、结构稳定的语法树输入。
- 在多语言代码库上以一致的节点词汇表遍历声明、语句和表达式。
- 从任意 AST 节点回溯到对应 CST 节点、源文件与行列范围。
- 通过语言扩展访问宏、模板、注解、装饰器等语言特有语义。

## 模块边界

### Common AST 负责

- 为五种语言分别实现 CST 到核心节点的转换。
- 维护跨语言统一的核心节点集合与 `NodeKind` 枚举。
- 生成稳定 `NodeID` 并建立到 CST 与源码区间的映射。
- 将语言特有语法归入 `LanguageExtensions`，无法归类时降级为未归类节点加扩展，不丢弃。
- 收集并输出无法映射语法的转换诊断。
- 提供确定性的只读节点遍历、子节点与字段访问能力。

### Common AST 不负责

- 循环、分支、成员访问等语义等价形式的归一化。
- 作用域构建、名称解析、类型求值与类型关联。
- 宏展开、条件编译分支选择与头文件包含。
- 依赖路径解析与跨文件符号合并。
- 直接管理 Tree-sitter C 对象生命周期，该职责由 Parser 承担。

## 核心数据结构

### 节点标识与公共字段

```go
// NodeID 是 AST 节点在当前分析任务中的稳定标识，不依赖 Tree-sitter 节点身份。
type NodeID uint64

// NodeKind 表示核心节点的语法种类，用于区分声明、语句、表达式和类型引用。
type NodeKind uint16

// Node 提供对 common AST 节点的只读访问能力。
type Node interface {
    // ID 返回该节点的稳定标识。
    ID() NodeID
    // Kind 返回该节点的核心语法种类。
    Kind() NodeKind
    // Range 返回该节点覆盖的源码范围，同时包含字节偏移和零基行列位置。
    Range() SourceRange
    // ChildCount 返回直接子节点数量。
    ChildCount() int
    // Child 返回指定位置的直接子节点。
    Child(index int) (Node, bool)
    // ChildByRole 返回指定语义角色对应的子节点，例如条件、循环体或被调用目标。
    ChildByRole(role Role) (Node, bool)
    // Extension 返回挂在该节点上的语言扩展，不存在时返回 false。
    Extension() (LanguageExtension, bool)
    // CSTRange 返回构建该节点的原始 CST 节点范围，用于回溯。
    CSTRange() SourceRange
}
```

`ChildByRole` 使用语义角色而非固定位置索引访问关键子节点，避免不同语言子节点顺序差异导致下游写死索引。`Role` 覆盖如条件、初始化、更新、循环体、被调用者、实参、接收者、返回类型等分析关键位置。

### 核心节点集合

核心节点按声明、语句、表达式、类型引用四大类组织，偏细粒度且互斥。下表给出核心节点及其主要来源语法；标注“独立”的节点仅由特定语言产生，但语义唯一、不与其他核心节点冲突。

**声明类**

| 节点 | 含义 | 主要来源语法 |
|---|---|---|
| `ImportDecl` | 引入外部依赖 | C/C++ `#include`、`using`；Java `import`；Go `import`；Python `import`/`from` |
| `PackageDecl` | 包声明 | Java `package`；Go `package` |
| `NamespaceDecl` | 命名空间 | C++ `namespace` |
| `FunctionDecl` | 函数或方法定义，接收者与所属类型记为字段 | 五语言 `func`/`def`/方法定义 |
| `ClassDecl` | 类 | C++ `class`；Java `class`/`record` |
| `StructDecl` | 结构体 | C/C++ `struct`；Go `struct` |
| `InterfaceDecl` | 接口 | Java `interface`；Go `interface` |
| `EnumDecl` | 枚举 | C/C++ `enum`；Java `enum` |
| `UnionDecl` | 联合体 | C/C++ `union` |
| `TypeAliasDecl` | 类型别名 | C `typedef`；C++ `using`/`typedef`；Go type alias；Python `TypeAlias` |
| `FieldDecl` | 字段或成员变量 | 五语言成员变量 |
| `VariableDecl` | 变量声明 | 五语言局部与全局变量 |
| `ConstDecl` | 常量声明 | C++ `const`/`constexpr`；Java `final`；Go `const`；Python `Final` |
| `ParameterDecl` | 形式参数 | 五语言参数 |
| `EnumMemberDecl` | 枚举成员 | C/C++/Java 枚举项 |

**语句类**

| 节点 | 含义 | 主要来源语法 |
|---|---|---|
| `Block` | 复合语句块 | 五语言 `{...}` 或缩进块 |
| `ExprStmt` | 表达式语句 | 五语言 |
| `VarDeclStmt` | 局部声明语句，包装声明节点 | 五语言 |
| `IfStmt` | 条件语句 | 五语言 |
| `ForStmt` | C 风格三段循环 | C/C++/Java/Go `for(;;)` |
| `ForEachStmt` | 迭代循环 | Java for-each；Go for-range；C++ range-for；Python for-in |
| `WhileStmt` | 前测循环 | C/C++/Java/Python/Go |
| `DoWhileStmt` | 后测循环 | C/C++/Java `do-while` |
| `SwitchStmt` | 值分支选择 | C/C++/Java/Go `switch` |
| `MatchStmt` | 结构化模式匹配 | Python `match` |
| `ReturnStmt` | 返回 | 五语言 |
| `BreakStmt` | 跳出循环或分支 | 五语言 |
| `ContinueStmt` | 继续下一次迭代 | 五语言 |
| `LabelStmt` | 标签 | C/C++ 标签；Java 带标签语句 |
| `GotoStmt` | 无条件跳转 | C/C++ `goto` |
| `TryStmt` | 异常处理，含 catch 与 finally 子句 | C++ `try`；Java `try`；Python `try` |
| `ThrowStmt` | 抛出异常 | C++/Java `throw`；Python `raise` |
| `WithStmt` | 资源上下文 | Python `with` |
| `DeferStmt` | 延迟执行（独立） | Go `defer` |
| `GoStmt` | 启动 goroutine（独立） | Go `go` |
| `SyncStmt` | 同步块 | Java `synchronized` 块 |

**表达式类**

| 节点 | 含义 | 主要来源语法 |
|---|---|---|
| `Literal` | 字面量，类别记为 `LiteralKind`（整数/浮点/字符串/字符/布尔/空值） | 五语言 |
| `NameRef` | 名称引用 | 五语言标识符 |
| `MemberAccess` | 成员访问，运算符记为字段（`.`/`->`/`::`） | 五语言 |
| `IndexAccess` | 下标访问 | 五语言 |
| `CallExpr` | 调用 | 五语言函数与方法调用 |
| `NewExpr` | 显式对象构造 | C++ `new`；Java `new`；Go `&T{}`/`T{}` 复合字面量构造 |
| `UnaryExpr` | 一元运算 | 五语言 |
| `BinaryExpr` | 二元运算 | 五语言 |
| `AssignExpr` | 赋值与复合赋值，复合运算符记为字段 | 五语言 |
| `TernaryExpr` | 三元条件 | C/C++/Java/Python 条件表达式 |
| `LambdaExpr` | 匿名函数或闭包 | C++ lambda；Java lambda；Go 函数字面量；Python `lambda` |
| `CastExpr` | 显式类型转换 | C/C++/Java/Go 类型转换 |
| `CompositeLiteral` | 聚合或集合字面量，类别记为字段（数组/映射/集合/元组/结构体） | Go composite literal；Java 数组初始化；Python list/dict/set/tuple |
| `ComprehensionExpr` | 推导式（独立） | Python list/dict/set/generator 推导式 |
| `AwaitExpr` | 等待异步结果（独立） | Python `await` |
| `YieldExpr` | 生成值（独立） | Python `yield` |

**类型引用类**

| 节点 | 含义 | 主要来源语法 |
|---|---|---|
| `NamedType` | 具名类型引用，含限定名 | 五语言 |
| `PointerType` | 指针类型 | C/C++ `*`；Go `*` |
| `ReferenceType` | 引用类型 | C++ `&`/`&&` |
| `ArrayType` | 定长数组类型 | C/C++/Java/Go 数组 |
| `SliceType` | 切片类型 | Go `[]T` |
| `MapType` | 映射类型 | Go `map[K]V` |
| `FunctionType` | 函数类型或函数指针 | C/C++ 函数指针；Go 函数类型 |
| `GenericType` | 泛型实例化类型 | C++ `vector<T>`；Java `List<T>`；Go 泛型实例 |
| `QualifierType` | 类型限定包装，限定符记为字段 | C/C++ `const`/`volatile` |

纯语法噪音不建立独立节点：括号表达式按决策丢弃括号层、保留内部表达式并记录源码范围；分号、逗号等分隔符不进入 AST。注释默认作为附属信息挂在最近节点，不作为独立节点。

### 代表性节点结构

以下给出保真原则下三个代表性节点，说明字段承载方式。

```go
// ForStmt 表示 C 风格三段式循环语句，忠实保留初始化、条件、更新和循环体，不做归一。
type ForStmt struct {
    // id 是该节点的稳定标识。
    id NodeID
    // init 是循环初始化子节点，缺省时为空。
    init Node
    // cond 是循环条件子节点，缺省时为空。
    cond Node
    // update 是每次迭代后的更新子节点，缺省时为空。
    update Node
    // body 是循环体子节点。
    body Node
    // rng 是该语句的源码范围。
    rng SourceRange
}

// WhileStmt 表示前测循环语句，与 ForStmt、DoWhileStmt、ForEachStmt 保持独立，归一交给统一 IR。
type WhileStmt struct {
    // id 是该节点的稳定标识。
    id NodeID
    // cond 是循环条件子节点。
    cond Node
    // body 是循环体子节点。
    body Node
    // rng 是该语句的源码范围。
    rng SourceRange
}

// MemberAccess 表示成员访问表达式，运算符差异记为字段而非拆成不同节点。
type MemberAccess struct {
    // id 是该节点的稳定标识。
    id NodeID
    // object 是被访问的对象子节点。
    object Node
    // member 是成员名称子节点。
    member Node
    // operator 记录访问运算符，取值为点、箭头或作用域解析符。
    operator MemberOperator
    // rng 是该表达式的源码范围。
    rng SourceRange
}
```

### 语言扩展承载

```go
// LanguageExtension 承载核心节点无法表达的语言特有语义，挂在最近的核心节点上。
type LanguageExtension interface {
    // Language 返回该扩展所属的编程语言。
    Language() Language
    // Kind 返回该语言特有语义的类别名称，例如宏、模板、注解或装饰器。
    Kind() string
    // Range 返回该扩展对应的源码范围。
    Range() SourceRange
}
```

各语言主要通过扩展承载的语义：

- C：宏定义与调用、预处理指令、位域、指定初始化器。
- C++：模板与特化、运算符重载、`constexpr`、结构化绑定、`co_await` 等协程语法。
- Java：注解、泛型通配符与边界、模块声明、`record` 组件的额外语义。
- Go：channel 操作与 `select`、`iota`、结构体标签、`init` 语义。
- Python：装饰器、海象运算符、星号解包、f-string 结构、类型参数语法。

## 对外接口

```go
// Converter 定义单个源文件从 CST 到 common AST 的转换能力。
type Converter interface {
    // Convert 将一个已解析的语法树转换为 common AST，并返回转换诊断。
    Convert(ctx context.Context, tree parser.SyntaxTree) (CommonAST, []Diagnostic, error)
}

// CommonAST 表示单个源文件转换得到的统一抽象语法树。
type CommonAST interface {
    // Language 返回该 AST 对应的编程语言。
    Language() Language
    // FileID 返回该 AST 对应的文件标识。
    FileID() FileID
    // Root 返回该 AST 的根节点。
    Root() Node
    // NodeByID 返回指定标识的节点，不存在时返回 false。
    NodeByID(id NodeID) (Node, bool)
}
```

接口约束：

- `Convert` 必须接受 `context.Context`，取消后尽快结束转换。
- 无法映射的单个语法通过 `Diagnostic` 表达，不作为 Go `error` 返回。
- Go `error` 仅用于不支持的语言、上游语法树无效或内部不变量破坏等系统性失败。
- 同一棵 CST 重复转换必须产生结构、节点顺序和 `NodeID` 分配完全一致的 AST。
- `NodeID` 由本模块分配，不复用 Tree-sitter 节点身份，可作为下游稳定引用。
- 节点遍历顺序固定为源码顺序，不依赖 map 顺序。

顶层构建入口 `BuildCommonAST` 按语言分派到对应 `Converter`，对批量文件的语义等价于逐文件调用 `Convert`。

## 处理流程

1. 校验上下文与上游语法树有效性，确定源文件语言。
2. 按语言分派到对应的 `Converter` 实现。
3. 自顶向下遍历 CST，按单一映射规则把每个 CST 构造转换为唯一确定的核心节点。
4. 对语言特有构造生成 `LanguageExtension` 并挂到最近核心节点；无法归类时降级为未归类节点加扩展，不丢弃。
5. 为每个核心节点分配稳定 `NodeID`，并记录到 CST 节点与源码区间的双向映射。
6. 保留语法形态差异，不合并语义等价的不同写法。
7. 对无法映射的语法生成带源码位置的诊断，继续转换其余部分。
8. 校验节点范围包含关系、子节点顺序与源码边界不变量。
9. 返回 `CommonAST`、诊断与统计信息。

同一 CST 内的转换过程必须无全局可变状态，`NodeID` 分配按源码顺序确定，保证确定性。

## 错误处理

- 上游语法树为空或已关闭时返回明确的系统错误，不 panic。
- 不支持的语言返回 `ErrUnsupportedLanguage`。
- 无法映射的语法节点生成 `DiagnosticUnmappedSyntax`，保留源码范围与 CST 节点类型，并尽量输出部分 AST。
- 上游 CST 已包含语法错误节点时，转换须跳过错误子树并记录诊断，不得中止整棵树转换。
- 内部不变量破坏且无法恢复时返回致命错误。
- 同范围重复诊断按明确规则去重。

## 性能与资源限制

- 按需构造节点包装，不为整棵 CST 建立一次性完整 Go 对象镜像。
- 转换结果不复制完整源码内容，源码切片按需从上游获取。
- 转换过程接受上下文取消信号，禁止不可中断的长时间转换。
- 对超深嵌套设置可配置递归深度上限，超限时保守终止并记录诊断。
- 内置计时与 profile 埋点，可输出结构化中间 AST 摘要用于验证功能与性能，埋点默认关闭且不影响正常转换。
- 性能基准与资源上限在获得真实 fixture 与基准数据后确定，不提前虚构指标。

## 安全考虑

- 所有输入 CST 与源码均视为不可信数据。
- 转换过程不执行、编译或加载被分析项目代码。
- 对超深语法树、超大文件和大量未映射节点设置可配置资源边界，防止 panic 或无界资源消耗。
- 不跟随不受控路径读取任意文件，源码切片只来自上游提供的文件内容。
- 诊断信息不包含与当前转换无关的环境敏感信息。

## 测试设计

### 测试分层

| 测试层 | 目录 | 输入来源 | 默认执行 |
|---|---|---|---|
| 单元测试 | `02-ast/test/unit/` | 项目自有最小 fixture 与跨语言等价 fixture | 是 |
| E2E 测试 | `02-ast/test/e2e/` | 复用 01 的真实项目 submodule | 否，使用 `e2e` build tag |

### 单元测试覆盖

- 五种语言的声明、语句、表达式和类型引用核心节点映射。
- 保真验证：`for`、`while`、`do-while`、`foreach` 分别映射为四种独立节点，不被合并。
- 单一映射验证：同一 CST 构造只映射到唯一核心节点，无二义。
- 语言扩展：宏、模板、注解、channel、装饰器等语义完整挂载且不污染公共字段。
- 未映射语法生成诊断并输出部分 AST。
- 纯语法噪音处理：括号层丢弃但保留内部表达式与源码范围。
- 节点源码范围包含关系、子节点顺序与角色访问正确。
- 空文件、仅注释文件、超深嵌套等边界情况。
- 确定性：相同 CST 重复转换产生一致的结构、节点顺序与 `NodeID`。

### 跨语言等价性测试

在 `fixture/cross/` 下按语义分组组织 fixture，每组用五种语言各写一个等价程序，断言核心 AST 结构一致。初始语义分组至少包括：

- 带一个参数的函数定义，函数体含一次条件判断与一次调用。
- 一个类型定义，含一个字段与一个方法。
- 一次 while 循环遍历并累加。
- 一次成员访问后的方法调用。

跨语言测试只比较核心节点结构，允许语言扩展与命名差异；核心结构不一致即判定跨语言统一失败。

### 正报与误报口径

- 正报：应识别的核心节点被正确映射，且节点范围与 CST 一致。
- 误报：合法源码的核心结构被错误映射为不同节点或产生非预期诊断。
- 漏报：应映射的核心结构未生成对应节点。
- 系统失败：转换 panic、无法输出 AST 或生命周期错误，直接导致测试失败。

误报率按文件统计：产生非预期节点映射或非预期诊断的合法文件数除以合法文件总数。最低标准为五种语言均有正报、总体及分语言误报率低于 30%，且不得通过删除困难 fixture 降低标准。

## 验收标准

- C、C++、Java、Go 和 Python 均有独立 CST 到 common AST 转换器。
- 五种语言的核心节点集合统一命名，下游无需读取 Tree-sitter 节点即可遍历。
- 保真原则得到验证：语义等价但语法不同的写法保留为不同节点。
- 单一映射、正交、跨语言语义一致三条不冲突约束在测试中得到验证。
- 语言特有语义通过 `LanguageExtensions` 完整保留，无公共字段污染。
- 所有节点具备稳定 `NodeID` 与到 CST、源码的双向映射。
- 跨语言等价性测试对每个语义分组断言核心结构一致。
- 五种语言均有有效正报，系统失败为零，总体及分语言误报率低于 30%。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确 Common AST 模块职责与 AST 保真、IR 归一的分工。
- ✅ 确定核心节点加语言扩展的分层结构与不冲突约束。
- ✅ 完成核心节点集合、接口、处理流程与错误处理设计。
- ✅ 设计单元测试、跨语言等价性测试与误报口径。
- ❌ 创建 `02-ast/` 包结构与节点类型定义。
- ❌ 创建五种语言的单元测试 fixture 与跨语言等价 fixture。
- ❌ 编写并确认失败测试。
- ❌ 实现五种语言的 CST 到 common AST 转换器。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计五种语言的正报、误报、漏报与性能基线。
