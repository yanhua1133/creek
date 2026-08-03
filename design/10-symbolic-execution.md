# 局部符号执行与主路径可达性验证模块设计

## 结论

本模块对污点分析（09）给出的 source→sink 主路径做局部符号执行，验证该路径是否真实可达，剪除可证明不可达的误报。执行方式是自建一个轻量 IR 虚拟机，解释执行统一 IR（04），全程不运行、不编译、不加载被分析项目的原生代码——因此本模块是纯静态分析，不违反项目安全边界。约束求解只针对主路径，轻量判定其可满足性，不无止境或与主路径无关地枚举求解。

模块坚持不制造漏报：只有在可证明路径约束 UNSAT 时才剪除路径；求解超时或存在未建模操作时保守保留为可能可达。剪枝方向以从 sink 反向回溯为主，能更早用 sink 侧约束缩小搜索。

```text
10-symbolic-execution/
├── vm.go           # 轻量 IR 虚拟机与符号状态定义
├── path.go         # 主路径表示与路径约束收集
├── constraint.go   # 约束表达与轻量求解接口
├── solver.go       # 约束求解后端封装
├── verify.go       # 可达性判定与剪枝
├── diagnostic.go   # 符号执行诊断
└── test/
    ├── unit/
    │   ├── fixture/{c,cpp,java,go,python}/{positive,negative,boundary,regression}
    │   ├── vm_test.go
    │   ├── constraint_test.go
    │   └── feasibility_test.go
    └── e2e/
        └── symexec_e2e_test.go
```

## 目标

- 以污点主路径为唯一验证目标，只针对该路径工作。
- 在自建轻量 IR 虚拟机中解释执行统一 IR。
- 沿主路径收集分支条件、SSA 值约束与指针别名约束。
- 轻量求解主路径约束的可满足性。
- 可满足则确认可达，可证明 UNSAT 才剪除。
- 求解超时或未建模操作时保守保留为可能可达。

## 非目标

- 不做全程序或全路径符号执行。
- 不运行、编译或加载被分析项目原生代码。
- 不做污点分析或数据流求解，只验证给定路径。
- 不追求完备求解，未建模操作走保守分支。

## 使用场景

- 验证一条 source→sink 污点路径是否真实可达。
- 为报告提供路径可达性判定与置信度。
- 剪除约束矛盾的误报数据流。
- 从判定结果回溯到路径步骤与源码位置。

## 模块边界

### 本模块负责

- 轻量 IR 虚拟机解释执行与符号状态维护。
- 主路径约束收集与轻量可满足性求解。
- 可达性判定与保守剪枝。

### 本模块不负责

- 污点传播与数据流求解。
- 全程序路径枚举。
- 报告格式化。

## 核心数据结构

```go
// Feasibility 表示一条主路径的可达性判定结果，必须显式区分。
type Feasibility uint8

const (
    // FeasibleReachable 表示路径约束可满足，路径可达。
    FeasibleReachable Feasibility = iota + 1
    // FeasibleInfeasible 表示路径约束可证明 UNSAT，路径不可达。
    FeasibleInfeasible
    // FeasibleUnknown 表示求解超时或存在未建模操作，保守视为可能可达。
    FeasibleUnknown
)

// UnknownReason 细分 FeasibleUnknown 的原因，便于诊断与度量，避免用同一状态掩盖不同成因。
type UnknownReason uint8

const (
    // UnknownUnmodeledOp 表示路径含虚拟机未建模的 IR 操作。
    UnknownUnmodeledOp UnknownReason = iota + 1
    // UnknownUnsupportedConstraint 表示约束超出支持的理论（如非线性算术）。
    UnknownUnsupportedConstraint
    // UnknownSolverTimeout 表示约束求解超时。
    UnknownSolverTimeout
)

// SymbolicValue 表示虚拟机中的一个符号值或具体值。
type SymbolicValue struct {
    // ValueRef 是该符号值对应的 SSA 值标识。
    ValueRef ValueID
    // Concrete 标记该值是否已具体化。
    Concrete bool
    // Expr 是该值的符号表达式，具体化时为常量表达式。
    Expr ConstraintExpr
}

// PathConstraint 表示主路径上收集到的一条约束。
type PathConstraint struct {
    // Step 是产生该约束的路径步索引。
    Step int
    // Expr 是该约束的布尔表达式。
    Expr ConstraintExpr
    // Origin 是该约束来源，例如分支条件、值约束或别名约束。
    Origin ConstraintOrigin
}

// TargetPath 表示一条待验证的污点主路径。
type TargetPath struct {
    // FlowRef 是对应污点数据流的标识，回指 09 的 TaintFlow。
    FlowRef TaintFlowRef
    // Steps 是主路径的有序步骤，直接来自 09 的 TaintFlow.Path。每步的 PathStep 含 Func 与 Value，虚拟机据此在对应函数的 SSA 中定位值，从而跨函数复用不同函数的局部 ValueID 空间而不冲突。
    Steps []PathStep
    // Status 是该路径来自 09 的污点状态，默认只验证 TaintConfirmed 的路径，是否纳入 TaintCandidate 由配置决定。
    Status TaintStatus
}

// FeasibilityVerdict 表示对一条主路径的最终判定。
type FeasibilityVerdict struct {
    // Path 是被验证的主路径。
    Path TargetPath
    // Result 是可达性判定结果。
    Result Feasibility
    // Constraints 是判定所依据的路径约束集合。
    Constraints []PathConstraint
    // Unknown 是 Result 为 FeasibleUnknown 时的细分原因，其他结果时为零值。
    Unknown UnknownReason
    // Reason 是判定为不可达或未知的补充说明。
    Reason string
}

// PathFeasibility 聚合主路径与其可达性判定，供下游查询。
type PathFeasibility interface {
    // Verdict 返回指定污点数据流的可达性判定，不存在时返回 false。
    Verdict(flow TaintFlowRef) (FeasibilityVerdict, bool)
    // Verdicts 返回全部主路径的可达性判定。
    Verdicts() []FeasibilityVerdict
}
```

