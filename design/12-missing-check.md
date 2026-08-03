# 缺失安全检查检测模块设计

## 结论

缺失安全检查检测模块基于代码仓中其他片段找出安全检查异常：当代码库中承担相同职责的多个相似片段大多存在某项安全检查（如鉴权）、个别片段缺失时，报出缺失者。典型例子是六处调用中五处有鉴权、第六处没有，则第六处被判为异常。模块借助语法模式匹配（11）的**结构等价**判定，确保被比较的片段结构等价——只允许括号、等价字面量等语法细微差别。需要诚实界定能力边界：结构等价是语法层判定，不做跨函数数据流推断，因此无法排除“安全检查实际由调用方完成”这类深层语义等价的情形；这类残余误报由样本量阈值与存在比例阈值控制，并对低置信异常标注待人工复核，而非声称能证明完整语义等价。

模块坚持低误报：分组样本过少或等价性无法确认时不报告，避免把正常差异当成异常。检测依据 Call Graph（05）与统一 IR（04）识别片段职责与安全检查调用，依据类型层次（06）判定同类方法。模块整体可通过配置独立开关。

```text
12-missing-check/
├── group.go        # 同类片段分组定义
├── check.go        # 安全检查识别与期望检查集合
├── detect.go       # 一致性异常检测
├── anomaly.go      # 缺失检查异常与置信度
├── diagnostic.go   # 检测诊断
└── test/
    ├── unit/
    │   ├── fixture/{c,cpp,java,go,python}/{positive,negative,boundary,regression}
    │   ├── grouping_test.go
    │   ├── expected_check_test.go
    │   └── anomaly_test.go
    └── e2e/
        └── missingcheck_e2e_test.go
```

## 目标

- 将代码库中承担相同职责的相似片段聚为同类分组。
- 借助模式匹配结构等价判定确认同组片段结构等价（仅允许语法细微差别）。
- 统计同组普遍存在的安全检查，建立期望检查集合。
- 找出缺失期望检查的个别片段并作为异常报告。
- 对样本不足或等价不确定的情形不报告。
- 保证相同输入产生确定性的分组与异常。

## 非目标

- 不做污点数据流或可达性验证。
- 不自行构建 AST、IR、调用图或类型层次。
- 不判定安全检查本身是否正确实现，只判定是否存在。
- 不对孤立、无同类样本的片段下异常结论。

## 使用场景

- 检测某操作在多数调用处有鉴权、个别处缺失的漏洞。
- 检测资源释放、边界校验、加锁等应一致存在的检查缺失。
- 为报告提供带同类样本对照的异常发现。
- 从异常回溯到缺失位置与同类样本位置。

## 模块边界

### 缺失安全检查检测负责

- 同类片段分组与结构等价确认。
- 期望检查集合统计与缺失识别。
- 异常报告与置信度、样本对照记录。

### 缺失安全检查检测不负责

- 数据流、污点与可达性分析。
- 模式匹配与等价判定实现，这来自 11。
- AST、IR、调用图、类型层次构建。

## 核心数据结构

```go
// GroupID 是同类片段分组的稳定标识。
type GroupID uint32

// CodeFragment 表示一个参与比较的代码片段。
type CodeFragment struct {
    // Root 是该片段根节点标识。
    Root NodeID
    // Callable 是该片段所属可调用实体标识。
    Callable CallableID
    // Range 是该片段的源码范围。
    Range SourceRange
}

// GroupBasis 表示片段被判为同类的分组依据，按优先级从高到低取用。
type GroupBasis uint8

const (
    // BasisSignature 表示依据相同的函数或方法签名分组，优先级最高。
    BasisSignature GroupBasis = iota + 1
    // BasisCallee 表示依据调用同一目标函数分组。
    BasisCallee
    // BasisType 表示依据同一所属类型或同类方法分组，优先级最低。
    BasisType
)

// PeerGroup 表示一组结构等价、承担相同职责的片段。
type PeerGroup struct {
    // ID 是该分组的稳定标识。
    ID GroupID
    // Fragments 是该分组内的结构等价片段集合。
    Fragments []CodeFragment
    // Basis 是判定这些片段同组的分组依据种类。
    Basis GroupBasis
    // BasisReason 是分组依据的补充说明。
    BasisReason string
}

// ExpectedCheck 表示一个分组内期望存在的安全检查。
type ExpectedCheck struct {
    // Group 是该期望检查所属分组标识。
    Group GroupID
    // CheckRuleID 是识别该安全检查所用的模式规则标识。
    CheckRuleID string
    // PresentCount 是分组内存在该检查的片段数量。
    PresentCount int
    // TotalCount 是分组内片段总数。
    TotalCount int
}

// AnomalySite 表示一处缺失期望检查的异常。
type AnomalySite struct {
    // Group 是所属分组标识。
    Group GroupID
    // MissingIn 是缺失检查的片段。
    MissingIn CodeFragment
    // Expected 是所缺失的期望检查。
    Expected ExpectedCheck
    // Confidence 是该异常的置信度，依据存在比例与样本量。
    Confidence float64
    // Peers 是提供对照的同组存在检查的片段。
    Peers []CodeFragment
}

// MissingCheckFindings 聚合分组、期望检查与异常，供下游查询。
type MissingCheckFindings interface {
    // Group 返回指定标识的分组，不存在时返回 false。
    Group(id GroupID) (PeerGroup, bool)
    // Anomalies 返回全部缺失检查异常。
    Anomalies() []AnomalySite
}
```

