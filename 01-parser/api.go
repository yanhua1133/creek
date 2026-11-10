package parser

import "context"

// SourceFile 表示一次解析所需的源码文件及其基本信息。
type SourceFile struct {
	// ID 是调用方分配的文件编号，只要求在同一分析任务内唯一，不保证跨运行稳定，不得写入快照或 golden。
	ID FileID
	// Path 是以 / 分隔的项目内相对路径，必须包含可识别的扩展名，用于语言识别、诊断定位和在快照中标识文件。
	Path string
	// Content 保存待解析的 UTF-8 源码内容，在语法树存活期间必须保持不可变。
	Content []byte
}

// ParseOptions 控制单次源码解析的可选行为。
type ParseOptions struct {
	// CollectDiagnostics 指定是否遍历语法树收集语法错误和缺失节点诊断。
	CollectDiagnostics bool
	// RawSource 为真时直接解析原始源码，不做任何 C/C++ 预处理，用于对照和排查。
	RawSource bool
	// ConditionalOnly 为真时只做条件编译分支选择，不做宏中和，用于对照和排查。
	ConditionalOnly bool
	// Macros 是项目级宏定义索引，为空时宏中和只使用本文件内的定义。
	Macros *MacroIndex
	// CollectMetrics 指定是否记录各处理环节的耗时，默认关闭。
	CollectMetrics bool
}

// DiagnosticKind 表示解析诊断问题的类型。
type DiagnosticKind uint8

const (
	// DiagnosticSyntaxError 表示 Tree-sitter 的 ERROR 节点对应的语法错误。
	DiagnosticSyntaxError DiagnosticKind = iota + 1
	// DiagnosticMissingToken 表示 Tree-sitter 补出的 MISSING 缺失语法元素。
	DiagnosticMissingToken
)

// Diagnostic 表示一个可以定位到源码范围的解析问题。
type Diagnostic struct {
	// Kind 表示诊断问题的类型。
	Kind DiagnosticKind
	// Message 是供开发者阅读的简洁问题说明。
	Message string
	// Range 是问题对应的源码范围。
	Range SourceRange
	// NodeKind 是产生该诊断的 Tree-sitter 节点类型名称。
	NodeKind string
}

// HeaderResolution 记录 .h 头文件用 C 与 C++ 双解析择优的过程数据，便于测试与问题定位。
type HeaderResolution struct {
	// CErrorCount 是 C grammar 解析产生的 ERROR 与 MISSING 节点总数。
	CErrorCount int
	// CPPErrorCount 是 C++ grammar 解析产生的 ERROR 与 MISSING 节点总数。
	CPPErrorCount int
	// CErrorBytes 是 C grammar 解析中错误节点覆盖的源码字节数。
	CErrorBytes uint
	// CPPErrorBytes 是 C++ grammar 解析中错误节点覆盖的源码字节数。
	CPPErrorBytes uint
	// Chosen 是最终选择的语言。
	Chosen Language
}

// ParseResult 保存语法树和解析过程中产生的诊断。
type ParseResult struct {
	// Language 是 Parser 根据扩展名和解析结果确定的实际语言。
	Language Language
	// Tree 是本次解析生成的语法树，非空不代表语法完全正确，须结合诊断判断。
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
}

// Parser 定义单个源码文件的解析能力。
type Parser interface {
	// Parse 解析指定源码文件并返回语法树和诊断。语法错误通过诊断表达，不作为 error 返回；
	// error 仅用于不支持的扩展名、grammar 不兼容、上下文取消或底层无法产生语法树等系统性失败。
	Parse(ctx context.Context, file SourceFile, options ParseOptions) (ParseResult, error)
}

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
	// Close 释放语法树持有的底层资源，重复调用安全且返回稳定结果。
	Close() error
}

// Node 提供对语法树节点的只读访问能力，所有方法在所属语法树关闭后返回 ErrTreeClosed。
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
	// Child 返回指定位置的直接子节点，越界时第二个返回值为 false。
	Child(index int) (Node, bool, error)
	// ChildByField 返回指定字段对应的直接子节点，不存在时第二个返回值为 false。
	ChildByField(name string) (Node, bool, error)
	// FieldNameForChild 返回指定位置子节点对应的字段名称，无字段名时返回空字符串。
	FieldNameForChild(index int) (string, error)
	// Text 返回当前节点覆盖的源码内容的只读副本；完全落在一次宏改写范围内的节点返回写入的关键字。
	Text() ([]byte, error)
}
