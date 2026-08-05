# 语法模式匹配模块设计

## 结论

语法模式匹配模块接收带元变量的语法模式规则，以 Common AST（02）为**主要匹配对象**在语法树上匹配结构，识别硬编码口令等简单漏洞模式。命中位置与元变量绑定统一以 AST 节点（`NodeID`）表达，不在统一 IR 上另建独立匹配结果，避免 AST/IR 双重语义；元变量约束可选引用统一 IR（04）与符号表（03）提供的类型信息来收紧匹配。元变量绑定具体子节点并在同一模式内保持一致性约束，且可施加名称、类型、常量或字面量等约束以缩小匹配范围、提高精确度。模块输出带源码范围和元变量绑定的匹配结果，既直接产生漏洞发现，也为缺失安全检查检测（12）提供片段语义等价判定能力。

模块坚持低误报：匹配只基于语法结构与可用符号信息，不做跨函数数据流推断，不臆造无依据结果。模块整体可通过配置独立开关。

```text
11-pattern-match/
├── pattern.go      # 模式规则、元变量与约束定义
├── matcher.go      # 结构匹配引擎
├── binding.go      # 元变量绑定与一致性约束
├── equiv.go        # 供 12 使用的片段语义等价判定
├── diagnostic.go   # 匹配诊断
└── test/
    ├── unit/
    │   ├── fixture/{c,cpp,java,go,python}/{positive,negative,boundary,regression}
    │   ├── match_test.go
    │   ├── metavar_test.go
    │   ├── constraint_test.go
    │   └── equivalence_test.go
    └── e2e/
        └── pattern_e2e_test.go
```

## 目标

- 接收带元变量的语法模式规则并在 AST/IR 上匹配。
- 元变量绑定子节点并保持同一模式内一致性。
- 支持名称、类型、常量、字面量等元变量约束。
- 输出带源码范围与元变量绑定的匹配结果。
- 提供片段语义等价判定能力供缺失检查检测复用。
- 保证相同规则与输入产生确定性匹配结果。

## 非目标

- 不做跨函数数据流或污点推断。
- 不做路径可达性验证。
- 不自行构建 AST、IR 或符号表。
- 不匹配无依据的动态构造，这类不产生结果。

## 使用场景

- 匹配硬编码口令如 `password = "..."`。
- 匹配危险 API 的固定误用模式。
- 为缺失检查检测判定两段代码是否语义等价。
- 从匹配结果回溯到源码范围与元变量绑定。

## 模块边界

### 语法模式匹配负责

- 解析模式规则与元变量约束。
- 在 AST/IR 上做结构匹配与元变量绑定。
- 输出匹配结果与绑定。
- 提供片段语义等价判定。

### 语法模式匹配不负责

- 数据流、污点与可达性分析。
- AST、IR、符号表构建。
- 报告格式化。

## 核心数据结构

```go
// MetaVar 表示模式中的一个元变量，可绑定具体子节点。
type MetaVar struct {
    // Name 是元变量名称，同名元变量在一个模式内必须绑定一致。
    Name string
    // Constraints 是施加在该元变量上的约束集合。
    Constraints []MetaConstraint
}

// MetaConstraintKind 表示元变量约束的种类。
type MetaConstraintKind uint8

const (
    // ConstraintName 表示约束绑定节点的名称。
    ConstraintName MetaConstraintKind = iota + 1
    // ConstraintType 表示约束绑定节点的类型。
    ConstraintType
    // ConstraintConstant 表示约束绑定节点为常量。
    ConstraintConstant
    // ConstraintLiteral 表示约束绑定节点为特定字面量类别。
    ConstraintLiteral
)

// PatternRule 表示一条语法模式规则。
type PatternRule struct {
    // ID 是该规则的稳定标识。
    ID string
    // Language 是该规则适用的语言，跨语言规则时为空表示通用。
    Language Language
    // Template 是模式模板的 AST/IR 结构表示。
    Template PatternNode
    // MetaVars 是该模式使用的元变量集合。
    MetaVars []MetaVar
    // Message 是命中该规则时的说明。
    Message string
}

// MetaBinding 表示一次匹配中元变量到具体节点的绑定。
type MetaBinding struct {
    // Var 是被绑定的元变量名称。
    Var string
    // Node 是绑定到的 AST 或 IR 节点标识。
    Node NodeID
}

// MatchSite 表示一次模式命中。
type MatchSite struct {
    // RuleID 是命中的规则标识。
    RuleID string
    // Root 是命中位置的根节点标识。
    Root NodeID
    // Bindings 是本次命中的元变量绑定集合。
    Bindings []MetaBinding
    // Range 是命中位置的源码范围。
    Range SourceRange
}

// PatternMatches 聚合模式命中结果，供下游查询。
type PatternMatches interface {
    // Sites 返回全部模式命中。
    Sites() []MatchSite
    // SitesByRule 返回指定规则的全部命中。
    SitesByRule(ruleID string) []MatchSite
    // Equivalent 判定两个 AST 子树是否结构等价：在忽略规则允许的语法细节（如括号、等价字面量写法）后结构一致。这是语法层的结构等价，不含数据流或跨函数语义推断，不等于完整语义等价；12 据此做基于结构相似性的启发式检测，其残余误报由样本阈值控制。
    Equivalent(a, b NodeID) bool
}
```

