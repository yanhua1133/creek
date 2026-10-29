package commonast

import "errors"

var (
	// ErrInvalidSyntaxTree 表示上游语法树为空、已关闭或不可访问。
	ErrInvalidSyntaxTree = errors.New("上游语法树无效")
	// ErrUnsupportedLanguage 表示当前转换器不支持输入语法树的语言。
	ErrUnsupportedLanguage = errors.New("不支持的源码语言")
	// ErrInvalidAST 表示待序列化的 Common AST 为空或缺少根节点。
	ErrInvalidAST = errors.New("Common AST 无效")
)
