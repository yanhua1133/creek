# Parser 模块设计

## 结论

Parser 模块使用官方 Go 绑定 `github.com/tree-sitter/go-tree-sitter`，分别接入 Tree-sitter 官方维护的 C、C++、Java、Go 和 Python grammar。模块只负责源码识别、语法解析、错误恢复、语法树访问和源码位置映射，不承担预处理展开、类型推导、名称解析、common AST 或 IR 构建。

测试严格拆分为两个独立目录：

```text
01-parser/
├── parser.go
├── language.go
├── diagnostic.go
├── tree.go
├── node.go
└── test/
    ├── unit/
    │   ├── parser_test.go
    │   ├── language_test.go
    │   ├── diagnostic_test.go
    │   └── fixture/
    │       ├── c/
    │       ├── cpp/
    │       ├── java/
    │       ├── go/
    │       └── python/
    └── e2e/
        ├── parser_e2e_test.go
        ├── project_manifest_test.go
        └── fixture/
            ├── projects.lock.json
            └── projects/
                ├── redis/              # git submodule，C
                ├── protobuf/           # git submodule，C++
                ├── junit-framework/    # git submodule，Java
                ├── go-git/             # git submodule，Go
                └── django/              # git submodule，Python
```

单元测试使用项目自有的最小独立源码 fixture；E2E 测试使用固定提交的真实 GitHub 开源项目 submodule。任何外部项目在加入前必须同时满足：GitHub Star 数不低于 200，目标语言源码文件不低于 100 个。E2E 测试运行时不得访问网络，所有输入都来自已初始化的 submodule。

## 目标

- 使用 Tree-sitter 统一解析 C、C++、Java、Go 和 Python 源文件。
- 对合法源码生成稳定、可遍历、可回溯源码位置的 concrete syntax tree。
- 对不完整或错误源码返回部分语法树及结构化诊断，而不是直接失败。
- 所有 `.h`、`.hh`、`.hpp`、`.hxx` 等头文件都必须解析，不得因语言无法预先确定而跳过。
- `.h` 文件必须由 Parser 自动使用 C 与 C++ grammar 双重解析，不得读取编译信息或项目配置决定语言。
- 隔离 Tree-sitter 对象生命周期，禁止下游模块直接管理 C 内存。
- 支持项目级并发解析，并保证结果确定性。
- 为 common AST 提供稳定、与 grammar 版本变化隔离的语法树访问接口。
- 使用小型 fixture 完成精确单元测试，使用大型真实项目完成 E2E 兼容性验证。

## 非目标

- 不展开 C/C++ 宏，不执行 C 预处理器。
- 不解析编译数据库、构建脚本或依赖下载流程；项目加载能力另行设计。
- 不执行类型检查、名称解析、符号绑定或调用目标解析。
- 不把 Tree-sitter concrete syntax tree 直接定义为 common AST。
- 第一版不实现编辑器场景的增量更新接口，但数据结构必须保留后续接入旧语法树和 `InputEdit` 的空间。
- 不编译或运行被分析项目中的任何代码。
- E2E 测试不负责验证外部项目自身能够完整构建。

## 使用场景

- 解析单个源码文件并获取根节点、子节点、字段名和源码范围。
- 批量解析一个代码库中的 C、C++、Java、Go 或 Python 文件。
- 在存在局部语法错误时继续提取可恢复的声明和语句结构。
- 将语法节点稳定映射到原始文件、字节区间和行列位置。
- 为下一阶段 common AST 转换器提供按语言分派的语法树输入。

## 模块边界

### Parser 负责

- 注册并管理五种语言 grammar。
- 根据文件扩展名自动选择 grammar，并对 `.h` 文件执行 C 与 C++ 双重解析。
- 按请求解析 UTF-8 源码。
- 包装 Tree-sitter tree、node、point 和 range。
- 收集 `ERROR` 节点、`MISSING` 节点和解析失败诊断。
- 提供确定性的节点遍历、字段访问和源码切片能力。
- 管理 Parser、Tree、TreeCursor 等底层对象的释放。

### Parser 不负责

- C/C++ 宏求值、条件编译分支选择和头文件展开。
- Java classpath、Go module、Python import path 或 C/C++ include path 解析。
- 语义级声明、引用、类型或作用域计算。
- 过滤业务上“不重要”的语法节点。
- 把无法解析的文件伪装成成功结果。

## 技术选型

### Tree-sitter 绑定

使用官方 Go 绑定：

```text
github.com/tree-sitter/go-tree-sitter
```

