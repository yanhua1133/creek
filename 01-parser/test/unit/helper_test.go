// Package unit 是 Parser 模块的黑盒单元测试包，只通过公开接口访问被测模块，
// 测试数据全部来自 fixture 目录下的独立文件。
package unit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	parser "creek/01-parser"
)

// fixtureSourceExts 是 fixture 中被视为源码（需解析并快照）的扩展名集合，排除 .sexp、.diag 等 golden 文件。
var fixtureSourceExts = map[string]bool{
	".c": true, ".h": true, ".cpp": true, ".java": true, ".go": true, ".py": true,
}

// loadFixture 读取指定相对路径的 fixture 文件并包装为 SourceFile；读取失败直接终止测试。
func loadFixture(t *testing.T, rel string) parser.SourceFile {
	t.Helper()
	content, err := os.ReadFile(rel)
	if err != nil {
		t.Fatalf("读取 fixture 失败 %s: %v", rel, err)
	}
	return parser.SourceFile{ID: 1, Path: rel, Content: content}
}

// expectedPath 把 fixture 输入路径映射到平行的 expected 目录下的期望文件路径，二者按相对路径一一对应。
// 例如 fixture/c/positive/basic.c 加后缀 .sexp 得到 expected/c/positive/basic.c.sexp。
func expectedPath(fixtureRel, suffix string) string {
	return strings.Replace(fixtureRel, "fixture/", "expected/", 1) + suffix
}

// sourceFixtureFiles 枚举 fixture 目录下全部源码文件（按扩展名过滤），路径按字典序稳定排序。
func sourceFixtureFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir("fixture", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if fixtureSourceExts[strings.ToLower(filepath.Ext(path))] {
			files = append(files, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历 fixture 失败: %v", err)
	}
	sort.Strings(files)
	return files
}

// treeSexpr 把语法树序列化为规范化的 S-expression 快照：只包含命名节点、带字段名前缀、
// 按源码顺序排列、每层缩进两个空格。快照不含字节范围，因此对源码微调稳定，用于结构级 golden 比较。
func treeSexpr(t *testing.T, n parser.Node) string {
	t.Helper()
	var b strings.Builder
	writeSexpr(t, &b, n, 0, "")
	return b.String()
}

// writeSexpr 递归地把一个节点及其命名子节点写入 S-expression 构造器。
// field 是该节点在父节点中的字段名，无字段名时为空。
func writeSexpr(t *testing.T, b *strings.Builder, n parser.Node, depth int, field string) {
	t.Helper()
	kind, err := n.Kind()
	if err != nil {
		t.Fatalf("读取节点 kind 失败: %v", err)
	}
	b.WriteString(strings.Repeat("  ", depth))
	if field != "" {
		b.WriteString(field)
		b.WriteString(": ")
	}
	b.WriteString("(")
	b.WriteString(kind)

	count, err := n.ChildCount()
	if err != nil {
		t.Fatalf("读取子节点数失败: %v", err)
	}
	for i := 0; i < count; i++ {
		child, ok, err := n.Child(i)
		if err != nil {
			t.Fatalf("读取子节点失败: %v", err)
		}
		if !ok {
			continue
		}
		named, err := child.IsNamed()
		if err != nil {
			t.Fatalf("读取命名标志失败: %v", err)
		}
		if !named {
			continue
		}
		fieldName, err := n.FieldNameForChild(i)
		if err != nil {
			t.Fatalf("读取字段名失败: %v", err)
		}
		b.WriteString("\n")
		writeSexpr(t, b, child, depth+1, fieldName)
	}
	b.WriteString(")")
}

// diagKindName 返回诊断类型的稳定短名称，用于诊断快照。
func diagKindName(k parser.DiagnosticKind) string {
	switch k {
	case parser.DiagnosticSyntaxError:
		return "syntax_error"
	case parser.DiagnosticMissingToken:
		return "missing_token"
	default:
		return "unknown"
	}
}

// diagnosticsSnapshot 把诊断列表序列化为稳定的规范化快照，每条一行，包含类型、字节范围、行列范围和节点类型，
// 用于对诊断这一核心产出做结构级 golden 比较。
func diagnosticsSnapshot(diags []parser.Diagnostic) string {
	var b strings.Builder
	for _, d := range diags {
		fmt.Fprintf(&b, "%s %d-%d (%d:%d-%d:%d) node=%s\n",
			diagKindName(d.Kind),
			d.Range.StartByte, d.Range.EndByte,
			d.Range.StartPoint.Row, d.Range.StartPoint.Column,
			d.Range.EndPoint.Row, d.Range.EndPoint.Column,
			d.NodeKind)
	}
	return b.String()
}
