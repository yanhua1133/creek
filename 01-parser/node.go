package parser

import (
	ts "github.com/tree-sitter/go-tree-sitter"
)

// node 是 Node 接口的实现，持有底层 Tree-sitter 节点及其所属语法树的引用。
// 所有访问都在所属树的锁下进行，并检查关闭状态，避免访问已释放内存。
type node struct {
	// owner 是该节点所属的语法树，用于加锁与关闭检查。
	owner *syntaxTree
	// raw 是底层 Tree-sitter 节点，所属树关闭后不得再访问。
	raw *ts.Node
}

// toSourceRange 把 Tree-sitter 的范围转换为本模块统一的源码范围。
func toSourceRange(r ts.Range) SourceRange {
	return SourceRange{
		StartByte:  r.StartByte,
		EndByte:    r.EndByte,
		StartPoint: SourcePoint{Row: r.StartPoint.Row, Column: r.StartPoint.Column},
		EndPoint:   SourcePoint{Row: r.EndPoint.Row, Column: r.EndPoint.Column},
	}
}

// access 在所属树的锁下执行只读操作 fn，若树已关闭则返回 ErrTreeClosed。
func (n *node) access(fn func(raw *ts.Node)) error {
	n.owner.mu.Lock()
	defer n.owner.mu.Unlock()
	if n.owner.closed || n.owner.tree == nil || n.raw == nil {
		return ErrTreeClosed
	}
	fn(n.raw)
	return nil
}

// Kind 返回当前节点的语法类型名称。
func (n *node) Kind() (string, error) {
	var out string
	err := n.access(func(raw *ts.Node) { out = raw.Kind() })
	return out, err
}

// IsNamed 报告当前节点是否是 grammar 定义的命名节点。
func (n *node) IsNamed() (bool, error) {
	var out bool
	err := n.access(func(raw *ts.Node) { out = raw.IsNamed() })
	return out, err
}

// IsError 报告当前节点是否表示无法正常归类的语法错误。
func (n *node) IsError() (bool, error) {
	var out bool
	err := n.access(func(raw *ts.Node) { out = raw.IsError() })
	return out, err
}

// IsMissing 报告当前节点是否是 Tree-sitter 补出的缺失语法元素。
func (n *node) IsMissing() (bool, error) {
	var out bool
	err := n.access(func(raw *ts.Node) { out = raw.IsMissing() })
	return out, err
}

// Range 返回当前节点对应的源码范围。
func (n *node) Range() (SourceRange, error) {
	var out SourceRange
	err := n.access(func(raw *ts.Node) { out = toSourceRange(raw.Range()) })
	return out, err
}

// ChildCount 返回当前节点的直接子节点数量。
func (n *node) ChildCount() (int, error) {
	var out int
	err := n.access(func(raw *ts.Node) { out = int(raw.ChildCount()) })
	return out, err
}

// Child 返回指定位置的直接子节点，索引越界或对应子节点为空时第二个返回值为 false。
func (n *node) Child(index int) (Node, bool, error) {
	if index < 0 {
		return nil, false, nil
	}
	var (
		child *ts.Node
		ok    bool
	)
	err := n.access(func(raw *ts.Node) {
		if uint(index) >= raw.ChildCount() {
			return
		}
		child = raw.Child(uint(index))
		ok = child != nil
	})
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return &node{owner: n.owner, raw: child}, true, nil
}

// ChildByField 返回指定字段对应的直接子节点，不存在时第二个返回值为 false。
func (n *node) ChildByField(name string) (Node, bool, error) {
	var (
		child *ts.Node
		ok    bool
	)
	err := n.access(func(raw *ts.Node) {
		child = raw.ChildByFieldName(name)
		ok = child != nil
	})
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return &node{owner: n.owner, raw: child}, true, nil
}

// FieldNameForChild 返回指定位置子节点对应的字段名称，无字段名时返回空字符串。
func (n *node) FieldNameForChild(index int) (string, error) {
	if index < 0 {
		return "", nil
	}
	var out string
	err := n.access(func(raw *ts.Node) {
		if uint(index) >= raw.ChildCount() {
			return
		}
		out = raw.FieldNameForChild(uint32(index))
	})
	return out, err
}

// Text 返回当前节点覆盖的源码内容的只读副本。
func (n *node) Text() ([]byte, error) {
	var out []byte
	err := n.access(func(raw *ts.Node) {
		start, end := raw.ByteRange()
		if end > uint(len(n.owner.content)) || start > end {
			return
		}
		buf := make([]byte, end-start)
		copy(buf, n.owner.content[start:end])
		out = buf
	})
	return out, err
}
