package parser

import "errors"

var (
	// ErrUnsupportedExtension 表示文件扩展名不属于任何受支持语言，且不是需要双解析的头文件。
	ErrUnsupportedExtension = errors.New("parser: 不支持的文件扩展名")
	// ErrHeaderParseFailed 表示 .h 头文件的 C 与 C++ 两次解析都发生系统性失败。
	ErrHeaderParseFailed = errors.New("parser: 头文件 C 与 C++ 双解析均失败")
	// ErrIncompatibleGrammar 表示 grammar 的 ABI 版本与运行时不兼容。
	ErrIncompatibleGrammar = errors.New("parser: grammar ABI 版本不兼容")
	// ErrTreeClosed 表示在语法树已关闭后访问其节点。
	ErrTreeClosed = errors.New("parser: 语法树已关闭")
	// ErrEmptyTree 表示底层解析未能产生任何语法树。
	ErrEmptyTree = errors.New("parser: 未能生成语法树")
)
