package parser

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// errorStats 前序遍历语法树，统计 ERROR 与 MISSING 节点的数量及其覆盖的源码字节数，
// 用于 .h 头文件双解析择优。
func errorStats(tree *ts.Tree) (count int, coveredBytes uint) {
	root := tree.RootNode()
	var walk func(n *ts.Node)
	walk = func(n *ts.Node) {
		if n == nil {
			return
		}
		if n.IsError() || n.IsMissing() {
			count++
			start, end := n.ByteRange()
			if end >= start {
				coveredBytes += end - start
			}
		}
		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return count, coveredBytes
}

// diagKey 用于按范围与类型对诊断去重。
type diagKey struct {
	startByte uint
	endByte   uint
	kind      DiagnosticKind
}

// collectDiagnostics 前序遍历语法树，把 ERROR 节点转为语法错误诊断、MISSING 节点转为缺失 token 诊断，
// 按源码位置排序并对同范围同类型诊断去重。
func collectDiagnostics(tree *ts.Tree) []Diagnostic {
	root := tree.RootNode()
	seen := map[diagKey]bool{}
	var diags []Diagnostic
	var walk func(n *ts.Node)
	walk = func(n *ts.Node) {
		if n == nil {
			return
		}
		if n.IsMissing() {
			addDiagnostic(n, DiagnosticMissingToken, "缺失语法元素", seen, &diags)
		} else if n.IsError() {
			addDiagnostic(n, DiagnosticSyntaxError, "语法错误", seen, &diags)
		}
		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	sort.SliceStable(diags, func(i, j int) bool {
		if diags[i].Range.StartByte != diags[j].Range.StartByte {
			return diags[i].Range.StartByte < diags[j].Range.StartByte
		}
		if diags[i].Range.EndByte != diags[j].Range.EndByte {
			return diags[i].Range.EndByte < diags[j].Range.EndByte
		}
		return diags[i].Kind < diags[j].Kind
	})
	return diags
}

// addDiagnostic 构造一条诊断并在未重复时追加到结果集。
func addDiagnostic(n *ts.Node, kind DiagnosticKind, msg string, seen map[diagKey]bool, out *[]Diagnostic) {
	r := n.Range()
	key := diagKey{startByte: r.StartByte, endByte: r.EndByte, kind: kind}
	if seen[key] {
		return
	}
	seen[key] = true
	*out = append(*out, Diagnostic{
		Kind:     kind,
		Message:  msg,
		Range:    toSourceRange(r),
		NodeKind: n.Kind(),
	})
}
