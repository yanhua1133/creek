# 函数内 SSA 模块设计

## 结论

函数内 SSA 模块以统一 IR（04）中每个函数或方法体为单位构建控制流图，为局部变量插入 φ 函数并重命名，生成静态单赋值形式。每个 SSA 值保留到 IR 节点与源码位置的映射，并明确区分局部变量、参数、字段访问和调用返回值等值来源。SSA 是指针分析（08）、污点分析（09）和符号执行（10）的直接输入。

模块只做函数内分析，不做跨函数分析；跨函数信息由 Call Graph 与后续过程间分析承接。SSA 建立在 IR 已归一的控制流之上，因此循环、分支等已是规范形式，无需在本模块处理语法差异。

```text
07-ssa/
├── cfg.go          # 控制流图、基本块与稳定标识定义
├── ssa.go          # SSA 值、φ 函数与值来源定义
├── build.go        # 从 IR 函数体构建 CFG 与 SSA
├── dominance.go    # 支配树与支配边界计算
├── diagnostic.go   # SSA 构建诊断
└── test/
    ├── unit/
    │   ├── fixture/{c,cpp,java,go,python}/{positive,negative,boundary,regression}
    │   ├── cfg_test.go
    │   ├── phi_test.go
    │   └── value_source_test.go
    └── e2e/
        └── ssa_e2e_test.go
```

## 目标

- 为每个函数体构建控制流图与基本块。
- 计算支配关系并在支配边界插入 φ 函数。
- 对局部变量重命名生成静态单赋值形式。
- 保留每个 SSA 值到 IR 节点与源码位置的映射。
- 明确区分局部变量、参数、字段访问与调用返回值等值来源。
- 保证相同 IR 产生确定性的 CFG 与 SSA。

## 非目标

- 不做跨函数分析与过程间数据流。
- 不做别名分析与指向关系，这是指针分析的职责。
- 不做常量传播、死代码消除等优化变换。
- 不处理语法差异，控制流已由 IR 归一。

## 使用场景

- 为指针分析提供每个变量的定义与使用位置。
- 为污点分析提供 def-use 链与控制流。
- 为符号执行提供基本块级别的执行单元。
- 从 SSA 值回溯到 IR 节点与源码位置。

## 模块边界

### 函数内 SSA 负责

- 构建 CFG、计算支配树与支配边界。
- 插入 φ 函数并重命名局部变量。
- 标注每个 SSA 值的来源类别。
- 维护 SSA 值到 IR 与源码的映射。

### 函数内 SSA 不负责

- 跨函数与过程间分析。
- 别名与指向关系计算。
- 类型推导与调用解析。

## 核心数据结构

```go
// BlockID 是基本块在控制流图中的稳定标识。
type BlockID uint32

// ValueID 是 SSA 值在单个函数内的稳定标识，仅在所属函数内唯一。跨函数引用一个 SSA 值时，必须与其所属函数的标识配对才能唯一定位；下游污点路径与符号执行据此处理跨函数值标识空间。
type ValueID uint32

// ValueSource 表示一个 SSA 值的来源类别，必须显式区分而非混淆。
type ValueSource uint8

const (
    // SourceLocal 表示来自局部变量赋值。
    SourceLocal ValueSource = iota + 1
    // SourceParam 表示来自函数参数。
    SourceParam
    // SourceField 表示来自字段或成员访问。
    SourceField
    // SourceCallReturn 表示来自调用返回值。
    SourceCallReturn
    // SourcePhi 表示来自 φ 函数合并。
    SourcePhi
)
```

字段与堆内存的表示遵循标准 SSA 约定：本模块只对局部标量变量做 SSA 重命名与 φ 插入，不对对象字段或堆内存建立 SSA 值或 φ。字段读写以统一 IR 的字段加载/存储节点（`IRLoadField`/`IRStoreField`，见 04）为锚点——字段加载产生一个来源为 `SourceField` 的 SSA 值，字段存储不产生新的标量 SSA 值但保留其 IR 节点。局部指针分析（08）在这些字段节点及其基指针对应的 SSA 值之上解析别名，本模块不承担字段别名，从而与 08 形成清晰的锚点契约。

