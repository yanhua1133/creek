# 局部指针分析模块设计

## 结论

局部指针分析模块在函数内 SSA（07）之上识别对象创建、字段存储和字段加载点，为对象创建点分配抽象对象标识，建立指针指向关系，并跟踪 `new`、`store`、`load` 之间的传播以解析字段级别名关系。核心目标是保证被别名连接的值在数据流上连续可追踪，避免污点分析等下游数据流在字段读写处断链。

模块坚持低误报：无法确定指向的指针保留候选对象集合并显式记录不确定来源，不臆造指向。分析是局部的，以函数为主要范围，跨函数指向依据 Call Graph 与污点分析按需求解时再行扩展，本模块不做全程序指针分析。

本模块区分两类别名，避免概念混淆：**值别名**指两个 SSA 值指向同一抽象对象，由 `PointsToSet` 表达；**字段别名**指字段存储与后续加载之间的读写依赖（`store` 后 `load` 读到同一值），由 `FieldAccess` 与 `LoadSources` 表达，锚定在 04 的 `IRLoadField`/`IRStoreField` 节点上。数据流连续性承诺分级：函数内的值别名与字段别名完整（级别一）；通过参数与返回值传递的对象引用依赖 Call Graph 在跨函数场景保持指向（级别二，由污点分析按需触发）；跨函数全局对象的连续追踪属于后续全程序分析，本模块不承诺（级别三）。任一级别下无法确定的指向都保留候选并标注，既不制造断链也不制造假别名。

```text
08-pointer-analysis/
├── object.go       # 抽象对象与稳定标识定义
├── pointsto.go     # 指向关系与字段别名定义
├── analyze.go      # 基于 SSA 的指向传播
├── field.go        # 字段 store/load 匹配与别名解析
├── diagnostic.go   # 指针分析诊断
└── test/
    ├── unit/
    │   ├── fixture/{c,cpp,java,go,python}/{positive,negative,boundary,regression}
    │   ├── pointsto_test.go
    │   ├── field_alias_test.go
    │   └── dataflow_continuity_test.go
    └── e2e/
        └── pointer_e2e_test.go
```

## 目标

- 识别对象创建、字段存储与字段加载点。
- 为对象创建点分配抽象对象标识。
- 建立变量到抽象对象的指向关系。
- 解析字段级 store 与 load 之间的别名，保证数据流连续。
- 对无法确定的指向保留候选并记录来源。
- 保证相同 SSA 产生确定性的指向关系。

## 非目标

- 不做全程序过程间指针分析。
- 不做精确形状分析或分离逻辑。
- 不做污点传播，本模块只提供别名事实。
- 不处理运行时反射与完全动态的对象创建，这类保留候选或未知。

## 使用场景

- 为污点分析提供字段读写处的别名，保证污点不断链。
- 查询某个变量可能指向的抽象对象集合。
- 查询某次字段加载可能读到的存储来源。
- 从指向关系回溯到 SSA 值与源码位置。

## 模块边界

### 局部指针分析负责

- 抽象对象建模与指向关系建立。
- 字段 store/load 匹配与别名解析。
- 数据流连续性保证与不确定来源记录。

### 局部指针分析不负责

- 全程序过程间指针求解。
- 污点、常量或区间等其他数据流。
- 类型推导与调用解析。

## 核心数据结构

```go
// ObjectID 是抽象对象在分析中的稳定标识。
type ObjectID uint32

// AbstractObject 表示一个由创建点抽象出的对象。
type AbstractObject struct {
    // ID 是该抽象对象的稳定标识。
    ID ObjectID
    // CreationSite 是创建该对象的 IR 节点标识。
    CreationSite IRID
    // TypeSym 是该对象的类型符号标识，未知时为空。
    TypeSym SymbolID
}

// PointsToSet 表示一个 SSA 值可能指向的抽象对象集合及其确定性。
type PointsToSet struct {
    // Value 是持有该指向集合的 SSA 值标识。
    Value ValueID
    // Objects 是可能指向的抽象对象标识集合。
    Objects []ObjectID
    // Precise 标记该指向是否唯一确定，非唯一时为候选。
    Precise bool
    // Reason 是候选或未知指向的依据说明。
    Reason string
}

// FieldAccess 表示一次字段存储或加载。
type FieldAccess struct {
    // Node 是该访问的 IR 节点标识。
    Node IRID
    // Base 是被访问对象的 SSA 值标识。
    Base ValueID
    // Field 是被访问的字段名称。
    Field string
    // IsStore 标记该访问是存储还是加载。
    IsStore bool
    // StoredValue 是存储访问写入的值标识，加载访问时为空。
    StoredValue ValueID
}

// PointerFacts 聚合抽象对象、指向关系与字段别名，供下游查询。
type PointerFacts interface {
    // Object 返回指定标识的抽象对象，不存在时返回 false。
    Object(id ObjectID) (AbstractObject, bool)
    // PointsTo 返回指定 SSA 值的指向集合。
    PointsTo(v ValueID) (PointsToSet, bool)
    // LoadSources 返回一次字段加载可能读到的存储来源。
    LoadSources(load IRID) []FieldAccess
}
```

