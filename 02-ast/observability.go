package commonast

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// ConversionOptions 控制单次 Common AST 转换的资源限制与可观测性输出。
type ConversionOptions struct {
	// MaxDepth 是允许转换的最大 CST 嵌套深度，零值使用转换器默认限制。
	MaxDepth int
	// CollectMetrics 指定是否采集耗时、内存和处理量指标。
	CollectMetrics bool
	// CollectSnapshot 指定是否返回稳定 JSON 格式的中间 AST 快照。
	CollectSnapshot bool
	// EnableTrace 指定是否创建 Go 运行时跟踪区域。
	EnableTrace bool
}

// ConversionMetrics 保存单次 Common AST 转换的性能和处理量指标。
type ConversionMetrics struct {
	// Elapsed 是转换及可选快照生成的总耗时。
	Elapsed time.Duration
	// AllocatedBytes 是转换期间 Go 运行时累计分配的字节数。
	AllocatedBytes uint64
	// InputBytes 是输入 CST 根节点覆盖的源码字节数。
	InputBytes uint
	// NodeCount 是生成的 Common AST 节点数。
	NodeCount int
	// DiagnosticCount 是转换产生的诊断数。
	DiagnosticCount int
}

// ConversionResult 保存带选项转换产生的 AST、诊断和可选观测数据。
type ConversionResult struct {
	// AST 是转换完成后的 Common AST，系统错误发生时为空。
	AST CommonAST
	// Diagnostics 是按源码顺序产生的转换诊断。
	Diagnostics []Diagnostic
	// Metrics 是按需采集的转换指标，未启用时为空。
	Metrics *ConversionMetrics
	// Snapshot 是按需生成的稳定 JSON 快照，未启用时为空。
	Snapshot []byte
}

// snapshotDocument 是稳定 JSON 快照的顶层结构。
type snapshotDocument struct {
	// Language 是源码语言的稳定短名称。
	Language string `json:"language"`
	// FileID 是源码文件的稳定标识。
	FileID uint64 `json:"file_id"`
	// Root 是 Common AST 根节点。
	Root snapshotNode `json:"root"`
}

// snapshotNode 是稳定 JSON 快照中的单个 AST 节点。
type snapshotNode struct {
	// ID 是节点的稳定标识。
	ID uint64 `json:"id"`
	// Kind 是节点的核心语法种类。
	Kind string `json:"kind"`
	// Role 是节点在父节点中的语义角色，根节点不输出。
	Role string `json:"role,omitempty"`
	// Range 是节点覆盖的源码范围。
	Range snapshotRange `json:"range"`
	// Text 是名称、字面量和未归类节点保留的源码文本。
	Text string `json:"text,omitempty"`
	// Attributes 是按名称排序的节点语法属性。
	Attributes []snapshotAttribute `json:"attributes,omitempty"`
	// Extension 是节点关联的语言扩展。
	Extension *snapshotExtension `json:"extension,omitempty"`
	// Children 是按源码顺序输出的直接子节点。
	Children []snapshotNode `json:"children,omitempty"`
}

// snapshotRange 是稳定 JSON 快照中的源码范围。
type snapshotRange struct {
	// StartByte 是范围起始字节偏移。
	StartByte uint `json:"start_byte"`
	// EndByte 是范围结束字节偏移。
	EndByte uint `json:"end_byte"`
	// StartRow 是范围起始零基行号。
	StartRow uint `json:"start_row"`
	// StartColumn 是范围起始零基列号。
	StartColumn uint `json:"start_column"`
	// EndRow 是范围结束零基行号。
	EndRow uint `json:"end_row"`
	// EndColumn 是范围结束零基列号。
	EndColumn uint `json:"end_column"`
}

// snapshotAttribute 是稳定 JSON 快照中的单个节点属性。
type snapshotAttribute struct {
	// Name 是属性名称。
	Name string `json:"name"`
	// Value 是属性值。
	Value string `json:"value"`
}

// snapshotExtension 是稳定 JSON 快照中的语言扩展。
type snapshotExtension struct {
	// Language 是扩展所属语言的稳定短名称。
	Language string `json:"language"`
	// Kind 是扩展语法的中文名称。
	Kind string `json:"kind"`
}

// MarshalSnapshot 把 Common AST 序列化为确定性的 JSON 中间结果。
func MarshalSnapshot(tree CommonAST) ([]byte, error) {
	if tree == nil || tree.Root() == nil {
		return nil, ErrInvalidAST
	}
	document := snapshotDocument{
		Language: tree.Language().String(),
		FileID:   uint64(tree.FileID()),
		Root:     buildSnapshotNode(tree.Root(), RoleNone),
	}
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化 Common AST 快照失败: %w", err)
	}
	return append(content, '\n'), nil
}

// buildSnapshotNode 递归构造按源码顺序排列的稳定快照节点。
func buildSnapshotNode(node Node, role Role) snapshotNode {
	result := snapshotNode{
		ID:    uint64(node.ID()),
		Kind:  node.Kind().String(),
		Range: makeSnapshotRange(node.Range()),
		Text:  node.Text(),
	}
	if role != RoleNone {
		result.Role = role.String()
	}
	result.Attributes = snapshotAttributes(node)
	if extension, ok := node.Extension(); ok {
		result.Extension = &snapshotExtension{Language: extension.Language().String(), Kind: extension.Kind()}
	}
	for index := 0; index < node.ChildCount(); index++ {
		child, ok := node.Child(index)
		if !ok {
			continue
		}
		result.Children = append(result.Children, buildSnapshotNode(child, node.ChildRole(index)))
	}
	return result
}

// makeSnapshotRange 把公共源码范围转换为稳定 JSON 字段。
func makeSnapshotRange(sourceRange SourceRange) snapshotRange {
	return snapshotRange{
		StartByte:   sourceRange.StartByte,
		EndByte:     sourceRange.EndByte,
		StartRow:    sourceRange.StartPoint.Row,
		StartColumn: sourceRange.StartPoint.Column,
		EndRow:      sourceRange.EndPoint.Row,
		EndColumn:   sourceRange.EndPoint.Column,
	}
}

// snapshotAttributes 返回按名称排序的节点属性副本。
func snapshotAttributes(node Node) []snapshotAttribute {
	internal, ok := node.(*astNode)
	if !ok || len(internal.attributes) == 0 {
		return nil
	}
	names := make([]string, 0, len(internal.attributes))
	for name := range internal.attributes {
		names = append(names, name)
	}
	sort.Strings(names)
	attributes := make([]snapshotAttribute, 0, len(names))
	for _, name := range names {
		attributes = append(attributes, snapshotAttribute{Name: name, Value: internal.attributes[name]})
	}
	return attributes
}