## 对外接口

```go
// Verifier 定义对污点主路径做局部符号执行与可达性验证的能力。
type Verifier interface {
    // Verify 对给定污点主路径做符号执行，返回可达性判定与诊断。
    Verify(ctx context.Context, paths []TargetPath, modules []IRModule) (PathFeasibility, []Diagnostic, error)
}
```

接口约束：

- `Verify` 只处理传入的主路径，不自行发现路径。
- 不可达与未知判定通过 `Feasibility` 表达，不作为 Go `error` 返回。
- Go `error` 仅用于系统性失败。
- 相同路径与相同 IR 产生确定性的判定结果。
- 顶层入口 `VerifyPathFeasibility` 转发到 `Verifier`。
- 本模块整体可通过配置独立开关，关闭时污点数据流不做可达性过滤直接下传报告。

## 处理流程

0. 依据设计维护一份**明确的支持清单**：可解释的 `IRKind` 集合与可求解的约束理论（至少覆盖线性整数算术与布尔逻辑）。清单外的操作或约束不得默认判定，一律记为 `FeasibleUnknown` 并标注对应 `UnknownReason`。核心污点路径高频涉及的 IRKind 必须在支持清单内，否则本模块无法产生有效剪枝，需回到 04 协调。
1. 接收污点主路径，选择从 sink 反向或从 source 正向的验证方向；默认只处理 `Status` 为 `TaintConfirmed` 的路径。
2. 在轻量 IR 虚拟机中沿主路径解释执行统一 IR，维护符号状态；每步用 `PathStep.Func` 与 `Value` 在对应函数 SSA 中定位值，跨函数切换时切换到被调用/调用函数的 SSA 值空间。
3. 收集分支条件、SSA 值约束与指针别名约束为路径约束。
4. 调用轻量求解判定路径约束可满足性。
5. 可满足记为可达；可证明 UNSAT 记为不可达并剪除对应数据流。
6. 求解超时或遇未建模操作记为未知，保守保留为可能可达。
7. 输出 `PathFeasibility` 与诊断。

## 错误处理

- 未建模的 IR 操作记为未知并保守保留，不判为不可达以免漏报。
- 求解器超时记为未知并标注，不谎报可达或不可达。
- 主路径引用的 IR 缺失时记录诊断并跳过该路径，不中止其余验证。
- 内部不变量破坏且无法恢复时返回致命错误。

## 性能与资源限制

- 只沿主路径执行，不展开无关分支。
- 对路径长度、分支数量、虚拟机步数和约束求解时间设置可配置上限，超限保守终止为未知。
- 求解过程接受上下文取消信号。
- 符号状态按路径步演进并复用，避免重复构造。
- 内置计时与 profile 埋点并可输出符号状态与约束中间摘要，默认关闭。
- 性能基准在获得真实 fixture 后确定。

## 安全考虑

- 所有输入路径与 IR 均视为不可信数据。
- 符号执行只在自建轻量虚拟机中解释统一 IR，不运行被分析项目原生代码。
- 对路径长度、虚拟机步数与求解时间设置资源上限，防止无界求解。
- 诊断不包含与当前分析无关的环境敏感信息。

## 测试设计

- 单元测试覆盖五种语言主路径的虚拟机执行、约束收集与可达性判定。
- 正例：约束可满足的真实路径被判为可达。
- 剪枝专项：构造约束矛盾的路径（如互斥分支条件），断言被判为不可达并剪除。
- 保守专项：含未建模操作或超时的路径被判为未知并保留，不漏报。
- 负例：不把可达路径误判为不可达。
- 边界：空约束路径、深层分支、循环内路径与多约束合取。
- 回归：每个已修复判定缺陷对应独立 fixture。
- 确定性：相同路径产生一致判定。

由于本模块是误报过滤器，指标口径为：正确剪除的不可达路径计为有效正报；把真实可达路径误剪为不可达计为误报（等价于引入漏报，必须为零容忍方向），未知保留不计误报。最低标准为存在有效剪除正报、且不存在把可达误判为不可达的情形。

## 验收标准

- 能在自建轻量 IR 虚拟机上对五种语言的污点主路径做符号执行。
- 约束矛盾路径被正确判为不可达并剪除。
- 未建模与超时路径保守保留为未知，不产生漏报。
- 全程不运行被分析项目原生代码。
- 模块可通过配置独立开关。
- 存在有效剪除正报，无可达误判为不可达的情形。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确符号执行职责、纯静态边界与只验主路径的定位。
- ✅ 完成轻量虚拟机、路径约束与可达性判定的数据结构设计。
- ✅ 完成主路径执行、约束收集与保守剪枝流程设计。
- ✅ 设计单元测试、剪枝与保守专项及指标口径。
- ❌ 创建 `10-symbolic-execution/` 包结构。
- ❌ 选定并封装约束求解后端。
- ❌ 创建五种语言测试 fixture 与失败测试。
- ❌ 实现轻量 IR 虚拟机、约束收集与可达性判定。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计有效剪除、误判与性能基线。