## 对外接口

```go
// Analyzer 定义在函数内 SSA 之上做局部指针分析的能力。
type Analyzer interface {
    // Analyze 消费一个函数体的 SSA，产出其指针别名事实与诊断。
    Analyze(ctx context.Context, fn FunctionSSA) (PointerFacts, []Diagnostic, error)
}
```

接口约束：

- `Analyze` 以单个函数体 SSA 为主要范围。
- 无法确定的指向通过 `PointsToSet.Precise` 与 `Reason` 表达，不作为 Go `error` 返回。
- Go `error` 仅用于系统性失败。
- 相同 SSA 产生确定性的 `ObjectID` 分配与指向集合排序。
- 顶层入口 `AnalyzePointers` 对项目内函数体逐一分析。

## 处理流程

1. 遍历 SSA 识别对象创建点，为每个创建点分配抽象对象。
2. 建立变量到抽象对象的初始指向关系。
3. 沿 def-use 链传播指向，处理赋值与拷贝。
4. 匹配字段 store 与 load：当基指针指向集合相交且字段相同时建立别名，使 load 能读到对应 store 的值。
5. 对指向不唯一的情形保留候选集合并记录来源。
6. 校验被别名连接的值在数据流上连续，输出 `PointerFacts` 与诊断。

## 错误处理

- 无法确定的指向保留候选并标注 `Reason`，不臆造。
- 字段访问基指针未知时记录诊断并保守处理，不制造断链也不制造假别名。
- SSA 缺失导致无法分析时记录诊断，不中止其余函数处理。
- 内部不变量破坏且无法恢复时返回致命错误。

## 性能与资源限制

- 抽象对象与指向集合使用稳定标识关联，避免难以回收的引用环。
- 指向传播使用工作表迭代至定点，设置可配置迭代上限。
- 分析过程接受上下文取消信号。
- 对指向集合爆炸设置可配置上限，超限时保守合并并记录。
- 内置计时与 profile 埋点并可输出指向关系中间摘要，默认关闭。
- 性能基准在获得真实 fixture 后确定。

## 安全考虑

- 所有输入 SSA 均视为不可信数据。
- 不执行、编译或加载被分析项目代码。
- 对对象与指向集合爆炸设置资源边界。
- 诊断不包含与当前分析无关的环境敏感信息。

## 测试设计

- 单元测试覆盖五种语言的对象创建、指向传播与字段别名。
- 正例：`new`、`store`、`load` 序列正确建立别名，load 能追溯到对应 store。
- 数据流连续性专项：构造对象字段写后读，断言下游可沿别名连续追踪，不断链。
- 负例：不产生无依据的别名，不同对象的同名字段不误连。
- 边界：自引用对象、循环别名、深层字段嵌套与空指向。
- 变体：等价语义的不同写法产生一致别名。
- 回归：每个已修复别名缺陷对应独立 fixture。
- 确定性：相同 SSA 产生一致指向关系。

局部指针分析是分析流水线的**中间步骤**，不产出漏洞报告，因此不适用端到端误报率阈值（见 AGENTS.md 第 8 条），以**正确性**口径验收：别名与指向关系必须与 golden fixture 精确一致，系统失败必须为零，不得产生错误结构。正报为正确产出的别名与指向结构，负例为不产生无依据的错误别名或错误指向。最低标准为五种语言均有正报，任何结构错误都是必须修复的缺陷，目标是零错误。

## 验收标准

- 五种语言均能建立对象指向与字段别名关系。
- 数据流连续性专项测试通过：字段写后读可连续追踪。
- 无法确定的指向保留候选并记录来源。
- 五种语言均有有效正报，别名与指向关系与 golden fixture 精确一致，系统失败为零，无错误结构（零错误）。
- `go test`、`go test -race`、静态检查与构建全部通过。

## 实施进度

- ✅ 明确局部指针分析职责与保证数据流连续的核心目标。
- ✅ 完成抽象对象、指向关系与字段访问的数据结构设计。
- ✅ 完成指向传播与字段别名解析流程设计。
- ✅ 设计单元测试、数据流连续性专项与误报口径。
- ❌ 创建 `08-pointer-analysis/` 包结构。
- ❌ 创建五种语言测试 fixture 与失败测试。
- ❌ 实现指向传播与字段别名解析。
- ❌ 运行单元测试、Race Detector 与 E2E 测试。
- ❌ 统计五种语言的正报、误报与性能基线。
