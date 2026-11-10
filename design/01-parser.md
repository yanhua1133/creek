# Parser 模块设计

## 结论

Parser 模块使用官方 Go 绑定 `github.com/tree-sitter/go-tree-sitter`，分别接入 Tree-sitter 官方维护的 C、C++、Java、Go 和 Python grammar。模块只负责源码识别、C/C++ 轻量预处理（条件编译分支选择、宏中和和错误驱动的宏占位符，均保持字节偏移不变）、语法解析、错误恢复、语法树访问和源码位置映射，不承担完整宏展开、类型推导、名称解析、common AST 或 IR 构建。

测试严格拆分为两个独立目录：

```text
01-parser/
├── parser.go
├── language.go
├── diagnostic.go
├── tree.go
├── preprocess.go
├── macro.go
├── node.go
└── test/
    ├── unit/
    │   ├── parser_test.go
    │   ├── language_test.go
    │   ├── diagnostic_test.go
    │   ├── preprocess_test.go
    │   ├── macro_test.go
    │   └── fixture/
    │       ├── c/
    │       ├── cpp/
    │       ├── java/
    │       ├── go/
    │       ├── python/
    │       └── macro/
    └── e2e/
        ├── parser_e2e_test.go
        ├── resource_e2e_test.go
        ├── support_test.go
        └── fixture/
            └── expected/
                ├── c-redis.json
                ├── cpp-protobuf.json
                ├── java-junit.json
                ├── go-go-git.json
                ├── python-django.json
                └── resource-baseline.json
```

单元测试使用项目自有的最小独立源码 fixture；E2E 测试读取根目录 `e2e-corpus/` 中固定提交的真实 GitHub 开源项目 submodule。任何外部项目在加入前必须同时满足：GitHub Star 数不低于 200，目标语言源码文件不低于 100 个。E2E 测试运行时不得访问网络，所有输入都来自已初始化的共享 submodule。

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

- 不执行完整的 C 预处理器：不处理 `#include`，不按编译构建的头文件顺序展开宏；只做保持字节偏移不变的条件编译分支选择和宏中和，见“条件编译分支选择”和“轻量容错宏预处理”。
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
- C/C++ 解析前的条件编译分支选择和宏中和。
- 包装 Tree-sitter tree、node、point 和 range。
- 收集 `ERROR` 节点、`MISSING` 节点和解析失败诊断。
- 提供确定性的节点遍历、字段访问和源码切片能力。
- 管理 Parser、Tree、TreeCursor 等底层对象的释放。

### Parser 不负责

- C/C++ 完整宏展开、头文件展开，以及依赖外部配置的条件求值。
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
    // ID 是调用方分配的文件编号，只要求在同一分析任务内唯一，用于在内存中关联节点、诊断和下游结果。
    ID FileID
    // Path 是以 / 分隔的项目内相对路径，用于语言识别、诊断定位和在快照中标识文件。
    Path string
    // Content 保存待解析的 UTF-8 源码内容。
    Content []byte
}

// ParseOptions 控制单次源码解析的可选行为。
type ParseOptions struct {
    // CollectDiagnostics 指定是否收集语法错误和缺失节点诊断。
    CollectDiagnostics bool
    // RawSource 为真时直接解析原始源码，不做任何 C/C++ 预处理。
    RawSource bool
    // ConditionalOnly 为真时只做条件编译分支选择，不做宏中和，用于对照和排查。
    ConditionalOnly bool
    // Macros 是项目级宏定义索引，为空时宏中和只使用本文件内的定义。
    Macros *MacroIndex
    // CollectMetrics 指定是否记录各处理环节的耗时，默认关闭。
    CollectMetrics bool
    // SkipPlaceholders 为真时不放置宏占位符，用于对照和排查。
    SkipPlaceholders bool
}