```go
// BasicBlock 表示控制流图中的一个基本块。
type BasicBlock struct {
    // ID 是该基本块的稳定标识。
    ID BlockID
    // Instrs 是该块内按顺序排列的 SSA 值标识。
    Instrs []ValueID
    // Preds 是前驱基本块标识集合。
    Preds []BlockID
    // Succs 是后继基本块标识集合。
    Succs []BlockID
}

// SSAValue 表示一个静态单赋值值。
type SSAValue struct {
    // ID 是该值的稳定标识。
    ID ValueID
    // Source 是该值的来源类别。
    Source ValueSource
    // IRNode 是产生该值的 IR 节点标识。
    IRNode IRID
    // Operands 是该值依赖的操作数值标识，顺序稳定。
    Operands []ValueID
    // Block 是该值所在的基本块标识。
    Block BlockID
}

// PhiNode 表示一个 φ 函数，合并来自不同前驱的值。
type PhiNode struct {
    // Result 是 φ 函数产生的结果值标识。
    Result ValueID
    // Incomings 是每个前驱块及其传入值的对应关系。
    Incomings []PhiIncoming
}

// FunctionSSA 表示单个函数体的控制流图与 SSA。
type FunctionSSA interface {
    // Entry 返回入口基本块标识。
    Entry() BlockID
    // Block 返回指定标识的基本块，不存在时返回 false。
    Block(id BlockID) (BasicBlock, bool)
    // Value 返回指定标识的 SSA 值，不存在时返回 false。
    Value(id ValueID) (SSAValue, bool)
    // DefOf 返回定义指定值的位置，UseOf 返回其全部使用位置。
    DefOf(id ValueID) (ValueID, bool)
}
```

## 对外接口

```go
// Builder 定义从 IR 函数体构建函数内 SSA 的能力。
type Builder interface {
    // Build 消费一个函数体的 IR 根节点，构建并返回其控制流图与 SSA。
    Build(ctx context.Context, function IRNode) (FunctionSSA, []Diagnostic, error)
}
```

接口约束：

- `Build` 以单个函数体为单位，不接收跨函数信息。
- 无法构建 SSA 的局部结构通过 `Diagnostic` 表达，不作为 Go `error` 返回。
- Go `error` 仅用于系统性失败。
- 相同 IR 产生确定性的 `BlockID`、`ValueID` 分配与 φ 插入。
- 顶层入口 `BuildFunctionSSA` 对项目内全部函数体逐一构建。

## 处理流程

1. 遍历函数体 IR，按归一后的控制流切分基本块并建立 CFG 边。
2. 计算支配树与支配边界。
3. 在支配边界为被多处赋值的局部变量插入 φ 函数。
4. 沿支配树重命名局部变量，生成 SSA 值。
5. 为每个 SSA 值标注来源类别并记录到 IR 与源码的映射。
6. 输出 `FunctionSSA` 与诊断。

## 错误处理

- 不可达代码块必须显式标记，不静默丢弃。
- 无法确定来源的值标注为对应上游状态，不臆造来源。
- IR 缺失或非法控制流记录诊断，不中止其余函数处理。
- 内部不变量破坏且无法恢复时返回致命错误。

## 性能与资源限制

- 基本块与值使用稳定标识关联，避免难以回收的引用环。
- 支配计算使用与函数规模成比例的算法，避免超线性退化。
- 构建过程接受上下文取消信号。
- 对超大函数体与超多基本块设置可配置上限。
- 内置计时与 profile 埋点并可输出 CFG 与 SSA 中间摘要，默认关闭。
- 性能基准在获得真实 fixture 后确定。

## 安全考虑

- 所有输入 IR 均视为不可信数据。
- 不执行、编译或加载被分析项目代码。
- 对超大 CFG 与 φ 爆炸设置资源边界。
- 诊断不包含与当前分析无关的环境敏感信息。

## 测试设计

- 单元测试覆盖五种语言函数体的 CFG 构建、φ 插入与重命名。
- 正例：分支合并处正确插入 φ，def-use 链正确。
- 负例：不产生错误的 φ 或错误的值来源。
- 边界：无分支线性函数、深层嵌套循环、不可达块与空函数体。
- 变体：等价控制流的不同来源 IR 产生一致 SSA。
- 回归：每个已修复 SSA 缺陷对应独立 fixture。
- 确定性：相同 IR 产生一致的 CFG 与 SSA。

由于 SSA 是结构性构建而非漏洞检测，正报定义为正确构建的函数体，误报定义为产生错误 CFG、错误 φ 或错误值来源的函数体。误报率按函数统计，最低标准为存在正报且误报率低于 30%。

## 验收标准

- 五种语言的函数体均能构建 CFG 与 SSA。
- φ 插入、重命名与值来源标注正确且通过测试。
- 不可达块被显式标记，def-use 链可查询。
- 存在有效正报，误报率低于 30%。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确函数内 SSA 职责与只做函数内分析的边界。
- ✅ 完成 CFG、SSA 值、φ 函数与值来源的数据结构设计。
- ✅ 完成 CFG 构建、支配计算与 φ 插入流程设计。
- ✅ 设计单元测试与误报口径。
- ❌ 创建 `07-ssa/` 包结构。
- ❌ 创建五种语言测试 fixture 与失败测试。
- ❌ 实现 CFG 构建、支配计算与 SSA 重命名。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计正报、误报与性能基线。
