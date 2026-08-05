# Call Graph 模块设计

## 结论

Call Graph 模块在统一 IR（04）与符号表（03）之上建立调用关系图。节点表示可调用实体，边表示调用点到候选目标的关系。直接调用优先精确解析；C/C++ 函数指针与虚调用、Java 虚方法、Go 接口调用、Python 一等函数与动态属性调用保留有依据的候选集合。未解析调用必须显式记录，不得静默丢失或绑定到无依据目标。

模块坚持低误报：每条边保留调用点、解析方式、置信状态与候选来源；无依据时保持未解析，而不是猜测目标。类型层次（06）在本模块之后构建，因此本模块的虚调用候选只依据符号表（03）记录的**原始继承与实现声明**（谁 extends/implements 谁）给出保守候选集合，并一律标注为 `ConfidenceCandidate`，不做覆盖分析、不判定唯一目标。06 构建完成后可作为后处理对这些候选边做精化（缩小或补全覆盖者），该精化是单向后处理，不构成本模块对 06 的反向构建依赖。

```text
05-call-graph/
├── graph.go        # 调用图节点、边与稳定标识定义
├── callable.go     # 可调用实体识别
├── callsite.go     # 调用点收集
├── resolve.go      # 调用目标解析与候选生成
├── c.go / cpp.go / java.go / go.go / python.go   # 各语言动态调用规则
├── diagnostic.go   # 调用解析诊断
└── test/
    ├── unit/
    │   ├── fixture/{c,cpp,java,go,python}/{positive,negative,boundary,regression}
    │   ├── direct_test.go
    │   ├── dynamic_test.go
    │   └── unresolved_test.go
    └── e2e/
        └── callgraph_e2e_test.go
```

## 目标

- 从 IR 收集全部可调用实体与调用点。
- 精确解析直接函数与静态方法调用。
- 为动态调用生成有依据的候选边并标注解析方式。
- 显式记录未解析调用及原因。
- 对构造、初始化与隐式调用建立明确规则。
- 保证相同输入产生确定性的节点、边与候选排序。

## 非目标

- 不做完整虚分派精化，依赖类型层次的增强单独进行。
- 不做数据流、指针分析或污点传播。
- 不推断运行时反射与完全动态生成的调用目标，这类保持未解析。
- 不展开宏或条件编译。

## 使用场景

- 查询某个函数的调用者与被调用者。
- 枚举某个调用点的候选目标及置信状态。
- 为污点与符号执行提供跨函数调用边。
- 从调用边回溯到 IR 调用节点与源码位置。

## 模块边界

### Call Graph 负责

- 识别可调用实体并分配稳定节点标识。
- 收集调用点并解析目标。
- 生成已解析边、候选边与未解析记录。
- 按语言规则处理动态调用候选。

### Call Graph 不负责

- 类型继承层次构建与虚方法覆盖分析。
- 数据流、指针与污点分析。
- 名称解析与类型关联，这些来自符号表与 IR。

## 核心数据结构

```go
// CallableID 是可调用实体在调用图中的稳定标识。
type CallableID uint64

// CallEdgeKind 表示调用边的解析方式。
type CallEdgeKind uint8

const (
    // CallDirect 表示精确解析的直接调用。
    CallDirect CallEdgeKind = iota + 1
    // CallVirtual 表示基于候选的虚方法或接口调用。
    CallVirtual
    // CallIndirect 表示通过函数指针或一等函数的间接调用。
    CallIndirect
    // CallConstructor 表示构造或初始化触发的调用。
    CallConstructor
)

// ResolveConfidence 表示一条调用边的置信状态。
type ResolveConfidence uint8

const (
    // ConfidenceResolved 表示唯一确定的目标。
    ConfidenceResolved ResolveConfidence = iota + 1
    // ConfidenceCandidate 表示存在有依据的候选集合。
    ConfidenceCandidate
    // ConfidenceUnresolved 表示无依据，保持未解析。
    ConfidenceUnresolved
)

// CallableNode 表示一个可调用实体。
type CallableNode struct {
    // ID 是该可调用实体的稳定标识。
    ID CallableID
    // Symbol 是该实体对应的符号标识。
    Symbol SymbolID
    // Entry 是该实体函数体 IR 的根节点标识。
    Entry IRID
}

// CallEdge 表示一个调用点到候选目标的关系。
type CallEdge struct {
    // CallSite 是发起调用的 IR 调用节点标识。
    CallSite IRID
    // Caller 是调用方可调用实体标识。
    Caller CallableID
    // Kind 是该调用边的解析方式。
    Kind CallEdgeKind
    // Confidence 是该调用边的置信状态。
    Confidence ResolveConfidence
    // Targets 是候选目标集合，唯一解析时长度为一。
    Targets []CallableID
    // Reason 是候选或未解析的依据说明。
    Reason string
}

// CallGraph 聚合可调用实体与调用边，供下游查询。
type CallGraph interface {
    // Callable 返回指定标识的可调用实体，不存在时返回 false。
    Callable(id CallableID) (CallableNode, bool)
    // EdgesFrom 返回指定可调用实体发出的调用边。
    EdgesFrom(id CallableID) []CallEdge
    // EdgesTo 返回指向指定可调用实体的调用边。
    EdgesTo(id CallableID) []CallEdge
}
```