// ParseResult 保存语法树和解析过程中产生的诊断。
type ParseResult struct {
    // Language 是 Parser 根据扩展名和解析结果确定的实际语言。
    Language Language
    // Tree 是本次解析生成的语法树。
    Tree SyntaxTree
    // Diagnostics 保存按源码位置排序的解析诊断。
    Diagnostics []Diagnostic
    // Header 在解析 .h 头文件时记录双解析择优过程，其他情况为空。
    Header *HeaderResolution
    // Preprocess 记录最终选中语言对应的条件编译分支选择结果，非 C/C++ 文件或 RawSource 为真时为空。
    Preprocess *PreprocessReport
    // Macros 记录最终选中语言对应的宏中和结果，非 C/C++ 文件、RawSource 或 ConditionalOnly 为真时为空。
    Macros *MacroReport
    // Metrics 记录各处理环节耗时，CollectMetrics 为假时为空。
    Metrics *ParseMetrics
    // Placeholders 是最终选中语言对应的宏占位符，按源码顺序排列，与 Tree.Placeholders() 相同。
    Placeholders []Placeholder
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

文件标识规则：

- `FileID` 由调用方分配，Parser 不生成也不修改，原样交给 `SyntaxTree.FileID()`。它只保证在同一分析任务内唯一，不保证跨运行稳定：同一批文件换一种编号顺序，或增删一个文件，编号都可能变化。
- `Path` 由调用方提供，Parser 原样交给 `SyntaxTree.Path()`。它必须是以 `/` 分隔的项目内相对路径，不得是绝对路径，保证同一份源码在任何机器、任何运行中都相同。
- 内存中关联节点、诊断和下游结果时使用 `FileID`；快照、golden 和 expected 中标识文件时使用 `Path`，不得写入 `FileID`。

## 对外接口

```go
// Parser 定义单个源码文件的解析能力。
type Parser interface {
    // Parse 解析指定源码文件并返回语法树和诊断。
    Parse(ctx context.Context, file SourceFile, options ParseOptions) (ParseResult, error)
}

// BuildConditionalView 为 C/C++ 源码生成条件编译分支选择后的解析视图和报告，视图与原文件等长。
func BuildConditionalView(content []byte, lang Language) ([]byte, PreprocessReport)

// BuildMacroIndex 扫描项目头文件中的 #define，建立按目录距离查找的宏定义索引；非宏定义文件被忽略。
func BuildMacroIndex(files []SourceFile) *MacroIndex

// IsMacroDefinitionFile 报告路径是否属于宏定义索引收录的头文件类型。
func IsMacroDefinitionFile(path string) bool

// BuildMacroView 在条件编译视图上做宏中和，返回与原文件等长的解析视图和报告；index 可以为空。
func BuildMacroView(view []byte, path string, lang Language, index *MacroIndex) ([]byte, MacroReport)

// SyntaxTree 表示一次解析生成且需要显式关闭的语法树。
type SyntaxTree interface {
    // Language 返回该语法树对应的编程语言。
    Language() Language
    // FileID 返回该语法树对应的文件编号。
    FileID() FileID
    // Path 返回解析时传入的项目内相对路径。
    Path() string
    // Root 返回语法树的根节点。
    Root() Node
    // Placeholders 返回按源码顺序排列的宏占位符；落在其范围内的节点是占位结构，不是源码真实结构。
    Placeholders() []Placeholder
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
2. 根据扩展名确定单 grammar 解析或 `.h` 双 grammar 解析。C/C++ 按本次使用的 grammar 先做条件编译分支选择，再在其结果上做宏中和，得到与原文件等长的解析视图，见“条件编译分支选择”和“轻量容错宏预处理”。
3. 创建或从受控池中取得对应 grammar 的 Parser 实例。
4. 设置 grammar 并解析 UTF-8 字节；C/C++ 解析的是解析视图，语法树节点的 `Text` 仍返回原始源码，只有完全落在一次宏改写范围内的节点返回写入的关键字。
5. `.h` 文件比较 C 与 C++ 两次解析结果并选择错误更少的语法树。
6. 关闭未被选择的语法树，包装选中语法树并转移底层 tree 所有权。
7. 遍历选中语法树并收集 `ERROR` 与 `MISSING` 诊断。
8. 校验根节点范围、子节点范围和源码边界不变量。
9. 返回实际语言、语法树、诊断和统计信息。
10. 调用方完成消费后显式关闭语法树。

并发解析时，每个工作任务独占一个 Parser 实例。不得让多个 goroutine 同时操作同一个底层 Parser、TreeCursor 或 QueryCursor。

## 条件编译分支选择

### 结论

C/C++ 在交给 Tree-sitter 之前，先选择条件编译分支：每组 `#if`、`#ifdef`、`#ifndef`、`#elif`、`#else` 只保留一个分支，其余分支和全部条件编译指令行替换为空格。Tree-sitter 要求每个条件分支内部的语法结构完整，而 `#ifdef __cplusplus` 后单独出现 `extern "C" {` 这类写法会把一个语法结构拆到不同分支里，是 C/C++ 语法错误的主要来源之一。

### 规则

- 只把字节替换为空格，换行符 `\n` 和回车符 `\r` 保留，不插入、不删除任何字节。处理前后文件长度、全部字节偏移和行列位置完全一致，诊断和节点范围直接对应原始源码。
- 用词法扫描识别指令：只有行首（前面只有空白）的 `#` 开始一条指令，注释、字符串、字符字面量和 C++ 原始字符串（`R"分隔符(...)分隔符"`）中的 `#` 不算；反斜杠续行属于同一条指令。
- 条件求值结果分为真、假、未知三种，只使用以下确定信息：
  - 默认目标配置：Linux x86_64、GCC 13、C17/C++17。用 C++ grammar 解析时 `__cplusplus=201703`、`__GNUG__=13`、`__STDC_VERSION__` 未定义；用 C grammar 解析时 `__cplusplus`、`__GNUG__` 未定义、`__STDC_VERSION__=201710`。`.h` 文件的两次解析分别求值。另外定义 `__GNUC__`、`__linux__`、`__x86_64__`、`__LP64__`、字节序、类型宽度等编译器预定义宏和 `INT_MAX` 等标准头文件整数上限宏，并把 `_WIN32`、`_MSC_VER`、`__APPLE__`、`__clang__`、`__aarch64__` 等其他平台和编译器宏视为未定义。完整列表见 `preprocess.go` 的 `targetDefined` 和 `targetUndefined`。
  - 整数字面量，例如 `#if 0`、`#if 1`。
  - 本文件内、在已保留代码中先出现的 `#define` 和 `#undef`：`#define X` 后 `defined(X)` 为真，值为整数字面量时可参与比较；`#undef X` 后 `defined(X)` 为假。
  - 运算符 `defined`、`!`、`&&`、`||`、`==`、`!=`、`<`、`>`、`<=`、`>=` 和括号；其他运算符、未知标识符和无法解析的表达式求值为未知。`&&`、`||` 按短路语义处理未知，例如“假 && 未知”为假。
- 每组条件编译只保留第一个结果为真或未知的分支；所有分支都为假时保留 `#else` 分支，没有 `#else` 时整组都不保留。被丢弃分支中的指令和宏定义不影响后续求值。
- 缺少 `#endif` 的条件块一直延续到文件末尾，并计入报告；没有对应 `#if` 的 `#elif`、`#else`、`#endif` 保留原样，不记入报告。tree-sitter-c 把多余的 `#endif` 解析为普通预处理指令节点 `preproc_call`，不产生诊断，这类缺陷 Parser 不会报告。
- `#define`、`#undef`、`#include`、`#pragma`、`#error` 等非条件指令保留在解析视图中。
- `RawSource` 选项为真时不做分支选择，直接解析原始源码，用于对照和排查。

### 代价

只分析一种条件组合：未保留的分支（例如其他平台的实现）不进入语法树，下游无法分析其中的代码。被丢弃的分支全部记录在报告中，下游可以据此判断哪些代码未参与分析。

### 数据结构

```go
// ConditionValue 表示条件编译条件的求值结果。
type ConditionValue uint8

const (
    // ConditionUnknown 表示条件依赖外部配置或无法解析，无法确定真假。
    ConditionUnknown ConditionValue = iota + 1
    // ConditionTrue 表示条件确定为真。
    ConditionTrue
    // ConditionFalse 表示条件确定为假。
    ConditionFalse
)

// ConditionalBranch 记录条件编译中一个分支的判断与选择结果。
type ConditionalBranch struct {
    // Directive 是分支指令所在的源码范围，包含续行。
    Directive SourceRange
    // Keyword 是指令名称：if、ifdef、ifndef、elif 或 else。
    Keyword string
    // Condition 是合并续行后的条件文本，else 分支为空。
    Condition string
    // Value 是条件求值结果，else 分支为真。
    Value ConditionValue
    // Taken 表示该分支是否保留在解析视图中。
    Taken bool
}

// PreprocessReport 记录一次 C/C++ 解析前的条件编译分支选择结果。
type PreprocessReport struct {
    // Branches 是按源码顺序排列的全部条件分支。
    Branches []ConditionalBranch
    // BlankedBytes 是替换为空格、不参与解析的字节数。
    BlankedBytes int
    // Unterminated 是缺少 #endif 的条件块数量。
    Unterminated int
}
```

`ParseOptions.RawSource` 控制是否跳过分支选择。`ParseResult.Preprocess` 保存最终选中语言对应的报告，非 C/C++ 文件或 `RawSource` 为真时为空。

### 测试

- 单元 fixture 覆盖：默认目标配置（`_WIN32`、`__APPLE__`、`__linux__`、`__GNUC__`、`__clang__`）、`extern "C"` 被拆到两个分支、`#if 0`、`#elif` 链、嵌套条件、按 grammar 区分的 `__cplusplus`、本文件 `#define`/`#undef` 影响求值、续行指令、注释和字符串及原始字符串中的 `#`、缺少 `#endif`、多余的 `#endif`、函数体内被丢弃的分支（验证节点文本取自原始源码）。
- 每个 C/C++ fixture 除语法树 golden 外，还有一份报告 golden（`.pp`），逐分支记录指令、条件、求值结果和是否保留。
- 回归 fixture：redis 中以反斜杠续行的多行条件、protobuf 中原始字符串内以 `#` 开头的行。
- 单元测试校验解析视图与原文件等长，且被替换的位置在原文件中不是换行符。
- E2E expected 增加报告摘要：分支数、保留分支数、未知条件分支数、替换字节数、缺少 `#endif` 的块数和报告 SHA-256。
- E2E 对每个文件再按原样解析一次作对照，expected 锁定按原样解析时的含错文件数、因分支选择变为无诊断的文件数、原本无诊断却新增诊断的文件数（必须为 0）和诊断条数增多的文件数。

## 轻量容错宏预处理

### 结论

条件编译分支选择之后、交给 Tree-sitter 之前，Parser 对 C/C++ 再做一步宏中和：把会破坏语法结构的宏调用，在原位置改写为其展开结果中的说明符关键字，或整体替换为空格。宏定义不按编译构建的头文件顺序查找，而是假设“项目里至少存在一个定义”，在项目全部头文件中按目录距离就近选取。整个过程不读取 include path、编译数据库或构建脚本，与编译构建解耦；改写保持文件长度和全部字节偏移不变，诊断和节点范围仍直接对应原始源码。

动机见“E2E 语法错误现状”：分支选择后 protobuf 仍有 1.6% 的语法树节点处于错误子树中，消融实验表明其中约三分之二来自 `PROTOBUF_EXPORT`、`PROTOBUF_NULLABLE`、`UPB_INLINE` 这类注解宏。它们留作标识符时被当成类型名或声明符，破坏整条声明，并在类体、函数体中连锁出错。

### 处理对象

只处理展开后只剩属性和说明符关键字的宏：

- 对象式宏，例如 `#define PROTOBUF_EXPORT __attribute__((visibility("default")))`、`#define PROTOBUF_FINAL final`、`#define UPB_INLINE static __inline__`。
- 函数式宏，展开后只剩属性和说明符关键字时整个调用连同参数一起改写，例如 `#define UPB_PRINTF(str, first) __attribute__((format(printf, str, first)))`。

常量、表达式、语句和声明生成宏（如 `HANDLE_TYPE(...)`、`TEST(...)`）不改写：它们要么本来就能按普通标识符或调用表达式解析，要么展开后长度变化，无法在保持偏移的前提下写回，留给阶段二。

### 宏定义索引

- 调用方传入项目内全部受跟踪的宏定义文件，扩展名为 `.h`、`.hh`、`.hpp`、`.hxx`、`.inc`、`.inl`、`.ipp`、`.tpp`、`.def`，包括第三方目录，因为项目代码同样使用第三方库的宏。`.c`、`.cc` 等实现文件中的定义只对本文件可见，不进入索引。
- 每个头文件分别按 C 和 C++ 规则做一遍条件编译扫描，为每条 `#define` 记录两种状态：
  - 保留：位于分支选择保留的区域。
  - 可达：未被确定为假的条件排除，即所在分支的条件不为假，且同组前面没有确定为真的分支。
- 同一文件中，定义之后又在可达区域被 `#undef` 的宏不收录，它们是头文件内部的临时宏。
- 索引建好后只读，可被并发解析共享。

### 查找规则

对文件 F 中的标识符 N，按以下顺序判断：

1. N 是 C/C++ 关键字或编译器内建关键字（如 `inline`、`__attribute__`）时不处理。
2. N 前面是 `.`、`->`、`::`，或后面是 `::` 时不处理，它们是成员或限定名。
3. 本文件优先：F 中此前已保留的 `#define N` 生效；N 已被 `#undef` 时不处理，也不再查索引。
4. 否则查索引：在 F 以外、对当前 grammar 存在可达定义的头文件中，选目录距离最小的一个；距离相同时选路径字典序最小的。目录距离等于从 F 所在目录到定义文件所在目录需要上溯和下行的目录层数之和。
5. 在选中文件内，先按源码顺序尝试保留的定义；只有对象式宏才在保留的定义都不能改写时继续尝试其余可达定义。第一个能改写的定义生效；都不能改写时不处理，也不回退到更远的文件。

第 5 条依据：条件未知时，各个分支都可能是真实构建配置。对象式宏留作标识符会破坏所在声明，此时能让代码通过解析的定义更可能是真实配置。例如 protobuf 的 `PROTOBUF_TSAN_DECLARE_MEMBER` 在 `#if defined(ABSL_HAVE_THREAD_SANITIZER)` 下是成员声明，在 `#else` 下为空；前者无法改写，因此取后者。函数式宏调用本身就能按调用表达式解析，回退只会删除代码，并与分支选择采用的配置不一致。例如 redis 的 `debugServerAssert(...)` 在 `#ifdef DEBUG_ASSERTIONS` 下是 `serverAssert(...)`，在 `#else` 下为空；保留的定义不能改写，因此不处理，调用保留在语法树中。

### 改写规则

1. 递归展开定义体，逐个处理记号：
   - 丢弃属性：`__attribute__((...))`、`__attribute((...))`、`__declspec(...)`、`[[...]]`。
   - 保留说明符关键字。C 为 `static`、`extern`、`const`、`volatile`、`register`、`restrict`、`__restrict`、`__restrict__`、`_Noreturn`、`_Thread_local`、`__thread`；C++ 在此基础上增加 `constexpr`、`consteval`、`constinit`、`virtual`、`explicit`、`final`、`override`、`noexcept`、`thread_local`、`mutable`。
   - 可省略关键字：`inline`、`__inline`、`__inline__`、`__forceinline`。
   - 标识符若是对象式宏，按同一查找规则递归展开；若是后跟括号的函数式宏，展开其定义体。递归深度上限为 16，宏在自身展开过程中不再展开。
   - 其他任何记号，包括出现在属性之外的宏参数，都使该定义不可改写。
2. 写回：保留下来的关键字用一个空格连接，长度不超过调用所占字节数时写在调用起始处，其余字节替换为空格；放不下时去掉可省略关键字重试；仍放不下则不改写，计为“放不下”。函数式宏调用从宏名到匹配的右括号，可以跨行，换行符保留，关键字只写在第一行内。
3. 只改写已保留的代码，不改写预处理指令内部。
4. 节点文本：节点范围完全落在一次宏改写范围内时（即由写入的关键字生成的节点），`Text` 返回写入的关键字，例如 `UPB_INLINE` 处生成的 `storage_class_specifier` 返回 `static`；其余节点仍返回原始源码，例如包含该调用的函数定义返回的文本里仍是 `UPB_INLINE`。节点范围始终对应原始源码。

### 代价

- 宏里的属性信息被丢弃，例如 `[[nodiscard]]`、`noreturn`；可省略关键字放不下时 `inline` 也会丢失。
- 就近查找不等价于真实的 include 关系：同名宏在不同模块有不同定义时可能选错。E2E 用“宏中和后原本无诊断的文件新增诊断数必须为 0”作为守护，并锁定全部改写记录的摘要。
- 距离相同时按路径字典序选取，报告中的定义来源可能不是真实来源。例如 protobuf 的 `PROTOBUF_EXPORT` 取自 `descriptor.h` 中 `#ifdef SWIG` 下的空定义，而不是 `port_def.inc`；两者改写结果相同。
- 改写不只发生在影响解析的位置：保留定义为空的函数式宏调用会从语法树中消失，例如 redis 在 `#ifndef REDIS_TEST_VERBOSE` 下定义为空的 `D(...)`。这与分支选择采用的配置一致。
- 定义不在项目内的外部宏（abseil、GoogleTest 等不在语料中）找不到定义，不处理。

### 数据结构

```go
// MacroIndex 是项目级宏定义索引，按宏名保存各头文件中的定义，构建后只读。
type MacroIndex struct {
    // definitions 按宏名保存全部收录的定义，同名定义按文件路径和源码顺序排列。
    definitions map[string][]indexedMacro
}

// MacroRewrite 记录一次宏调用改写。
type MacroRewrite struct {
    // Range 是被改写的宏调用范围，函数式宏包含参数和右括号。
    Range SourceRange
    // Name 是宏名。
    Name string
    // Replacement 是写在调用起始处的关键字文本，为空表示整个调用替换为空格。
    Replacement string
    // DefinitionPath 是生效定义所在文件的项目内相对路径。
    DefinitionPath string
    // DefinitionLine 是生效定义所在的行号，从 1 开始。
    DefinitionLine int
}

// MacroReport 记录一次宏中和的结果。
type MacroReport struct {
    // Rewrites 是按源码顺序排列的全部改写。
    Rewrites []MacroRewrite
    // Unsupported 是找到定义、但展开含不可改写记号而未改写的宏调用数。
    Unsupported int
    // TooLong 是展开可以改写、但关键字放不下而未改写的宏调用数。
    TooLong int
}

// ParseMetrics 记录一次解析中各处理环节的耗时，.h 文件为两次解析之和。
type ParseMetrics struct {
    // Conditional 是条件编译分支选择耗时。
    Conditional time.Duration
    // Macro 是宏中和耗时。
    Macro time.Duration
    // TreeSitter 是 Tree-sitter 解析耗时。
    TreeSitter time.Duration
    // Placeholder 是宏占位符查找与重新解析的耗时，不含第一次 Tree-sitter 解析。
    Placeholder time.Duration
}
```

### 测试

- 单元 fixture 是一个独立的小项目 `fixture/macro/`，覆盖：就近选择与远处同名定义、实现文件中的定义不进入索引、头文件内 `#undef` 的临时宏不收录、`_MSC_VER` 下不可达的定义、条件未知时取第一个能改写的定义、嵌套宏、函数式属性宏（含跨行调用）、C 与 C++ 不同展开、可省略关键字、放不下、常量宏、循环定义、本文件定义优先与 `#undef`、关键字同名宏、成员访问和限定名。
- 回归：函数式宏的保留定义不能改写时不得回退到其他可达定义（redis `debugServerAssert`）。
- 节点文本：宏改写生成的关键字节点返回关键字，包含宏调用的外层节点返回原始源码。
- 每个使用文件除语法树 golden 外，另有一份带索引解析的宏报告 golden，逐条记录改写位置、宏名、写入文本、定义文件和行号，以及未改写计数。
- 单元测试校验宏中和视图与原文件等长、换行位置不变，改动字节全部落在报告的改写范围内；校验索引构建与输入文件顺序无关，并发共享索引在 Race Detector 下通过。
- E2E 用项目全部受跟踪的宏定义文件建索引，对每个 C/C++ 文件分别按原样、只做分支选择、完整预处理解析三次。expected 锁定三种方式的含错文件数、错误行数和错误节点数，改写总数、未改写计数和改写记录 SHA-256，宏中和使之变为无诊断的文件数，以及原本无诊断却新增诊断的文件数（必须为 0）。

### 可观测性与性能

- `ParseResult.Macros` 逐条记录改写位置、宏名、写入文本、定义文件和行号，以及未改写计数，可单独验证宏中和是否正确。
- `ParseOptions.CollectMetrics` 打开后，`ParseResult.Metrics` 分别给出分支选择、宏中和、Tree-sitter 解析的耗时；默认关闭，不影响结果和性能。
- 实测（全量 E2E 语料，单线程）：protobuf 建索引读取 627 个宏定义文件，耗时 0.23 s；1095 个文件的分支选择共 0.30 s，宏中和 0.42 s，Tree-sitter 解析 10.67 s，宏中和约占 4%。redis 建索引 314 个文件 0.14 s；宏中和 0.13 s，Tree-sitter 解析 2.63 s。
- 宏占位符只发生在有错误的文件上（protobuf 185 个、redis 32 个），每轮多一次完整解析。实测 protobuf 首次 Tree-sitter 解析 7.38 s、占位符 6.10 s，redis 1.27 s、0.30 s。统计错误节点时跳过不含错误的子树、错误子树节点数取自 `DescendantCount`，protobuf 占位符耗时 8.35 s→6.10 s，结果不变。

### 错误驱动的宏占位符

#### 结论

宏中和之后仍有语法错误的文件，再对与错误直接相关的宏调用放置占位符：在宏调用原位置写入一段不超过原调用长度的合法语法，使这一处能够解析。每个占位符都记录在 `SyntaxTree.Placeholders()` 和 `ParseResult.Placeholders` 中，后续分析按范围查表即可知道此处宏未展开、语义缺失，再决定如何降级。只处理有错误的文件，只在错误减少时采纳，原本无诊断的文件不受影响。

#### 候选

- 函数式宏调用：名字是宏形名字（首字符为大写字母，只含大写字母、数字和下划线，至少 2 个字符），或在本文件、索引中有定义；后跟配对括号；且与某个 `ERROR`/`MISSING` 节点相关，见下文“相关”。
- 对象式宏：名字是宏形名字，且在本文件和索引中都没有定义（外部宏）；位于注解位置，即后面紧跟标识符或 `{`，前面不是 `=`、`(`、`[`、`,`、算术、逻辑或比较运算符；且与某个错误节点相关。
- 相关：错误节点的起点落在“宏名起点到调用之后第一个非空白字符（含）”之间，或终点落在“调用之前最后一个非空白字符到宏名起点”之间。后者对应 Tree-sitter 把宏之前的声明符一起报错的情况，例如 `char* data() EXT_LIFETIME_BOUND {` 的错误节点是 `data()`。
- 只看错误起点而不看整个错误范围：一个 `ERROR` 节点常覆盖整个函数体，范围内的合法宏调用（如 `EXPECT_EQ(...)`）与错误无关，不应被替换；修复一处后暴露的新错误由下一轮处理。
- 实参或范围与宏中和改写重叠的调用不是候选，保证改写范围互不重叠。
- 已被宏中和改写的位置、成员或限定名（前有 `.`、`->`、`::`）、预处理指令内部都不是候选。

#### 形态

依据当前语法树确定作用域：从候选位置所在的最小节点向上查找，先遇到 `enumerator_list` 为枚举项列表作用域，先遇到 `initializer_list`、`argument_list` 为列表作用域，先遇到 `compound_statement` 为语句作用域，先遇到 `translation_unit`、`declaration_list`（命名空间、`extern "C"`）或 `field_declaration_list`（类体）为声明作用域，`ERROR` 节点跳过。

| 情况 | 写入 | 形态 |
|---|---|---|
| 前一个记号是 `struct`、`union`、`enum`、`class`，或位于枚举项列表 | `NAME` | 名称 |
| 外部宏，声明作用域，前一个记号是 `)`、`const`、`volatile`、`override`、`final`、`noexcept` 或普通标识符，右括号后紧跟 `{`、`;`、`=`、`,` | 全部替换为空格 | 省略 |
| 外部宏，声明作用域，前一个非空白字符是 `;`、`{`、`}` 或位于文件开头，右括号后紧跟标识符 | 全部替换为空格 | 省略 |
| 列表作用域，右括号后紧跟标识符、数字、字符串、字符或 `{` | `NAME(),` | 表达式 |
| 列表作用域，其余情况 | `NAME()` | 表达式 |
| 前一个非空白字符是 `=`、`(`、`[`、`,` 或运算符，或前一个记号是 `return`、`case` | `NAME()` | 表达式 |
| 右括号后紧跟 `;` | `NAME()` | 表达式 |
| 声明作用域，且右括号后紧跟 `{` | `void NAME()` | 定义 |
| 其余函数式宏调用 | `NAME();` | 语句 |
| 对象式外部宏 | 全部替换为空格 | 省略 |

写入文本只放在调用的第一行内，放不下或与原文相同时不放置。宏名保留在语法树中，作为被调函数名或函数名；实参不进入语法树。表格按从上到下的顺序匹配。名称形态对应生成名字的宏，例如 upb 的 `typedef struct UPB_PRIVATE(upb_X) {` 写成 `typedef struct UPB_PRIVATE {`，枚举项 `UPB_PRIVATE(kX) = 32,` 写成 `UPB_PRIVATE = 32,`。声明符之后的省略形态对应 abseil 线程安全注解，例如 `int Size() const ABSL_SHARED_LOCKS_REQUIRED(mu) {`；声明之前的省略形态对应 `ABSL_DEPRECATED("msg") void f();`；只对外部宏使用，项目内有定义的宏已知不是注解，仍写成语句形态，宏名留在语法树中。列表作用域补逗号对应展开结果提供若干元素的宏，例如 Python C API 结构体初始化中的 `PyVarObject_HEAD_INIT(&PyType_Type, 0)` 后面紧跟 `"name",`，写成 `PyVarObject_HEAD_INIT(),`。例如 GoogleTest 的 `TEST(Suite, Name) {` 在命名空间作用域写成 `void TEST()`；redis 函数体内的 `TEST("name") {` 写成 `TEST();`，后面的代码块仍是普通复合语句。

#### 采纳规则

每轮对当前语法树的全部候选同时放置并完整重新解析，保证最终语法树等于对最终视图的完整解析。不使用 Tree-sitter 增量解析：E2E 中 protobuf 的 `repeated_field_proxy.h` 增量解析得到的树与完整解析不同，会使结果依赖放置过程。错误子树节点数严格减少、且 `ERROR`/`MISSING` 节点个数不增加时采纳本轮并进入下一轮，否则撤销本轮并停止；最多 3 轮。第二个条件防止“大错误变成多处小错误”的局部破坏被节点总数掩盖：E2E 中写错形态的 `PyVarObject_HEAD_INIT();` 曾使错误节点从 103 降到 49，诊断却从 11 条增到 20 条。`.h` 文件的 C、C++ 两次解析各自进行，再按原规则择优。

#### 下游约定

- 落在占位符范围内的节点不是源码真实结构：`NAME()` 调用和 `void NAME()` 函数定义都是占位。Common AST、符号表、调用图等后续模块必须查占位符表，把这些节点标为“宏未展开”，不得当作真实调用或真实函数定义；无法处理时降级，例如调用图对占位调用不建边，或只建“未知目标”边。
- 节点文本：完全落在占位符范围内的节点返回写入的文本，例如 `void`、宏名。
- 占位符范围始终对应原始源码，分析阶段可以据此取回宏调用原文和实参。

#### 代价

- 宏实参不进入语法树，其中的调用和变量引用在语法树中看不到，只能通过占位符范围取回原文。
- “错误与宏相关”按位置判断，可能替换掉与错误无关的宏调用；采纳规则只保证文件的错误节点总数减少。
- 每轮多一次完整解析，只发生在有错误的文件上。

#### 数据结构

```go
// PlaceholderForm 表示宏占位符的语法形态。
type PlaceholderForm uint8

const (
    // PlaceholderExpression 表示写成无参调用表达式 NAME()。
    PlaceholderExpression PlaceholderForm = iota + 1
    // PlaceholderStatement 表示写成无参调用语句 NAME();。
    PlaceholderStatement
    // PlaceholderDefinition 表示写成函数定义头 void NAME()。
    PlaceholderDefinition
    // PlaceholderOmitted 表示整个宏调用替换为空格。
    PlaceholderOmitted
    // PlaceholderName 表示写成名字 NAME，用于 struct、union、enum、class 之后生成类型名的宏。
    PlaceholderName
)

// Placeholder 记录一处宏占位符，即一处宏未展开、语义缺失的位置。
type Placeholder struct {
    // Range 是被替换的宏调用在原始源码中的范围，函数式宏包含实参和右括号。
    Range SourceRange
    // Name 是宏名。
    Name string
    // Form 是占位符形态。
    Form PlaceholderForm
    // Text 是写入的文本，省略形态为空。
    Text string
    // Defined 表示是否在本文件或索引中找到宏定义；为假表示外部宏。
    Defined bool
}
```

`ParseOptions.SkipPlaceholders` 为真时不放置占位符，用于对照；`RawSource`、`ConditionalOnly` 为真时同样不放置。

#### 测试

- 单元 fixture `fixture/macro/placeholder/` 覆盖：初始化列表中提供元素的宏（回归）、`struct` 之后和枚举项位置生成名字的宏（回归，upb 的 `UPB_PRIVATE`）、声明符之后和声明之前的函数式注解宏（回归，abseil 的 `ABSL_SHARED_LOCKS_REQUIRED`、`ABSL_DEPRECATE_AND_INLINE`）、写入文本与原文相同时不放置（回归）、语句作用域中缺分号的语句宏、声明作用域的定义型宏、函数体内后跟代码块的宏、实参不合法的表达式宏、外部注解宏、错误不在宏调用上时不放置、错误与宏无关时撤销、无错误文件不放置。占位符逐条记录在宏报告 golden 中，手写。
- 单元测试校验占位符节点文本、`SyntaxTree.Placeholders()` 与 `ParseResult.Placeholders` 一致、视图等长且只改动占位符和宏中和范围。
- E2E 增加“宏中和但不放占位符”的解析方式作对照，expected 锁定占位符数和占位符记录 SHA-256，原本无诊断的文件新增诊断必须为 0。

### 阶段二（未实现）

展开函数式宏和长度变化的对象式宏需要源码映射：解析视图不再与原文件等长，语法树节点和诊断的范围要通过映射表换算回原文件，节点完全落在一次展开内时还要区分展开文本和调用处原文。这会改变 `Range` 和 `Text` 的语义，并影响 Common AST 按字节范围建立的索引，需要单独设计后实施。

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
| E2E 测试 | `01-parser/test/e2e/` | 根目录 `e2e-corpus/` 的共享只读项目 | 否，使用 `e2e` build tag |

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
e2e-corpus/
├── projects.json
└── projects/
    ├── redis/
    ├── protobuf/
    ├── junit-framework/
    ├── go-git/
    └── django/

01-parser/test/e2e/
├── parser_e2e_test.go
├── resource_e2e_test.go
├── support_test.go
└── fixture/
    └── expected/
        ├── c-redis.json
        ├── cpp-protobuf.json
        ├── java-junit.json
        ├── go-go-git.json
        ├── python-django.json
        └── resource-baseline.json
```

五个大型项目由项目根目录 `e2e-corpus/` 全局共享，只保留一份 Git submodule。`internal/e2etest/` 统一负责共享清单读取、commit 校验和文件过滤。Parser 的 `fixture/expected/` 只保存 Parser 自己的预期，不得复制源码、重复实现语料访问逻辑或存放其他模块结果。

### E2E 开源项目

以下 Star 数于 2026 年 7 月 27 日核验，均显著高于 200。源码文件数必须在实际添加 submodule 后由离线清单脚本精确统计并写入 `e2e-corpus/projects.json`；任何项目目标语言文件数少于 100 时，测试必须立即失败并更换项目。

| 语言 | 共享 Submodule 路径 | GitHub 项目 | 核验 Star 数 | 计入源码扩展名 |
|---|---|---|---:|---|
| C | `e2e-corpus/projects/redis` | `redis/redis` | 约 74,900 | `.c`、`.h` |
| C++ | `e2e-corpus/projects/protobuf` | `protocolbuffers/protobuf` | 约 71,300 | `.cc`、`.cpp`、`.cxx`、`.h`、`.hh`、`.hpp`、`.hxx` |
| Java | `e2e-corpus/projects/junit-framework` | `junit-team/junit-framework` | 约 7,000 | `.java` |
| Go | `e2e-corpus/projects/go-git` | `go-git/go-git` | 约 7,400 | `.go` |
| Python | `e2e-corpus/projects/django` | `django/django` | 约 88,200 | `.py`、`.pyi` |

选择依据：

- Star 数远高于最低线，短期波动不会使项目跌破 200。
- 每个仓库都包含超过 100 个对应语言源码文件，且包含真实工程中的复杂语法。
- 五个项目覆盖网络服务、序列化系统、测试框架、Git 实现和 Web 框架，语法分布差异明显。
- 项目规模足以暴露 grammar 兼容性、资源占用、错误恢复和文件发现问题。

### E2E 项目锁定清单

共享清单 `e2e-corpus/projects.json` 至少记录：

```json
{
  "verified_at": "2026-07-27",
  "projects": [
    {
      "id": "c-redis",
      "language": "c",
      "path": "e2e-corpus/projects/redis",
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

表中只展示一个对象结构，实际清单必须包含五个项目。`source_file_count` 在添加 submodule 时由脚本计算并替换为真实值，禁止保留零值。Star 数只在引入或升级 submodule 时联网核验；常规测试只读取共享清单并离线校验 commit 与源码文件数，避免 CI 依赖 GitHub API。

Parser 每个 expected 文件必须明确包含：测试目的、`project_id`、源码文件数、成功语法树数、系统失败数、诊断文件数、按扩展名诊断计数、规范化语法树 SHA-256 和规范化诊断 SHA-256；以及条件编译报告摘要、宏中和报告摘要、宏定义索引文件数，按原样、只做分支选择、完整预处理三种方式的含错文件数、错误行数和错误节点数，和预处理新增、消除、增多诊断的文件数。测试逐字段比较实际结果与 expected，禁止只输出日志。

资源基线使用独立的 `resource-baseline.json` 明确记录测试目的、覆盖的五个共享项目和逐项阈值。测试连续执行三轮全量解析，每轮都必须覆盖全部共享源码文件（当前 6728 个）并全部生成可访问的语法树；三轮累计系统失败必须为零，最长轮次与最短轮次的耗时比必须小于 `3`，各轮结束时观测到的 Go 堆 `HeapSys` 峰值必须小于 `4 GiB`。资源测试关闭诊断收集并立即关闭每棵语法树，只度量 Parser 解析和资源释放路径，不重复计算结构摘要。

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
git submodule update --init --recursive e2e-corpus/projects
```

## 正确性口径

Parser 是分析流水线的**中间步骤**，不产出漏洞报告，因此没有正报、误报、误报率这类检测概念（见 AGENTS.md 第 8 条）。Parser 以**正确性**验收：

- 语法树正确性：合法源码解析出的语法树必须与 golden fixture 精确一致；错误恢复产出的部分树同样纳入 golden。
- 诊断正确性：源码的语法诊断（ERROR、MISSING 对应的诊断）必须与 golden fixture 精确一致，包括故意语法错误的源码必须触发对应诊断、合法源码不产生多余诊断。
- 系统失败必须为零：没有生成语法树、panic、超时或生命周期错误任一发生即测试失败。

正确性最低标准：

- 单元层每个 fixture 的语法树与诊断都与 golden 精确一致。
- E2E 中所有纳入文件必须生成非空根节点，系统失败数量必须为零。

### E2E 语法错误现状（未达标）

E2E 中含 `ERROR` 或 `MISSING` 节点的文件不是正确结果；expected 只锁定当前输出以发现变化，不代表这些语法树正确。C、C++ 和 Java 仍未达到零错误，Parser 的 E2E 正确性验收为 ❌。

#### 错误率口径

- 文件错误率 = 含 `ERROR`/`MISSING` 节点的文件数 / 纳入文件数。
- 行错误率 = 被 `ERROR` 节点覆盖或含 `MISSING` 节点的非空行数 / 非空行数。
- 节点错误率 = 位于 `ERROR`/`MISSING` 子树中的节点数 / 语法树节点总数，含匿名节点。

三项计数由 Parser E2E 按原样、只做分支选择、完整预处理三种方式分别统计并锁定在 expected 中。只统计 Tree-sitter 明确标出的错误；宏导致的结构合法但含义错误的树（如把 `TEST(A, B) {...}` 解析成函数定义）不计入，因此是下限。

#### 当前错误率

| 项目 | 语言 | 解析方式 | 文件 | 行 | 节点 |
|---|---|---|---:|---:|---:|
| redis | C | 原样 | 53/278（19.06%） | 850/206778（0.411%） | 3824/1854347（0.206%） |
| redis | C | 分支选择 | 43/278（15.47%） | 766/206778（0.370%） | 3399/1827984（0.186%） |
| redis | C | 宏中和 | 32/278（11.51%） | 285/206778（0.138%） | 481/1826038（0.026%） |
| redis | C | 完整预处理（加占位符） | 19/278（6.83%） | 60/206778（0.029%） | 175/1825049（0.010%） |
| protobuf | C++ | 原样 | 407/1095（37.17%） | 88279/446475（19.772%） | 845280/4323402（19.551%） |
| protobuf | C++ | 分支选择 | 372/1095（33.97%） | 18378/446475（4.116%） | 64429/3977591（1.620%） |
| protobuf | C++ | 宏中和 | 185/1095（16.89%） | 3124/446475（0.700%） | 19722/3951763（0.499%） |
| protobuf | C++ | 完整预处理（加占位符） | 138/1095（12.60%） | 868/446475（0.194%） | 4122/3943450（0.105%） |
| junit-framework | Java | 四种方式相同 | 6/1738（0.35%） | 10/189439（0.005%） | 87/1598382（0.005%） |
| go-git | Go | 四种方式相同 | 0/690 | 0/131697 | 0/1353223 |
| django | Python | 四种方式相同 | 0/2927 | 0/454158 | 0/4438252 |

“完整预处理”中的错误减少有一部分来自宏占位符：占位符处的语法树是占位结构，不是源码真实结构，语义缺失记录在占位符表中（redis 217 处、protobuf 1823 处）。按“源码真实结构解析正确”的口径，应以“宏中和”一行为准。

宏中和相对只做分支选择、占位符相对宏中和、完整预处理相对按原样解析，原本无诊断却新增诊断的文件数、诊断条数增多的文件数均为 0。

#### 宏中和的动机

分支选择后 protobuf 仍有 1.62% 的节点出错。消融实验在保持字节偏移的前提下改写源码后重新解析（实验代码不进仓库）：

| 处理方式（在分支选择基础上） | protobuf 节点错误率 | 含错文件 |
|---|---:|---:|
| 不处理 | 1.620% | 372 |
| 仓库内定义为空、属性或关键字的对象式宏全局替换为空格 | 0.701% | 205 |
| 再加 abseil 注解宏（不在语料中，手工列出） | 0.557% | 187 |
| 再把三类 grammar 缺陷改成等长合法写法 | 0.551% | 186 |

结论：剩余错误约三分之二来自注解宏，grammar 缺陷影响很小。全局替换会误伤同名普通标识符，因此实现改为按目录就近查找定义，见“轻量容错宏预处理”。

#### 宏中和后的剩余错误与占位符

宏中和后的剩余错误按错误位置附近的宏名估计（位置特征，未逐条证实）：

- 函数式宏：约占 protobuf 剩余错误节点的 61%、redis 的 53%，例如展开为整段 `case` 的 `HANDLE_TYPE(...)`、GoogleTest 的 `TEST`/`EXPECT_*`、`PROTOBUF_DEFINE_*_ACCESSOR(...)`、redis 配置表宏 `MEMORY_CONFIG(...)`。宏占位符处理其中与错误直接相关的调用；真正展开需要阶段二的源码映射。
- 定义不在语料中的外部宏：abseil 的 `ABSL_ATTRIBUTE_LIFETIME_BOUND`、`ABSL_CHECK` 等。注解位置的这类宏由占位符省略。
- grammar 缺陷，见下表。

占位符形态分布（E2E 抽查）：protobuf 1823 处，其中省略 917（几乎全是 abseil 注解）、表达式 476、语句 391、定义 27（全是 GoogleTest 的 `MATCHER*`）、名称 12（upb 的名字生成宏）；redis 217 处，其中语句 216（主要是测试宏 `TEST`、`TEST_DESC` 和 `run_with_period`）、表达式 1。

#### 已证实的 grammar 缺陷

以下合法语法在最小样例中单独解析也报错，与宏无关：

| 语言 | 语法 | 位置 |
|---|---|---|
| Java | Java 25 模块导入 `import module org.junit.start;` | junit 4 个文件 |
| Java | 可变参数类型注解 `Class<?> @Nullable... classes` | `ClassUtils.java`、`ReflectionUtils.java`，均为主代码 |
| C++ | 带默认参数的右值引用限定成员函数 `SerializeText(TextSerializationOptions options = {}) &&;` | `conformance/testee.h:107` |
| C++ | 依赖类型的函数式转换 `return typename T::Proxy();` | `hpb/backend/cpp/cpp.h:24` 等 |
| C++ | 匿名且带底层类型的枚举 `enum : int { kX = 1 };` | protobuf 生成代码 `*.pb.h` 等共 70 处 |
| C++ | 模板默认类型实参含 const、引用或指针 `template <typename T = const int&>` | `*.pb.h` 字段访问器 |
| C++ | new 指针数组 `new int*[n]()` | `generated_message_reflection.cc:213` |

当前接入的 tree-sitter-cpp v0.23.4 和 tree-sitter-java v0.23.5 都已是对应 Go 模块的最新版本标签。

#### 已纠正的输入缺陷

protobuf 的 `objectivec/` 目录是 Objective-C 源码，之前未被排除，40 个文件被当作 C/C++ 解析，其中 36 个出错，共 3011 条诊断。现已加入 `excluded_paths`。

排除路径及原因（只影响测试范围；宏定义索引仍读取这些目录中的头文件）：

| 项目 | 排除路径 | 原因 |
|---|---|---|
| redis | `deps/` | 第三方依赖源码 |
| protobuf | `third_party/` | 第三方依赖源码 |
| protobuf | `objectivec/` | Objective-C 源码，不属于 C/C++ |
| go-git | `vendor/` | 第三方依赖源码 |

#### 消除计划

1. 保持字节偏移的轻量预处理，每一步都必须用 E2E 证明错误减少，且原本无诊断的文件不新增诊断：
   - ✅ 条件编译分支选择。
   - ✅ 默认目标配置（Linux x86_64、GCC 13）。
   - ✅ 注解宏中和，宏定义按目录就近查找。
   - ✅ 错误驱动的宏占位符：使含错的宏调用可解析，并记录语义缺失位置。
   - ❌ 函数式宏和长度变化的宏展开（阶段二，需要源码映射）。
   - ❌ 语料中没有定义的外部宏（abseil、GoogleTest）：评估内置常见外部注解宏表。
2. 对剩余诊断逐条确认根因。grammar 缺陷列为已知豁免并单独统计，同时评估向上游提交修复或固定到包含修复的上游提交。

## 验收标准

- C、C++、Java、Go 和 Python grammar 均通过固定版本接入并完成 ABI 校验。
- 单元测试与 E2E 测试分别位于独立目录。
- 所有测试源码均为独立 fixture 文件，不存在内嵌被测源码字符串。
- 单元测试覆盖合法解析、错误恢复、语言识别、生命周期、并发和确定性。
- `.h` 文件全部执行 C 与 C++ 双 grammar 解析，并按固定错误评分规则选择结果。
- `.hh`、`.hpp`、`.hxx`、`.ipp`、`.tpp` 文件全部使用 C++ grammar 解析。
- C/C++ 头文件解析不得读取编译数据库、构建脚本、include path、项目配置或调用方语言提示。
- 根目录共享语料包含五个固定提交的 Git submodule，Parser 模块不复制源码。
- 每个 E2E 项目在加入时 Star 数不低于 200，且目标语言源码文件不少于 100。
- E2E 测试全量解析纳入范围的源码文件，不使用抽样替代。
- 单元层每个 fixture 的语法树与诊断均与 golden 精确一致。
- E2E 全量纳入文件系统失败为零、全部生成语法树，指标与 golden 基线一致。
- `go test`、`go test -race`、E2E 测试、静态检查和项目构建全部通过。
- 测试报告包含文件数、成功树数、含 ERROR 或 MISSING 节点的文件数、系统失败数。

## 实施进度

- ✅ 明确 Parser 模块职责与边界。
- ✅ 确定使用官方 `go-tree-sitter` 绑定与五种官方 grammar。
- ✅ 完成核心数据结构、接口、生命周期和错误处理设计。
- ✅ 单独设计 `01-parser/test/unit/` 单元测试目录。
- ✅ 单独设计 `01-parser/test/e2e/` E2E 测试目录。
- ✅ 选定五个 Star 数高于 200 的 GitHub 开源项目。
- ✅ 设计共享 E2E submodule、语料清单、模块 expected 与源码文件数校验规则。
- ✅ 定义 Parser 阶段的正报、误报、漏报和系统失败口径。
- ✅ 创建 Go module 与 Parser 包结构。
- ✅ 添加并固定 Tree-sitter runtime 与 grammar 依赖版本（go-tree-sitter v0.25.0；grammar：c v0.24.2、cpp v0.23.4、java v0.23.5、go v0.25.0、python v0.25.0）。
- ✅ 创建五种语言单元测试 fixture（positive/negative/boundary）。
- ✅ 实现 Parser、语言识别、语法树包装、诊断收集与 .h 双解析择优。
- ✅ 运行单元测试与 Race Detector 通过（`go test`、`go test -race`、`go vet`、`gofmt` 均通过）。
- ✅ 单元正确性：每个 fixture 的语法树与诊断均与 golden 精确一致，故意语法错误 fixture 全部触发对应诊断，合法 fixture 无多余诊断，系统失败=0。
- ✅ 根目录共享五个 E2E Git submodule 并记录真实 commit、Star 数和源码文件数，Parser 模块不再持有重复副本。
- ❌ E2E 零错误：系统失败为 0，全部生成语法树，输出与 expected 一致；但 junit 6 个、redis 19 个、protobuf 138 个文件仍含语法错误（宏中和后为 redis 32 个、protobuf 185 个，其余由占位符消除），见“E2E 语法错误现状”。
- ✅ 纠正语料与文档：排除 protobuf `objectivec/`，更正 grammar 版本、junit 错误来源和“已排除 Objective-C”的错误说法。
- ✅ 完成多轮资源基线 `TestE2EResourceBaseline`：普通 E2E 三轮均为全部共享源码文件成功生成语法树、系统失败=0、耗时波动=1.00x、峰值 `HeapSys=11960320` 字节；Race E2E 三轮同样全部成功、系统失败=0、耗时波动=1.00x、峰值 `HeapSys=11829248` 字节，均满足波动<3x、`HeapSys<4 GiB` 的固定退化保护。
- ✅ 明确 `FileID` 只在同一分析任务内唯一、`Path` 为项目内相对路径的规则；`SyntaxTree` 新增 `Path()`，单元测试 `TestSyntaxTreeKeepsFileIdentity` 先失败后通过；E2E 改为传入项目内相对路径。
- ✅ 条件编译分支选择：设计、15 个 C/C++ fixture 与手写报告 golden、失败测试（6 个合法 fixture 在旧实现上报诊断）、实现；单元测试全部通过。
- ✅ 条件编译分支选择 E2E：redis 含错文件 53→43、protobuf 407→374，原本无诊断的文件新增诊断数为 0；Java、Go、Python 语法树摘要不变；expected 增加报告摘要和按原样解析的对照字段。
- ✅ 合法 fixture 的诊断检查改为必须为零，删除原来“误报率低于 30%”的比例阈值。
- ✅ 回归覆盖：`ParseCtx` 空指针缺陷由 `TestParseContextCanceled` 固定回归；解析行为层暂无其他已修复缺陷，`regression/` 目录预留。
- ✅ 轻量容错宏预处理设计：宏定义索引、就近查找、改写规则、代价、数据结构和测试方案，见“轻量容错宏预处理”。
- ✅ 默认目标配置（Linux x86_64、GCC 13）纳入条件求值：新增 `fixture/c/positive/cond_target.c` 与手写报告 golden，去掉目标配置时 `_WIN32` 求值为未知、Windows 分支被保留，测试失败，加入后通过；E2E redis 未知条件分支 752→546、protobuf 3547→3262，protobuf 含错文件 374→372，原本无诊断的文件新增诊断为 0；Java、Go、Python 不变；`go test`、`go test -race` 单元测试通过。
- ✅ 宏中和 fixture、手写报告 golden 与失败测试：小项目 `fixture/macro/` 共 9 个文件，7 份带索引宏报告 golden（含回归 fixture）和全部无索引宏报告 golden 均为手写；未实现时 `TestMacroLegalFilesHaveNoDiagnostics` 等 7 个测试失败。
- ✅ 宏中和实现（`macro.go`），手写宏报告 golden 全部一致，宏小项目 5 个合法使用文件带索引解析后诊断为 0（其中 3 个只做分支选择时有诊断）；`go test`、`go test -race`、`go vet` 通过。
- ✅ 宏中和 E2E：redis 含错文件 43→32、节点错误率 0.186%→0.026%，protobuf 372→185、1.620%→0.499%；相对原样和相对分支选择，新增诊断文件和诊断增多文件均为 0；Java、Go、Python 语法树与诊断摘要不变；expected 锁定三种解析方式的文件、行、节点错误数和宏中和报告摘要。资源基线三轮通过（耗时波动 1.05x，峰值 `HeapSys=37158912` 字节）。Common AST E2E 随之更新 expected，见 `design/02-ast.md`。
- ✅ 记录文件、行、节点三种错误率及宏中和的消融动机，见“E2E 语法错误现状”。
- ✅ 回归：E2E 抽查发现函数式宏回退到未保留的空定义会删除可正常解析的调用（redis `debugServerAssert` 46 处、protobuf `PROTOBUF_ASSUME` 61 处），规则改为函数式宏只用保留的定义；回归 fixture `fixture/macro/lib/regression/debug_assert.c` 在旧逻辑下失败、新逻辑下通过。
- ✅ 节点文本：复查发现宏改写生成的关键字节点 `Text` 返回截断的宏名（如 `LIB_IN`），改为完全落在改写范围内的节点返回写入的关键字；`TestMacroRewrittenNodeText` 先失败后通过；E2E 只有 redis、protobuf 的语法树摘要变化，其余计数与摘要不变。
- ✅ 宏中和后的项目级验证：`go build`、`go vet`、`gofmt`、Parser 与 Common AST 单元测试、`go test -race` 单元测试、Parser 与 Common AST E2E、Parser 资源基线 E2E、`go test -race -tags=e2e ./01-parser/test/e2e/` 全部通过。
- ✅ 回归：E2E 比较“诊断是否增多”误用了每个文件的含错标记（只有 0 或 1），该项检查从未生效；改为比较诊断条数，expected 增加三种解析方式的诊断总条数（protobuf 25768、18699、2812）。修正后诊断增多的文件数仍为 0。
- ✅ 错误驱动的宏占位符设计：候选、形态、采纳规则、下游约定、代价、数据结构和测试方案，见“错误驱动的宏占位符”。
- ✅ 宏占位符 fixture、手写 golden 与失败测试：`fixture/macro/placeholder/` 共 12 个文件，占位符记录 golden 全部手写；未实现时 `TestPlaceholderFixesErrors`、`TestPlaceholderNodeText` 和快照测试失败。
- ✅ 宏占位符实现（`placeholder.go`），单元测试与 Race Detector 通过。E2E 抽查发现并以回归 fixture 固定 5 类形态缺陷，每个先失败后通过：初始化列表中的元素宏被写成语句（`initializer_list.c`）、`struct` 之后的名字宏被写成定义（`struct_name.c`）、枚举项名字宏被写成语句（`enum_name.c`）、声明符之后的注解宏被写成定义（`lock_annotation.cpp`）、声明之前的注解宏被写成语句或写入与原文相同的文本（`decl_annotation.cpp`）；采纳条件增加“错误节点个数不增加”。
- ✅ 宏占位符 E2E：redis 含错文件 32→19、节点错误率 0.026%→0.010%，protobuf 185→138、0.499%→0.105%；占位符 redis 217 处、protobuf 1823 处；原本无诊断的文件新增诊断和诊断增多的文件均为 0；Java、Go、Python 摘要不变；资源基线三轮通过（耗时波动 1.06x，峰值 `HeapSys=37158912` 字节）。Common AST E2E 随之更新 expected。

### 实现与验证说明

- 因单元测试为黑盒包，测试与构建使用精确包路径：实现构建 `go build ./01-parser/`、单元测试 `go test ./01-parser/test/unit`。不使用 `./...` 通配，避免 Go 语言 fixture（含故意错误与空文件的 `.go` 源）被 Go 工具链当作待构建包。
- go-tree-sitter v0.25.0 的 `ParseCtx` 在未设置取消标志时对空指针解引用，实现中在解析前预检查 `ctx.Err()` 并设置非空取消标志规避该问题，同时保留上下文取消语义。
- 资源基线只把各轮垃圾回收后的 `runtime.MemStats.HeapSys` 作为 Go 堆退化指标；Tree-sitter 语法树通过每文件显式 `Close` 和 Race E2E 验证生命周期，不把 `HeapSys` 错误解释为底层 C 分配的完整统计。
