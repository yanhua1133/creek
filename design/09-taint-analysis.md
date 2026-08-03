# 按需污点分析模块设计

## 结论

按需污点分析模块是本项目当前建设的重点。模块接收 source、sanitizer 和 sink 的规约作为查询输入，以函数内 SSA（07）、局部指针分析（08）的别名关系和 Call Graph（05）为基础，按需（on-demand）从 sink 或 source 出发触发求解，不预先展开全程序数据流。分析沿数据流和字段别名传播污点，遇到 sanitizer 按规则清除或降级污点，对到达 sink 且未被拦截的污点输出带完整传播路径的 source→sink 数据流。

需要澄清“按需”的确切含义，避免与上游的全量构建混淆：SSA（07）、指针分析（08）、调用图（05）由前端与分析基础阶段**全量预构建**，作为本模块的事实底座。本模块的“按需”指只沿与查询相关的路径做污点传播、不枚举全部 source-sink 组合、不在传播之外额外展开数据流；跨函数传播时直接**查表消费**已全量构建的上游结果，不重新触发上游构建。因此本模块不承担驱动 07/08/05 部分构建的职责。

模块坚持低误报：传播依据不足或路径不确定时保留候选并显式标注，不凭空补全。字段读写处依赖指针分析提供的别名，保证污点不断链。产出的数据流路径既供符号执行（10）做可达性验证，也供报告（13）生成 SARIF `codeFlows`。

```text
09-taint-analysis/
├── spec.go         # source/sanitizer/sink 规约定义
├── taint.go        # 污点状态、污点流与传播路径定义
├── engine.go       # 按需求解引擎
├── propagate.go    # 沿 def-use 与别名的污点传播
├── interproc.go    # 依据调用图的按需跨函数传播
├── diagnostic.go   # 污点分析诊断
└── test/
    ├── unit/
    │   ├── fixture/{c,cpp,java,go,python}/{positive,negative,boundary,regression}
    │   ├── source_sink_test.go
    │   ├── sanitizer_test.go
    │   ├── field_flow_test.go
    │   └── interproc_test.go
    └── e2e/
        └── taint_e2e_test.go
```

## 目标

- 接收 source、sanitizer、sink 规约作为查询输入。
- 按需从 source 正向或从 sink 反向触发求解，不预展开全程序数据流。
- 沿 def-use 链与字段别名传播污点。
- 在 sanitizer 处按规则清除或降级污点。
- 对到达 sink 的污点输出带完整传播路径的数据流。
- 依据调用图做按需跨函数传播。
- 对不确定传播保留候选并标注，保证低误报。

## 非目标

- 不做全程序穷举数据流。
- 不做路径可达性验证，这是符号执行的职责，本模块只给出数据流路径。
- 不自行构建 SSA、别名或调用图，均消费上游结果。
- 不推断完全动态、无依据的传播，这类保持未确认。

## 使用场景

- 检测污点从不可信输入流向危险操作的数据流。
- 给定 sink 反向回溯可能的污点来源。
- 为符号执行提供待验证的 source→sink 主路径。
- 为报告提供带每步位置的传播路径。

## 模块边界

### 按需污点分析负责

- 解析 source/sanitizer/sink 规约。
- 按需触发并沿数据流与别名传播污点。
- 处理 sanitizer 清除与降级。
- 依据调用图做按需跨函数传播。
- 输出带路径的数据流与不确定标注。

### 按需污点分析不负责

- SSA、别名、调用图与类型层次构建。
- 路径可达性求解与约束判定。
- 报告格式化。

## 核心数据结构

