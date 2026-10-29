package unit

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	parser "creek/01-parser"
	commonast "creek/02-ast"
)

// updateGolden 指定是否重新生成 AST golden 文件。
var updateGolden = flag.Bool("update", false, "重新生成 Common AST golden 文件")

// fixturePath 返回当前测试包中的 fixture 绝对路径。
func fixturePath(relative string) string {
	return filepath.Join("fixture", filepath.FromSlash(relative))
}

// loadSourceFile 从独立 fixture 文件加载待解析源码，项目内路径取相对 fixture 目录的路径。
func loadSourceFile(t *testing.T, relative string) parser.SourceFile {
	t.Helper()
	path := fixturePath(relative)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 fixture 失败 %s: %v", path, err)
	}
	return parser.SourceFile{ID: 1, Path: relative, Content: content}
}

// buildAST 先解析 fixture，再转换为 Common AST，并在测试结束前关闭语法树。
func buildAST(t *testing.T, relative string) (commonast.CommonAST, []commonast.Diagnostic) {
	t.Helper()
	parsed, err := parser.New().Parse(context.Background(), loadSourceFile(t, relative), parser.ParseOptions{CollectDiagnostics: true})
	if err != nil {
		t.Fatalf("解析 fixture 失败 %s: %v", relative, err)
	}
	defer parsed.Tree.Close()
	if len(parsed.Diagnostics) != 0 {
		t.Fatalf("fixture 存在非预期语法诊断 %s: %+v", relative, parsed.Diagnostics)
	}
	result, diagnostics, err := commonast.New().Convert(context.Background(), parsed.Tree)
	if err != nil {
		t.Fatalf("转换 Common AST 失败 %s: %v", relative, err)
	}
	return result, diagnostics
}

// snapshotNode 把 AST 节点序列化为稳定的缩进文本。
func snapshotNode(node commonast.Node, depth int, output *strings.Builder) {
	output.WriteString(strings.Repeat("  ", depth))
	output.WriteString(node.Kind().String())
	if text := strings.TrimSpace(node.Text()); text != "" && node.ChildCount() == 0 {
		output.WriteString(" ")
		output.WriteString(fmt.Sprintf("%q", text))
	}
	output.WriteByte('\n')
	for index := 0; index < node.ChildCount(); index++ {
		child, ok := node.Child(index)
		if !ok {
			continue
		}
		snapshotNode(child, depth+1, output)
	}
}

// snapshotAST 返回整棵 AST 的稳定文本快照。
func snapshotAST(tree commonast.CommonAST) string {
	var output strings.Builder
	snapshotNode(tree.Root(), 0, &output)
	return output.String()
}

// assertGolden 比较实际 AST 快照与独立 golden 文件。
func assertGolden(t *testing.T, name, actual string) {
	t.Helper()
	path := fixturePath(filepath.Join("expected", name+".ast"))
	if *updateGolden {
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatalf("写入 golden 失败 %s: %v", path, err)
		}
		return
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 golden 失败 %s: %v", path, err)
	}
	if actual != string(expected) {
		t.Fatalf("AST 快照不一致 %s\n=== 实际 ===\n%s\n=== 期望 ===\n%s", name, actual, string(expected))
	}
}

// findFirst 返回深度优先遍历中第一个指定类型的节点。
func findFirst(node commonast.Node, kind commonast.NodeKind) (commonast.Node, bool) {
	if node.Kind() == kind {
		return node, true
	}
	for index := 0; index < node.ChildCount(); index++ {
		child, ok := node.Child(index)
		if !ok {
			continue
		}
		if found, ok := findFirst(child, kind); ok {
			return found, true
		}
	}
	return nil, false
}

// countKind 统计子树中指定节点类型的数量。
func countKind(node commonast.Node, kind commonast.NodeKind) int {
	count := 0
	if node.Kind() == kind {
		count++
	}
	for index := 0; index < node.ChildCount(); index++ {
		child, ok := node.Child(index)
		if ok {
			count += countKind(child, kind)
		}
	}
	return count
}

// structuralShape 返回忽略名称、字面量和类型细节后的控制结构骨架。
func structuralShape(node commonast.Node, output *strings.Builder) {
	if commonast.IsStructuralKind(node.Kind()) {
		output.WriteString(node.Kind().String())
		output.WriteByte('(')
		for index := 0; index < node.ChildCount(); index++ {
			child, ok := node.Child(index)
			if ok {
				structuralShape(child, output)
			}
		}
		output.WriteByte(')')
		return
	}
	for index := 0; index < node.ChildCount(); index++ {
		child, ok := node.Child(index)
		if ok {
			structuralShape(child, output)
		}
	}
}
