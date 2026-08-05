# SARIF 报告模块设计

## 结论

SARIF 报告模块收集经路径可达性验证后的污点发现（09、10）及模式匹配（11）、缺失安全检查检测（12）等分析结果与诊断，映射为符合 SARIF 2.1.0 schema 的标准报告。每个发现映射为 `result`，规则信息映射为 `rule`，source→sink 的完整传播路径映射为 `codeFlows`，每一步保留文件、行列范围与消息。相同分析结果必须产生确定性的 SARIF 输出，字段顺序与结果排序稳定。

模块坚持可回溯与确定性：所有 `result`、`rule`、位置与 `codeFlows` 都能回溯到原始源码；按可达性判定标注结果置信度，把不可达路径过滤或降级。报告模块是流水线的出口，不产生新的分析结论。

发现的投影职责明确归属上游：各分析模块（09/11/12）负责把自身异构结果统一投影为 `Finding` 并填好 `Confidence` 与 `Kind`——污点发现的路径填入 `Finding.Flow`、置信度取 09 初值并经 10 可达性判定调整；模式匹配与缺失检查是点类发现，`Flow` 为空、置信度由各自模块按依据填写。13 只消费统一后的 `Finding`，不感知各模块的原始结构（`TaintFlow`/`MatchSite`/`AnomalySite`），从而以单一数据模型汇总异构发现。

```text
13-report/
├── sarif.go        # SARIF 2.1.0 数据模型定义
├── mapping.go      # 分析结果到 SARIF 的映射
├── codeflow.go     # 传播路径到 codeFlows 的映射
├── emit.go         # 确定性序列化输出
├── diagnostic.go   # 报告生成诊断
└── test/
    ├── unit/
    │   ├── fixture/{results,expected}/   # 分析结果输入与期望 SARIF 输出
    │   ├── mapping_test.go
    │   ├── codeflow_test.go
    │   └── determinism_test.go
    └── e2e/
        └── report_e2e_test.go
```

## 目标

- 收集各分析模块的发现与诊断作为报告输入。
- 将发现映射为 SARIF `result`，规则映射为 `rule`。
- 将传播路径映射为 SARIF `codeFlows`，保留每步位置与消息。
- 按可达性判定标注结果置信度。
- 生成符合 SARIF 2.1.0 schema 且可回溯源码的输出。
- 保证相同输入产生确定性 SARIF。

## 非目标

- 不产生新的分析结论或发现。
- 不做去重之外的结果过滤策略判断，过滤规则由输入置信度决定。
- 不自行运行任何分析模块。
- 不负责报告的外部分发与存储。

## 使用场景

- 把污点、模式匹配与缺失检查发现汇总为一份 SARIF 报告。
- 供支持 SARIF 的工具与平台消费分析结果。
- 通过 `codeFlows` 展示 source→sink 的完整数据流。
- 从报告条目回溯到源文件与行列范围。

## 模块边界

### SARIF 报告负责

- 汇总发现与诊断并映射到 SARIF 模型。
- 生成 `rule`、`result`、`codeFlows` 与位置信息。
- 确定性序列化输出。

### SARIF 报告不负责

- 分析求解与发现产生。
- 可达性判定，这来自符号执行。
- 报告分发与外部集成。

## 核心数据结构

```go
// Finding 表示一个待汇入报告的分析发现。各分析模块负责把自身发现投影为 Finding 并填好 Confidence，13 只做汇总与映射，不产生新结论。
type Finding struct {
    // RuleID 是产生该发现的规则标识。
    RuleID string
    // Message 是该发现的说明。
    Message string
    // Primary 是该发现的主位置。
    Primary SourceRange
    // Flow 是该发现的传播路径，仅路径类发现（污点）填充；点类发现（模式匹配、缺失检查）为空。为空时只生成主位置，不生成 codeFlows。
    Flow []PathStep
    // Confidence 是该发现的置信度，由产出模块填写：污点发现取 09 的初值并经 10 可达性判定调整，模式与缺失检查发现由各自模块按自身依据填写。
    Confidence FindingConfidence
    // Kind 是该发现的来源种类，例如污点、模式或缺失检查。
    Kind FindingKind
}

// FindingConfidence 表示发现的置信度级别。
type FindingConfidence uint8

const (
    // ConfidenceHigh 表示经可达性验证确认可达的高置信发现。
    ConfidenceHigh FindingConfidence = iota + 1
    // ConfidenceMedium 表示可达性未知、保守保留的中置信发现。
    ConfidenceMedium
    // ConfidenceLow 表示依据较弱的候选发现。
    ConfidenceLow
)

// SarifReport 表示一份可序列化的 SARIF 2.1.0 报告。
type SarifReport interface {
    // Rules 返回报告中的全部规则条目。
    Rules() []SarifRule
    // Results 返回报告中的全部结果条目。
    Results() []SarifResult
    // Marshal 以确定性字段顺序序列化为 SARIF JSON 字节。
    Marshal() ([]byte, error)
}
```