```go
// SpecKind 表示污点规约的种类。
type SpecKind uint8

const (
    // SpecSource 表示污点来源规约。
    SpecSource SpecKind = iota + 1
    // SpecSanitizer 表示污点清除或降级规约。
    SpecSanitizer
    // SpecSink 表示污点危险汇聚点规约。
    SpecSink
)

// TaintSpec 表示一条 source、sanitizer 或 sink 规约。
type TaintSpec struct {
    // Kind 是该规约的种类。
    Kind SpecKind
    // Matcher 是用于匹配目标调用、参数或字段的规约条件。
    Matcher SpecMatcher
    // Label 是该规约携带的污点标签，用于区分不同污点种类。
    Label string
}

// TaintStatus 表示一次求解结果的状态，必须显式区分。
type TaintStatus uint8

const (
    // TaintConfirmed 表示确认的 source 到 sink 数据流。
    TaintConfirmed TaintStatus = iota + 1
    // TaintCandidate 表示依据不足的候选数据流。
    TaintCandidate
    // TaintSanitized 表示被 sanitizer 拦截的数据流。
    TaintSanitized
    // TaintUnreached 表示未到达 sink。
    TaintUnreached
)

// PathStep 表示污点传播路径中的一步。
type PathStep struct {
    // Func 是该步所属的可调用实体标识。由于 SSA 的 ValueID 只在函数内唯一，跨函数路径必须用 Func 与 Value 一起唯一定位一个 SSA 值。
    Func CallableID
    // Value 是该步涉及的 SSA 值标识，仅在 Func 指定的函数内唯一。
    Value ValueID
    // IRNode 是该步对应的 IR 节点标识，全局唯一，可独立回溯源码。
    IRNode IRID
    // Note 是该步的传播说明，例如经字段别名或经调用参数。
    Note string
}

// TaintFlow 表示一条从 source 到 sink 的污点数据流。
type TaintFlow struct {
    // Label 是该数据流的污点标签。
    Label string
    // Status 是该数据流的结果状态。
    Status TaintStatus
    // Source 是起点 source 的 IR 节点标识。
    Source IRID
    // Sink 是终点 sink 的 IR 节点标识。
    Sink IRID
    // Path 是从 source 到 sink 的完整传播路径。
    Path []PathStep
    // Reason 是候选或未确认的依据说明。
    Reason string
}

// TaintResult 聚合污点规约与求解得到的数据流，供下游查询。
type TaintResult interface {
    // Flows 返回全部求解得到的污点数据流。
    Flows() []TaintFlow
    // FlowsToSink 返回到达指定 sink 的污点数据流。
    FlowsToSink(sink IRID) []TaintFlow
}
```

## 对外接口

```go
// Engine 定义按需污点求解能力。
type Engine interface {
    // Analyze 依据给定规约，在 SSA、别名和调用图之上按需求解污点数据流。
    Analyze(ctx context.Context, query TaintQuery) (TaintResult, []Diagnostic, error)
}

// TaintQuery 表示一次按需污点分析的查询输入。
type TaintQuery struct {
    // Specs 是本次查询使用的 source/sanitizer/sink 规约集合。
    Specs []TaintSpec
    // Direction 指定按需求解方向，从 source 正向或从 sink 反向。
    Direction SolveDirection
    // Scope 限定本次求解涉及的函数或文件范围。
    Scope QueryScope
}

// QueryScope 限定一次按需求解的展开范围。
type QueryScope struct {
    // Roots 是求解起始的函数或文件标识集合，为空表示以规约匹配到的位置为起点。
    Roots []CallableID
    // MaxCallDepth 是跨函数展开的最大调用深度，超过则停止展开并标注不完整。
    MaxCallDepth int
    // WholeProject 为真时允许沿调用图展开到全项目可达范围，为假时只在 Roots 直接涉及的函数内求解。
    WholeProject bool
}
```

接口约束：

- `Analyze` 按需触发，只展开与查询相关的路径。
- 不确定数据流通过 `TaintFlow.Status` 表达，不作为 Go `error` 返回。
- Go `error` 仅用于系统性失败。
- 相同查询与相同上游结果产生确定性的数据流与路径排序。
- 顶层入口 `AnalyzeTaint` 转发到 `Engine`。
- 本模块整体可通过配置独立开关，关闭时不影响其余阶段。