原因：

- 与 Tree-sitter 官方组织和 grammar Go binding 体系一致。
- 支持直接使用各 grammar 仓库的 `bindings/go` 包。
- 提供 Parser、Tree、Node、TreeCursor、Query 和增量解析相关能力。
- 对底层分配对象要求显式 `Close`，适合在本模块集中封装生命周期。

### Grammar

使用以下官方 grammar，并在 `go.mod` 中固定明确版本：

| 语言 | Grammar |
|---|---|
| C | `github.com/tree-sitter/tree-sitter-c/bindings/go` |
| C++ | `github.com/tree-sitter/tree-sitter-cpp/bindings/go` |
| Java | `github.com/tree-sitter/tree-sitter-java/bindings/go` |
| Go | `github.com/tree-sitter/tree-sitter-go/bindings/go` |
| Python | `github.com/tree-sitter/tree-sitter-python/bindings/go` |

Tree-sitter runtime 与 grammar 的 ABI 必须在启动或测试阶段校验。版本升级必须单独提交，并完整运行单元测试和 E2E 测试，禁止使用浮动的 `latest` 版本作为可复现构建依据。

### 链接方式

第一版使用 grammar Go binding 的静态 CGO 集成方式，不在运行时下载或动态加载 grammar。这样可以保证：

- 构建产物包含确定的 grammar 版本。
- 测试不依赖网络或外部共享库路径。
- grammar ABI 不会因运行环境中的共享库变化而漂移。

代价是构建环境必须启用 CGO 并具有可用的 C 编译器。无 CGO 的纯 Go 或 Wasm 方案不属于第一版范围。

## 核心数据结构

```go
// Language 表示源码使用的编程语言。
type Language uint8

const (
    // LanguageC 表示 C 语言。
    LanguageC Language = iota + 1
    // LanguageCPP 表示 C++ 语言。
    LanguageCPP
    // LanguageJava 表示 Java 语言。
    LanguageJava
    // LanguageGo 表示 Go 语言。
    LanguageGo
    // LanguagePython 表示 Python 语言。
    LanguagePython
)

// SourceFile 表示一次解析所需的源码文件及其基本信息。
type SourceFile struct {
    // ID 是文件在当前分析任务中的稳定标识。
    ID FileID
    // Path 是用于诊断和源码定位的项目内文件路径。
    Path string
    // Content 保存待解析的 UTF-8 源码内容。
    Content []byte
}

// ParseOptions 控制单次源码解析的可选行为。
type ParseOptions struct {
    // CollectDiagnostics 指定是否收集语法错误和缺失节点诊断。
    CollectDiagnostics bool
}

// ParseResult 保存语法树和解析过程中产生的诊断。
type ParseResult struct {
    // Language 是 Parser 根据扩展名和解析结果确定的实际语言。
    Language Language
    // Tree 是本次解析生成的语法树。
    Tree SyntaxTree
    // Diagnostics 保存按源码位置排序的解析诊断。
    Diagnostics []Diagnostic
}

// Diagnostic 表示一个可以定位到源码范围的解析问题。
type Diagnostic struct {
    // Kind 表示诊断问题的类型。
    Kind DiagnosticKind
    // Message 是供开发者阅读的简洁问题说明。
    Message string
    // Range 是问题对应的源码范围。
    Range SourceRange
    // NodeKind 是产生该诊断的 Tree-sitter 节点类型。
    NodeKind string
}
```

上述结构表示接口方向，具体字段在实现时可以根据测试细化，但必须满足以下不变量：

- `Language` 不得使用字符串自由输入，防止拼写和大小写产生隐式分支。
- `SourceFile.Path` 必须包含可识别的源码扩展名，Parser 不读取外部编译信息或项目配置补充语言。
- `SourceFile.Content` 在语法树存活期间必须保持不可变。
- `SyntaxTree` 对底层 Tree-sitter tree 具有唯一所有权，并提供幂等的 `Close`。
- 对外节点类型不得暴露底层 C 指针，也不得允许节点在所属树关闭后继续访问。
- 所有节点范围同时包含字节偏移和零基行列位置。
- `ParseResult.Tree` 非空不代表语法完全正确，必须结合诊断判断。
- Tree-sitter 节点 ID 不能作为跨进程或持久化身份；common AST 必须生成自己的稳定 ID。

## 对外接口