## 对外接口

```go
// Emitter 定义把分析发现汇总为 SARIF 报告的能力。
type Emitter interface {
    // Emit 消费一组分析发现与诊断，生成 SARIF 报告与生成诊断。
    Emit(ctx context.Context, findings []Finding, diags []Diagnostic) (SarifReport, []Diagnostic, error)
}
```

接口约束：

- 无发现时生成合法的空结果 SARIF 报告，而非错误。
- Go `error` 仅用于系统性失败，例如序列化违反 schema。
- 相同输入产生逐字节一致的 SARIF 输出。
- 结果排序按主位置与规则标识稳定排序，不依赖 map 顺序。
- 顶层入口 `EmitSarifReport` 转发到 `Emitter`。

## 处理流程

1. 收集各模块投影出的 `Finding` 与诊断。
2. 依据 `Finding.Confidence` 过滤或降级：可达性判定已在污点发现的投影阶段（09 初值经 10 调整）并入 Confidence，本模块不直接消费 `PathFeasibility`，只按已固化的置信度剔除或降级低置信发现、保留并标注中置信发现。
3. 汇总规则信息生成 `rule` 条目。
4. 为每个发现生成 `result`，映射主位置与消息，标注置信度。
5. 为带传播路径的发现生成 `codeFlows`，每步映射文件、行列范围与消息。
6. 按稳定顺序排序规则与结果。
7. 校验输出符合 SARIF 2.1.0 schema 并确定性序列化。
8. 输出 `SarifReport` 与生成诊断。

## 错误处理

- 发现缺少必要位置信息时记录诊断并降级处理，不生成非法 `result`。
- 路径步骤缺失位置时跳过该步并标注，不中止整份报告。
- 序列化违反 schema 时返回错误并指出违规条目。
- 内部不变量破坏且无法恢复时返回致命错误。

## 性能与资源限制

- 报告生成为一次性映射，避免对大结果集重复遍历。
- 生成过程接受上下文取消信号。
- 对超大结果集与超长 `codeFlows` 设置可配置上限。
- 内置计时与 profile 埋点并可输出映射中间摘要，默认关闭。
- 性能基准在获得真实 fixture 后确定。

## 安全考虑

- 所有输入发现与诊断均视为不可信数据。
- 不执行、编译或加载被分析项目代码。
- 报告不包含与分析无关的环境敏感信息，如绝对本地路径按配置相对化。
- 对结果集爆炸设置资源边界。

## 测试设计

- 单元测试以 fixture 输入分析结果、比对期望 SARIF 输出。
- 正例：污点、模式与缺失检查发现被正确映射为 `result` 与 `rule`。
- codeFlows 专项：传播路径每步位置与消息正确映射。
- 置信度专项：不可达发现被过滤或降级，未知保留为中置信。
- schema 专项：输出通过 SARIF 2.1.0 schema 校验。
- 确定性专项：相同输入重复生成逐字节一致输出。
- 边界：空发现、单发现、超长路径与多规则。
- 回归：每个已修复映射缺陷对应独立 fixture。

报告模块不做漏洞判定，指标口径为映射正确性：正报为正确映射的发现，误报为映射错误或非法的条目。最低标准为存在正报且映射错误率为零，schema 校验通过。报告模块是整个 SAST 工具的端到端出口，端到端最终漏洞报告的误报率在本模块对外产出的报告上度量，最低标准为误报率 ≤ 5%（以 0 为目标），此为端到端指标而非报告模块单独度量的映射指标。

## 验收标准

- 各模块发现均能映射为符合 SARIF 2.1.0 的 `result` 与 `rule`。
- 传播路径正确映射为 `codeFlows`，每步可回溯源码。
- 置信度按可达性判定正确标注，过滤或降级规则生效。
- 相同输入产生逐字节一致的确定性输出。
- schema 校验通过，映射错误率为零。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确报告模块职责与作为流水线出口的边界。
- ✅ 完成发现投影、SARIF 模型与置信度的数据结构设计。
- ✅ 完成结果映射、codeFlows 映射与确定性序列化流程设计。
- ✅ 设计单元测试、codeFlows 与确定性专项及指标口径。
- ❌ 创建 `13-report/` 包结构与 SARIF 2.1.0 数据模型。
- ❌ 创建分析结果输入与期望 SARIF 输出 fixture。
- ❌ 实现结果映射、codeFlows 映射与确定性序列化。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计映射正确性与性能基线。
