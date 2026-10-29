package unit

import (
	"bytes"
	"context"
	"os"
	"testing"

	parser "creek/01-parser"
	commonast "creek/02-ast"
)

// TestConvertWithOptions 验证按需采集转换指标和稳定结构化快照。
func TestConvertWithOptions(t *testing.T) {
	parsed, err := parser.New().Parse(context.Background(), loadSourceFile(t, "c/boundary/empty.c"), parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析空 C fixture 失败: %v", err)
	}
	defer parsed.Tree.Close()
	result, err := commonast.New().ConvertWithOptions(context.Background(), parsed.Tree, commonast.ConversionOptions{
		CollectMetrics:  true,
		CollectSnapshot: true,
		EnableTrace:     true,
	})
	if err != nil {
		t.Fatalf("带选项转换失败: %v", err)
	}
	if result.AST == nil || result.Metrics == nil {
		t.Fatal("带选项转换未返回 AST 或指标")
	}
	if result.Metrics.NodeCount != 1 || result.Metrics.InputBytes != 0 || result.Metrics.DiagnosticCount != 0 {
		t.Fatalf("空文件转换指标错误: %+v", *result.Metrics)
	}
	expected, err := os.ReadFile(fixturePath("expected/empty-c.json"))
	if err != nil {
		t.Fatalf("读取 JSON golden 失败: %v", err)
	}
	if !bytes.Equal(result.Snapshot, expected) {
		t.Fatalf("结构化快照不一致\n=== 实际 ===\n%s\n=== 期望 ===\n%s", result.Snapshot, expected)
	}
}

// TestMarshalSnapshotDeterminism 验证独立快照接口输出确定且不改变 AST。
func TestMarshalSnapshotDeterminism(t *testing.T) {
	tree, _ := buildAST(t, "go/positive/basic.go")
	first, err := commonast.MarshalSnapshot(tree)
	if err != nil {
		t.Fatalf("首次生成快照失败: %v", err)
	}
	second, err := commonast.MarshalSnapshot(tree)
	if err != nil {
		t.Fatalf("再次生成快照失败: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("同一 AST 重复生成的结构化快照不一致")
	}
}

// TestConversionDepthOption 验证调用方可以收紧单次转换的递归深度限制。
func TestConversionDepthOption(t *testing.T) {
	parsed, err := parser.New().Parse(context.Background(), loadSourceFile(t, "c/positive/basic.c"), parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析 C fixture 失败: %v", err)
	}
	defer parsed.Tree.Close()
	result, err := commonast.New().ConvertWithOptions(context.Background(), parsed.Tree, commonast.ConversionOptions{MaxDepth: 1})
	if err != nil {
		t.Fatalf("深度受限转换失败: %v", err)
	}
	found := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Kind == commonast.DiagnosticDepthLimit {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("收紧递归深度后未产生深度限制诊断")
	}
}