```go
// Parser 定义单个源码文件的解析能力。
type Parser interface {
    // Parse 解析指定源码文件并返回语法树和诊断。
    Parse(ctx context.Context, file SourceFile, options ParseOptions) (ParseResult, error)
}

// SyntaxTree 表示一次解析生成且需要显式关闭的语法树。
type SyntaxTree interface {
    // Language 返回该语法树对应的编程语言。
    Language() Language
    // FileID 返回该语法树对应的文件标识。
    FileID() FileID
    // Root 返回语法树的根节点。
    Root() Node
    // Close 释放语法树持有的底层资源。
    Close() error
}

// Node 提供对语法树节点的只读访问能力。
type Node interface {
    // Kind 返回当前节点的语法类型名称。
    Kind() (string, error)
    // IsNamed 报告当前节点是否是 grammar 定义的命名节点。
    IsNamed() (bool, error)
    // IsError 报告当前节点是否表示无法正常归类的语法错误。
    IsError() (bool, error)
    // IsMissing 报告当前节点是否是 Tree-sitter 补出的缺失语法元素。
    IsMissing() (bool, error)
    // Range 返回当前节点对应的源码范围。
    Range() (SourceRange, error)
    // ChildCount 返回当前节点的直接子节点数量。
    ChildCount() (int, error)
    // Child 返回指定位置的直接子节点。
    Child(index int) (Node, bool, error)
    // ChildByField 返回指定字段对应的直接子节点。
    ChildByField(name string) (Node, bool, error)
    // FieldNameForChild 返回指定子节点对应的字段名称。
    FieldNameForChild(index int) (string, error)
    // Text 返回当前节点覆盖的源码内容。
    Text() ([]byte, error)
}
```

接口约束：

- `Parse` 必须接受 `context.Context`，取消后尽快结束尚未开始或仍在执行的批量任务。
- 单文件语法错误通过 `Diagnostics` 表达，不作为 Go `error` 返回。
- Go `error` 仅用于无 grammar、不支持的文件扩展名、生命周期错误、上下文取消或底层无法产生语法树等系统性失败。
- `Node` 的访问方法必须检测所属语法树是否已经关闭，关闭后统一返回 `ErrTreeClosed`。
- `Text` 返回只读视图或副本，调用方不得修改 Parser 持有的源码。
- 遍历顺序固定为源码顺序，不得依赖 map 顺序。
- 后续可以新增批量解析器，但其语义必须等价于逐文件调用 `Parse`。

## 语言识别

### 扩展名映射

| 语言 | 默认扩展名 |
|---|---|
| C | `.c` |
| C/C++ 头文件 | `.h`，同时使用 C 与 C++ grammar 解析 |
| C++ | `.cc`、`.cpp`、`.cxx`、`.c++`、`.hh`、`.hpp`、`.hxx`、`.ipp`、`.tpp` |
| Java | `.java` |
| Go | `.go` |
| Python | `.py`、`.pyi` |

`.h` 文件必须解析，处理规则固定如下：

1. 使用 C grammar 完整解析一次。
2. 使用 C++ grammar 完整解析一次。
3. 优先选择 `ERROR` 与 `MISSING` 节点总数更少的结果。
4. 数量相同时，优先选择错误节点覆盖源码字节数更少的结果。
5. 仍然相同时选择 C++ 结果，因为 Tree-sitter C++ grammar 建立在 C grammar 之上，并增加 C++ 语法规则。
6. 选择过程必须记录两次解析的错误数量和最终语言，便于测试和问题定位。

`.hh`、`.hpp`、`.hxx`、`.ipp` 和 `.tpp` 直接使用 C++ grammar，`.c` 直接使用 C grammar。整个过程不读取 `compile_commands.json`、构建脚本、include path、项目配置或调用方语言提示。

Parser 模块使用同一个实现和同一套接口处理 C 与 C++；底层 Tree-sitter Parser 类型相同，但解析前需要设置对应的 C 或 C++ grammar。

## 处理流程

1. 校验上下文、文件 ID、路径和源码大小。
2. 根据扩展名确定单 grammar 解析或 `.h` 双 grammar 解析。
3. 创建或从受控池中取得对应 grammar 的 Parser 实例。
4. 设置 grammar 并解析 UTF-8 字节。
5. `.h` 文件比较 C 与 C++ 两次解析结果并选择错误更少的语法树。
6. 关闭未被选择的语法树，包装选中语法树并转移底层 tree 所有权。
7. 遍历选中语法树并收集 `ERROR` 与 `MISSING` 诊断。
8. 校验根节点范围、子节点范围和源码边界不变量。
9. 返回实际语言、语法树、诊断和统计信息。
10. 调用方完成消费后显式关闭语法树。

