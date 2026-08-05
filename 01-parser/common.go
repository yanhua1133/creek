// Package parser 提供基于 Tree-sitter 的多语言解析层，把 C、C++、Java、Go 和
// Python 源码解析为统一包装的语法树，并保留源码位置与语法诊断。
package parser

// FileID 是源码文件在当前分析任务中的稳定标识。
type FileID uint64

// SourcePoint 表示源码中的一个零基行列位置。
type SourcePoint struct {
	// Row 是零基行号。
	Row uint
	// Column 是零基列号，按字节计。
	Column uint
}

// SourceRange 表示源码中的一个区间，同时包含字节偏移和行列位置。
type SourceRange struct {
	// StartByte 是区间起始字节偏移，包含。
	StartByte uint
	// EndByte 是区间结束字节偏移，不包含。
	EndByte uint
	// StartPoint 是区间起始的行列位置。
	StartPoint SourcePoint
	// EndPoint 是区间结束的行列位置。
	EndPoint SourcePoint
}