## 处理流程

1. 解析 source/sanitizer/sink 规约并匹配 IR 中的对应位置。
2. 按查询方向选择起点：正向从 source，反向从 sink。
3. 沿 def-use 链传播污点，遇字段访问用指针分析别名跨越 store/load。
4. 遇 sanitizer 按规则清除或降级污点并在路径中标注。
5. 需要跨函数时依据调用图按需展开被调用体或调用者，不展开无关函数。
6. 到达 sink 且未被拦截的污点记为确认数据流并记录完整路径。
7. 依据不足的传播记为候选并标注原因。
8. 输出 `TaintResult` 与诊断。

## 错误处理

- 依据不足的传播记为 `TaintCandidate` 并说明原因，不凭空补全。
- 遇到调用图未解析或候选过多的调用点时，默认不自动把污点传播过该调用（避免误报爆炸），而是记为 `TaintCandidate` 断点并标注未解析原因；可按配置切换为保守传播模式。此策略以低误报优先，宁可产生候选断点也不制造无依据的确认数据流。
- 别名或调用候选不唯一时保留候选路径，不强行选择单一目标。
- 上游 SSA、别名或调用图缺失时记录诊断并保守处理，不制造断链或假数据流。
- 求解范围超限时保守终止并标注不完整，不谎报完整。
- 内部不变量破坏且无法恢复时返回致命错误。

## 性能与资源限制

- 按需展开，只处理与查询相关的函数与路径。
- 传播使用工作表并对已访问状态去重，避免重复展开。
- 求解过程接受上下文取消信号。
- 对路径长度、展开函数数与候选分支设置可配置上限，超限保守终止。
- 内置计时与 profile 埋点并可输出污点传播中间摘要，默认关闭。
- 性能基准在获得真实 fixture 后确定。

## 安全考虑

- 所有输入规约、SSA 与别名信息均视为不可信数据。
- 不执行、编译或加载被分析项目代码。
- 对传播爆炸与超深跨函数展开设置资源边界。
- 诊断不包含与当前分析无关的环境敏感信息。

## 测试设计

- 单元测试覆盖五种语言的 source-sink 传播、sanitizer 拦截、字段流与跨函数传播。
- 正例：污点从 source 经数据流到达 sink，输出正确完整路径。
- sanitizer 专项：经 sanitizer 的数据流被正确标记为已清除。
- 字段流专项：污点经对象字段 store 后 load 继续传播，不断链。
- 跨函数专项：污点经调用参数与返回值跨函数传播。
- 负例：无污点来源或被拦截的数据流不误报为 sink 命中。
- 边界：递归传播、循环别名、深层调用链与多标签污点。
- 回归：每个已修复传播缺陷对应独立 fixture。
- 确定性：相同查询产生一致的数据流与路径。

误报率按数据流报告统计：确认数据流中实际不成立的报告数除以确认数据流总数。最低标准为五种语言均有真实正报、总体及分语言误报率低于 30%。

## 验收标准

- 五种语言均能求解 source→sanitizer→sink 数据流并输出完整路径。
- sanitizer 拦截、字段流不断链、跨函数传播均有专项测试通过。
- 不确定传播保留候选并标注，不凭空补全。
- 模块可通过配置独立开关。
- 五种语言均有真实正报，误报率低于 30%。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确按需污点分析职责、当前重点地位与按需求解边界。
- ✅ 完成规约、污点状态、传播路径与数据流的数据结构设计。
- ✅ 完成按需求解、别名跨越与跨函数传播流程设计。
- ✅ 设计单元测试、sanitizer 与字段流专项及误报口径。
- ❌ 创建 `09-taint-analysis/` 包结构。
- ❌ 创建五种语言测试 fixture 与失败测试。
- ❌ 实现按需求解引擎与污点传播。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计五种语言的正报、误报与性能基线。