并发解析时，每个工作任务独占一个 Parser 实例。不得让多个 goroutine 同时操作同一个底层 Parser、TreeCursor 或 QueryCursor。

## 错误处理

### 可恢复语法错误

- Tree-sitter `ERROR` 节点转换为 `DiagnosticSyntaxError`。
- Tree-sitter `MISSING` 节点转换为 `DiagnosticMissingToken`。
- 同一范围内的重复诊断必须按明确规则去重。
- 即使存在语法诊断，只要 Tree-sitter 返回 tree，就必须保留部分语法树。

### 系统错误

- 不支持的文件扩展名返回 `ErrUnsupportedExtension`，但 `.h` 必须进入 C 与 C++ 双 grammar 解析流程。
- `.h` 的 C 与 C++ 两次解析都发生系统性失败时返回 `ErrHeaderParseFailed`。
- grammar ABI 不兼容返回 `ErrIncompatibleGrammar`。
- 上下文取消返回包装后的 `context.Canceled` 或 `context.DeadlineExceeded`。
- 底层对象创建失败或返回空 tree 时返回明确错误，不得 panic。

### 生命周期错误

- 所有底层资源必须显式关闭，不依赖终结器完成正常回收。
- 重复关闭必须安全并返回稳定结果。
- 关闭语法树后访问节点必须返回失败，不得访问已释放内存。
- 单元测试必须覆盖正常关闭、重复关闭和关闭后访问。

## 性能与资源限制

- 第一版默认仅接受 UTF-8 源码。
- 单文件大小限制必须可配置，默认值在基准测试后确定，设计阶段不虚构指标。
- 批量解析并发度必须可配置，默认不超过 `runtime.GOMAXPROCS(0)`。
- 不得为节点树建立完整的 Go 对象镜像；节点包装按需创建，避免重复占用内存。
- 解析结果不得无必要复制完整源码。
- E2E 测试必须记录文件数、总字节数、总耗时、最大单文件耗时和峰值内存的可获得指标。
- 性能退化阈值在获得首轮稳定基线后写入设计文档并固定，未建立基线前不得声明性能验收通过。

## 安全考虑

- 所有源码和路径均视为不可信输入。
- 禁止执行、编译、加载或调用被分析项目代码。
- 路径遍历必须限定在明确的项目根目录下，不跟随逃逸根目录的符号链接。
- 对超大文件、超深语法树和大量错误节点设置可配置资源边界。
- 所有 Tree-sitter C 对象必须按照所有权规则释放，重点防止 use-after-free、double-free 和泄漏。
- C/C++ 预处理指令只作为语法节点保留，不执行其中的文件包含或命令。
- E2E submodule 固定到已审核提交，不自动跟随远端分支。

## 测试设计

### 测试分层

Parser 测试分为完全独立的单元测试与 E2E 测试：

| 测试层 | 目录 | 输入来源 | 默认执行 |
|---|---|---|---|
| 单元测试 | `01-parser/test/unit/` | 项目自有最小 fixture | 是 |
| E2E 测试 | `01-parser/test/e2e/` | Git submodule 中的真实项目 | 否，使用 `e2e` build tag |

单元测试与 E2E 测试不得共享隐式全局状态。公共测试辅助代码只能放在明确的测试辅助包中，不能把测试数据嵌入 Go 字符串。

### 单元测试目录

```text
01-parser/test/unit/
├── parser_test.go
├── language_test.go
├── diagnostic_test.go
├── lifecycle_test.go
├── determinism_test.go
└── fixture/
    ├── c/
    │   ├── positive/
    │   ├── negative/
    │   ├── boundary/
    │   └── regression/
    ├── cpp/
    │   ├── positive/
    │   ├── negative/
    │   ├── boundary/
    │   └── regression/
    ├── java/
    │   ├── positive/
    │   ├── negative/
    │   ├── boundary/
    │   └── regression/
    ├── go/
    │   ├── positive/
    │   ├── negative/
    │   ├── boundary/
    │   └── regression/
    └── python/
        ├── positive/
        ├── negative/
        ├── boundary/
        └── regression/
```

单元测试至少覆盖：