## 对外接口

```go
// Builder 定义从 IR 与符号表构建调用图的能力。
type Builder interface {
    // Build 消费项目的 IR 模块集合与符号表，构建并返回调用图与诊断。
    Build(ctx context.Context, modules []IRModule, symbols SymbolIndex) (CallGraph, []Diagnostic, error)
}
```

接口约束：

- `Build` 接收项目全部 IR 模块以完成跨文件调用解析。
- 未解析调用通过 `CallEdge.Confidence` 表达，不作为 Go `error` 返回。
- Go `error` 仅用于系统性失败。
- 相同输入产生确定性的标识分配与候选排序。
- 顶层入口 `BuildCallGraph` 转发到 `Builder`。

## 处理流程

1. 遍历 IR 收集可调用实体，分配 `CallableID` 并关联符号与函数入口。
2. 收集全部 IR 调用与构造节点作为调用点。
3. 对直接调用通过符号绑定精确解析目标，生成 `CallDirect` 边。
4. 对虚方法与接口调用按语言规则生成 `CallVirtual` 候选边。
5. 对函数指针与一等函数生成 `CallIndirect` 候选边。
6. 对构造与初始化生成 `CallConstructor` 边。
7. 无依据的调用记录为 `ConfidenceUnresolved` 并说明原因。
8. 输出调用图与诊断。

### 语言规则

- C/C++：直接调用精确解析；函数指针依据可求得的赋值来源生成候选，否则未解析；C++ 虚调用依据符号表可见的类型信息给出候选。
- Java：静态与私有方法精确解析；虚方法依据声明类型与可见继承信息给出候选。
- Go：包级函数与具体类型方法精确解析；接口方法调用依据接口方法集给出候选。
- Python：模块级与明确绑定函数精确解析；一等函数依据可求得的绑定给出候选；完全动态属性调用保持未解析。

## 错误处理

- 无依据调用保持 `ConfidenceUnresolved` 并记录原因，不猜测目标。
- 符号或 IR 缺失导致无法解析时记录诊断，不中止其余解析。
- 候选冲突保留全部候选并标注，不静默丢弃。
- 内部不变量破坏且无法恢复时返回致命错误。

## 性能与资源限制

- 节点与边使用稳定标识关联，避免难以回收的引用环。
- 按需解析调用点，不预先展开无关路径。
- 构建过程接受上下文取消信号。
- 对候选爆炸设置可配置上限，超限时保守截断并记录。
- 内置计时与 profile 埋点并可输出调用图中间摘要，默认关闭。
- 性能基准在获得真实 fixture 后确定。

## 安全考虑

- 所有输入 IR 与符号信息均视为不可信数据。
- 不执行、编译或加载被分析项目代码。
- 对超大调用图与候选爆炸设置资源边界。
- 诊断不包含与当前分析无关的环境敏感信息。

## 测试设计

- 单元测试覆盖五种语言的直接调用、动态调用与未解析场景。
- 正例：直接调用被精确解析，虚调用与接口调用生成正确候选。
- 负例：无依据调用不产生错误目标，保持未解析。
- 边界：递归调用、相互递归、空函数体与深层调用链。
- 变体：等价调用的不同语法形式解析结果一致。
- 回归：每个已修复调用解析缺陷对应独立 fixture。
- 确定性：相同输入产生一致的图结构与候选排序。
- 语言专项：C/C++ 函数指针与虚调用、Java 虚方法、Go 接口调用、Python 一等函数。

Call Graph 是分析流水线的**中间步骤**，不产出漏洞报告，因此不适用端到端误报率阈值（见 AGENTS.md 第 8 条），以**正确性**口径验收：调用边、目标与候选必须与 golden fixture 精确一致，系统失败必须为零，不得产生错误结构。正报为正确产出的调用边结构，负例为不产生无依据的错误目标或错误候选。最低标准为五种语言均有正报，任何结构错误都是必须修复的缺陷，目标是零错误。

## 验收标准

- 五种语言的直接调用均被精确解析。
- 动态调用生成有依据的候选，未解析调用显式记录。
- 构造与初始化调用规则明确且通过测试。
- 五种语言均有有效正报，调用边、目标与候选与 golden fixture 精确一致，系统失败为零，无错误结构（零错误）。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确 Call Graph 职责与不反向依赖类型层次的边界。
- ✅ 完成节点、边、置信状态的数据结构设计。
- ✅ 完成直接与动态调用解析流程及各语言规则设计。
- ✅ 设计单元测试、语言专项测试与误报口径。
- ❌ 创建 `05-call-graph/` 包结构。
- ❌ 创建五种语言测试 fixture 与失败测试。
- ❌ 实现调用点收集与目标解析。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计五种语言的正报、误报与性能基线。