## 对外接口

```go
// Detector 定义基于片段一致性的缺失安全检查检测能力。
type Detector interface {
    // Detect 依据调用图、类型层次与模式匹配结果，检测缺失安全检查异常并返回诊断。
    Detect(ctx context.Context, in DetectInput) (MissingCheckFindings, []Diagnostic, error)
}

// DetectInput 表示一次检测所需的上游输入。
type DetectInput struct {
    // Modules 是项目的 IR 模块集合。
    Modules []IRModule
    // CallGraph 是项目调用图，用于识别片段职责与安全检查调用。
    CallGraph CallGraph
    // Hierarchy 是类型层次，用于判定同类方法。
    Hierarchy TypeHierarchy
    // Patterns 是模式匹配结果，用于等价判定与安全检查识别。
    Patterns PatternMatches
    // MinGroupSize 是形成有效分组的最小样本量，低于此不报异常。
    MinGroupSize int
    // MinPresentRatio 是判定为期望检查所需的最小存在比例。
    MinPresentRatio float64
}
```

接口约束：

- 无异常不是错误，结果为空集合。
- Go `error` 仅用于系统性失败。
- 相同输入与相同阈值产生确定性的分组与异常。
- 分组与异常排序稳定，不依赖 map 顺序。
- 顶层入口 `DetectMissingChecks` 转发到 `Detector`。
- 本模块整体可通过配置独立开关。

## 处理流程

1. 依据调用图与类型层次，把承担相同职责的相似片段聚为候选分组，分组依据按优先级取用：相同签名 > 调用同一目标 > 同类方法（记为 `GroupBasis`）。
2. 用模式匹配的 `Equivalent` 结构等价判定确认同组片段结构等价，剔除仅表面相似但结构不同者，只允许语法细微差别。
3. 分组样本量低于 `MinGroupSize` 时放弃该分组，不报异常。
4. 统计同组内各安全检查的存在片段数，存在比例达到 `MinPresentRatio` 的检查记为期望检查。
5. 找出缺失期望检查的片段，依据存在比例与样本量计算置信度。
6. 输出异常及其同组对照片段与诊断。

## 错误处理

- 等价性无法确认的片段不纳入分组，不制造异常。
- 样本量不足的分组不报异常，标注为样本不足。
- 上游调用图、类型层次或模式结果缺失时记录诊断并跳过相关分组，不中止其余检测。
- 内部不变量破坏且无法恢复时返回致命错误。

## 性能与资源限制

- 分组按职责键索引，避免片段两两全比较。
- 等价判定复用模式匹配结果，不重复遍历子树。
- 检测过程接受上下文取消信号。
- 对分组规模与片段数设置可配置上限。
- 内置计时与 profile 埋点并可输出分组与期望检查中间摘要，默认关闭。
- 性能基准在获得真实 fixture 后确定。

## 安全考虑

- 所有输入上游结果均视为不可信数据。
- 不执行、编译或加载被分析项目代码。
- 对分组与片段爆炸设置资源边界。
- 诊断不包含与当前分析无关的环境敏感信息。

## 测试设计

- 单元测试覆盖五种语言的分组、期望检查统计与异常识别。
- 正例：构造多处等价片段，多数含鉴权、个别缺失，断言缺失者被报出。
- 等价专项：语义不同但语法相似的片段不被误分入同组。
- 样本专项：样本量低于阈值时不报异常。
- 负例：所有片段一致存在检查或一致缺失时不报异常。
- 边界：单一样本、存在比例临界值与多种检查并存。
- 回归：每个已修复误报或漏报对应独立 fixture。
- 确定性：相同输入产生一致分组与异常。

误报率按异常统计：实际不构成缺失检查漏洞的异常数除以异常总数。最低标准为五种语言均有真实正报、总体及分语言误报率低于 30%。

## 验收标准

- 五种语言均能对等价片段分组并报出缺失检查异常。
- 等价确认、样本阈值与存在比例阈值均有专项测试通过。
- 样本不足或等价不确定时不报异常。
- 模块可通过配置独立开关。
- 五种语言均有真实正报，误报率低于 30%。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确缺失检查检测职责与依赖模式匹配等价判定的边界。
- ✅ 完成分组、期望检查与异常的数据结构设计。
- ✅ 完成分组、等价确认、期望统计与异常识别流程设计。
- ✅ 设计单元测试、等价与样本专项及误报口径。
- ❌ 创建 `12-missing-check/` 包结构。
- ❌ 创建五种语言测试 fixture 与失败测试。
- ❌ 实现分组、期望检查统计与异常检测。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计五种语言的正报、误报与性能基线。