- 五种语言的最小合法文件、声明、控制流、调用、类型和注释。
- C 预处理指令、声明器、函数指针和 GNU 常见扩展样例。
- C++ 模板、命名空间、重载、lambda、类、结构化绑定和现代标准语法。
- Java 包、导入、泛型、注解、内部类、lambda、record 和 module 声明。
- Go package、import、泛型、接口、方法、复合字面量、goroutine 和 channel。
- Python import、函数、类、装饰器、生成器、推导式、异步语法、模式匹配、海象运算符、类型标注和类型参数。
- Python 缩进错误、混合制表符与空格、续行、三引号字符串和 `.pyi` stub。
- 空文件、仅注释文件、无尾换行、UTF-8 标识符和不同换行符。
- `ERROR`、`MISSING`、错误恢复、诊断范围和诊断去重。
- `.h` 使用 C grammar 更优、C++ grammar 更优和两者得分相同的三类自动选择场景。
- `.h`、`.hh`、`.hpp`、`.hxx`、`.ipp`、`.tpp` 头文件均能生成非空语法树。
- 未知扩展名返回明确错误，不读取编译信息或项目配置进行补救。
- 节点源码范围包含关系、子节点顺序和字段访问。
- 正常关闭、重复关闭、关闭后访问和并发解析。
- 相同输入重复解析产生相同的规范化树摘要与诊断顺序。

### E2E 测试目录

```text
01-parser/test/e2e/
├── parser_e2e_test.go
├── project_manifest_test.go
├── metrics_test.go
└── fixture/
    ├── projects.lock.json
    └── projects/
        ├── redis/
        ├── protobuf/
        ├── junit-framework/
        ├── go-git/
        └── django/
```

`fixture/projects/` 下每个目录必须是 Git submodule，禁止复制源码快照代替 submodule。`.gitmodules` 必须记录标准 HTTPS 仓库地址，主仓库固定 submodule commit。

### E2E 开源项目

以下 Star 数于 2026 年 7 月 27 日核验，均显著高于 200。源码文件数必须在实际添加 submodule 后由离线清单脚本精确统计并写入 `projects.lock.json`；任何项目目标语言文件数少于 100 时，测试必须立即失败并更换项目。

| 语言 | Submodule 路径 | GitHub 项目 | 核验 Star 数 | 计入源码扩展名 |
|---|---|---|---:|---|
| C | `fixture/projects/redis` | `redis/redis` | 约 74,900 | `.c`、`.h` |
| C++ | `fixture/projects/protobuf` | `protocolbuffers/protobuf` | 约 71,300 | `.cc`、`.cpp`、`.cxx`、`.h`、`.hh`、`.hpp`、`.hxx`、`.inc` |
| Java | `fixture/projects/junit-framework` | `junit-team/junit-framework` | 约 7,000 | `.java` |
| Go | `fixture/projects/go-git` | `go-git/go-git` | 约 7,400 | `.go` |
| Python | `fixture/projects/django` | `django/django` | 约 88,200 | `.py`、`.pyi` |

选择依据：

- Star 数远高于最低线，短期波动不会使项目跌破 200。
- 每个仓库都包含超过 100 个对应语言源码文件，且包含真实工程中的复杂语法。
- 五个项目覆盖网络服务、序列化系统、测试框架、Git 实现和 Web 框架，语法分布差异明显。
- 项目规模足以暴露 grammar 兼容性、资源占用、错误恢复和文件发现问题。

### E2E 项目锁定清单

`projects.lock.json` 至少记录：

```json
{
  "verified_at": "2026-07-27",
  "projects": [
    {
      "language": "c",
      "path": "fixture/projects/redis",
      "repository": "redis/redis",
      "commit": "固定的完整提交哈希",
      "stars_at_verification": 74900,
      "source_file_count": 0,
      "extensions": [".c", ".h"],
      "excluded_paths": []
    }
  ]
}
```

表中只展示一个对象结构，实际清单必须包含五个项目。`source_file_count` 在添加 submodule 时由脚本计算并替换为真实值，禁止保留零值。Star 数只在引入或升级 submodule 时联网核验；常规测试只读取锁定清单并离线校验 commit 与源码文件数，避免 CI 依赖 GitHub API。

### E2E 文件计数规则

- 只统计 `git ls-files` 返回的已跟踪普通文件。
- 按项目清单中声明的扩展名统计目标语言代码。
- 默认排除 `.git/`、第三方 vendored 目录、构建产物和生成产物；每个排除路径必须写入锁定清单并说明原因。
- 不得为了满足误报率或运行时间任意排除解析失败文件。
- 每个项目过滤后目标语言源码文件必须不少于 100 个。
- 测试必须校验当前 submodule HEAD 与锁定 commit 完全一致。

### E2E 执行规则

