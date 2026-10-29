package commonast

// DiagnosticKind 表示 Common AST 转换诊断的类型。
type DiagnosticKind uint8

const (
	// DiagnosticUnmappedSyntax 表示语法节点尚未映射到明确的核心节点。
	DiagnosticUnmappedSyntax DiagnosticKind = iota + 1
	// DiagnosticUpstreamSyntax 表示上游 CST 中存在错误或缺失节点。
	DiagnosticUpstreamSyntax
	// DiagnosticDepthLimit 表示语法树深度超过转换限制。
	DiagnosticDepthLimit
	// DiagnosticInternalInvariant 表示 Common AST 内部不变量被破坏。
	DiagnosticInternalInvariant
)

// Diagnostic 表示一个可以定位到源码范围的转换问题。
type Diagnostic struct {
	// Kind 表示诊断类型。
	Kind DiagnosticKind
	// Message 是供开发者阅读的简洁问题说明。
	Message string
	// Range 是问题对应的源码范围。
	Range SourceRange
	// CSTKind 是产生诊断的 Tree-sitter 节点类型。
	CSTKind string
}