## 对外接口

```go
// Matcher 定义在 AST/IR 上做语法模式匹配的能力。
type Matcher interface {
    // Match 依据给定规则集合，在 AST 与 IR 上匹配并返回命中结果与诊断。
    Match(ctx context.Context, rules []PatternRule, units []ast.CommonAST, modules []IRModule) (PatternMatches, []Diagnostic, error)
}
```

接口约束：

- 单条规则无命中不是错误，匹配结果为空集合。
- Go `error` 仅用于系统性失败，例如规则无法解析。
- 相同规则与输入产生确定性的命中与绑定排序。
- `Equivalent` 判定必须确定、对称，且只容忍规则声明允许的语法细节差异。
- 元变量作用域限于单条 `PatternRule`，不跨规则共享；不同规则中的同名元变量互不影响。
- 顶层入口 `MatchPatterns` 转发到 `Matcher`。
- 本模块整体可通过配置独立开关。

## 处理流程

1. 解析模式规则，校验元变量与约束合法性。
2. 遍历 AST/IR，在每个候选位置尝试结构匹配。
3. 匹配时绑定元变量，校验同名元变量绑定一致。
4. 校验元变量约束（名称、类型、常量、字面量），不满足则匹配失败。
5. 对命中位置输出根节点、绑定与源码范围。
6. 为 12 提供 `Equivalent`：对两子树做结构比较，忽略规则允许的语法细节（如括号、等价字面量写法），但语义不同即判不等价。
7. 输出 `PatternMatches` 与诊断。

## 错误处理

- 规则语法非法返回 `ErrInvalidPattern`，标注规则位置。
- 元变量约束引用不可用信息（如类型缺失）时该约束保守判为不满足，不误命中。
- 上游 AST/IR 缺失时记录诊断并跳过该位置，不中止其余匹配。
- 内部不变量破坏且无法恢复时返回致命错误。

## 性能与资源限制

- 按节点种类索引候选位置，避免对每条规则全树扫描。
- 匹配过程接受上下文取消信号。
- 对模式深度与回溯次数设置可配置上限。
- 内置计时与 profile 埋点并可输出命中中间摘要，默认关闭。
- 性能基准在获得真实 fixture 后确定。

## 安全考虑

- 所有输入规则、AST 与 IR 均视为不可信数据。
- 不执行、编译或加载被分析项目代码。
- 对模式回溯爆炸设置资源边界。
- 诊断不包含与当前分析无关的环境敏感信息。

## 测试设计

- 单元测试覆盖五种语言的结构匹配、元变量绑定、约束与等价判定。
- 正例：硬编码口令等模式被正确命中，元变量绑定正确。
- 元变量专项：同名元变量一致性约束生效，不一致时不命中。
- 约束专项：名称、类型、常量、字面量约束正确缩小匹配。
- 等价专项：仅语法细节不同的两段代码判为等价，语义不同判为不等价。
- 负例：不满足模式或约束的代码不误命中。
- 边界：空模式、深层嵌套模式与重叠命中。
- 回归：每个已修复匹配缺陷对应独立 fixture。
- 确定性：相同规则与输入产生一致命中。

误报率按命中统计：实际不构成目标模式的命中数除以命中总数。此处误报率指整个 SAST 工具端到端跑完、对外产出的最终漏洞报告的误报率，不是本模块单独度量的中间指标。最低标准为五种语言均有真实正报、总体及分语言端到端最终漏洞报告误报率 ≤ 5%（以 0 为目标）。

## 验收标准

- 五种语言均能用带元变量约束的模式命中目标漏洞模式。
- 元变量一致性、约束与语义等价判定均有专项测试通过。
- `Equivalent` 判定确定、对称，只容忍允许的语法差异。
- 模块可通过配置独立开关。
- 五种语言均有真实正报，端到端最终漏洞报告误报率 ≤ 5%（以 0 为目标）。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确语法模式匹配职责与为 12 提供等价判定的定位。
- ✅ 完成模式规则、元变量、约束与命中的数据结构设计。
- ✅ 完成结构匹配、元变量绑定与等价判定流程设计。
- ✅ 设计单元测试、元变量与等价专项及误报口径。
- ❌ 创建 `11-pattern-match/` 包结构。
- ❌ 创建五种语言测试 fixture 与失败测试。
- ❌ 实现结构匹配引擎与等价判定。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计五种语言的正报、误报与性能基线。