1. 检查五个 submodule 是否完成初始化；缺失时明确失败并输出初始化命令。
2. 校验 submodule commit、项目目录和锁定清单一致。
3. 按扩展名收集文件并校验每个项目文件数不少于 100。
4. 对所有纳入文件执行解析，不允许抽样代替全量测试。
5. 汇总成功树、语法诊断、系统错误、panic、超时和资源指标。
6. 按项目、语言和错误节点类型输出排序稳定的报告。
7. 任一文件 panic、内存安全错误或无法生成语法树时测试失败。

### 测试命令

```text
go test ./01-parser/test/unit/...
go test -race ./01-parser/test/unit/...
go test -tags=e2e ./01-parser/test/e2e/...
go test -race -tags=e2e ./01-parser/test/e2e/...
```

Submodule 初始化命令：

```text
git submodule update --init --recursive 01-parser/test/e2e/fixture/projects
```

## 误报与正报定义

Parser 阶段必须给出适用于语法解析的明确统计口径：

- 正报：故意包含语法错误或缺失 token 的 fixture 被正确识别，并给出范围有效的对应诊断。
- 误报：经过 fixture 标注或项目自身编译流程确认合法的源码，被 Parser 报告为语法错误或缺失 token。
- 漏报：故意构造的语法错误 fixture 未产生预期诊断。
- 系统失败：没有生成语法树、发生 panic、超时或生命周期错误；系统失败不计入普通误报，而是直接导致测试失败。

误报率按文件统计：

```text
误报率 = 产生非预期语法诊断的合法文件数量 / 合法文件总数 × 100%
```

最低标准：

- 五种语言都必须存在正报。
- 总体及每种语言的误报率都必须低于 30%。
- 所有故意错误 fixture 必须至少有一个被正确识别，漏报不能通过删除 fixture 规避。
- E2E 中所有纳入文件必须生成非空根节点，系统失败数量必须为零。
- 目标值为每种语言 E2E 非预期语法诊断文件率低于 5%；目标值未达到时记录为待改进，但不得把低于 30% 的最低标准改宽。

## 验收标准

- C、C++、Java、Go 和 Python grammar 均通过固定版本接入并完成 ABI 校验。
- 单元测试与 E2E 测试分别位于独立目录。
- 所有测试源码均为独立 fixture 文件，不存在内嵌被测源码字符串。
- 单元测试覆盖合法解析、错误恢复、语言识别、生命周期、并发和确定性。
- `.h` 文件全部执行 C 与 C++ 双 grammar 解析，并按固定错误评分规则选择结果。
- `.hh`、`.hpp`、`.hxx`、`.ipp`、`.tpp` 文件全部使用 C++ grammar 解析。
- C/C++ 头文件解析不得读取编译数据库、构建脚本、include path、项目配置或调用方语言提示。
- E2E 目录包含五个固定提交的 Git submodule。
- 每个 E2E 项目在加入时 Star 数不低于 200，且目标语言源码文件不少于 100。
- E2E 测试全量解析纳入范围的源码文件，不使用抽样替代。
- 五种语言均有有效正报，系统失败为零，总体及分语言误报率低于 30%。
- `go test`、`go test -race`、E2E 测试、静态检查和项目构建全部通过。
- 测试报告包含文件数、成功树数、诊断文件数、系统失败数、正报数、误报数和误报率。

## 实施进度

- ✅ 明确 Parser 模块职责与边界。
- ✅ 确定使用官方 `go-tree-sitter` 绑定与五种官方 grammar。
- ✅ 完成核心数据结构、接口、生命周期和错误处理设计。
- ✅ 单独设计 `01-parser/test/unit/` 单元测试目录。
- ✅ 单独设计 `01-parser/test/e2e/` E2E 测试目录。
- ✅ 选定五个 Star 数高于 200 的 GitHub 开源项目。
- ✅ 设计 E2E submodule、锁定清单与源码文件数校验规则。
- ✅ 定义 Parser 阶段的正报、误报、漏报和系统失败口径。
- ❌ 创建 Go module 与 Parser 包结构。
- ❌ 添加并固定 Tree-sitter runtime 与 grammar 依赖版本。
- ❌ 创建单元测试 fixture 与首批失败测试。
- ❌ 实现 Parser 与语法树包装。
- ❌ 添加五个 E2E Git submodule 并记录真实 commit、Star 数和源码文件数。
- ❌ 运行单元测试、Race Detector 和 E2E 测试。
- ❌ 统计五种语言的正报、误报、漏报和性能基线。
