package unit

import (
	"context"
	"errors"
	"sync"
	"testing"

	parser "creek/01-parser"
	commonast "creek/02-ast"
)

// TestClosedSyntaxTree 验证转换已关闭的上游语法树会返回明确错误。
func TestClosedSyntaxTree(t *testing.T) {
	parsed, err := parser.New().Parse(context.Background(), loadSourceFile(t, "c/positive/basic.c"), parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析 fixture 失败: %v", err)
	}
	if err := parsed.Tree.Close(); err != nil {
		t.Fatalf("关闭语法树失败: %v", err)
	}
	_, _, err = commonast.New().Convert(context.Background(), parsed.Tree)
	if !errors.Is(err, commonast.ErrInvalidSyntaxTree) {
		t.Fatalf("关闭后转换错误 = %v，期望 ErrInvalidSyntaxTree", err)
	}
}

// TestCanceledContext 验证已取消上下文会立即终止转换。
func TestCanceledContext(t *testing.T) {
	parsed, err := parser.New().Parse(context.Background(), loadSourceFile(t, "cpp/positive/basic.cpp"), parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析 fixture 失败: %v", err)
	}
	defer parsed.Tree.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = commonast.New().Convert(ctx, parsed.Tree)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("取消后错误 = %v，期望 context.Canceled", err)
	}
}

// TestConcurrentConversion 验证同一棵只读 CST 并发转换不会串 NodeID、角色或诊断状态。
func TestConcurrentConversion(t *testing.T) {
	parsed, err := parser.New().Parse(context.Background(), loadSourceFile(t, "go/positive/basic.go"), parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析 fixture 失败: %v", err)
	}
	defer parsed.Tree.Close()

	const workers = 16
	results := make(chan string, workers)
	errorsChannel := make(chan error, workers)
	var waitGroup sync.WaitGroup
	for index := 0; index < workers; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			tree, diagnostics, convertErr := commonast.New().Convert(context.Background(), parsed.Tree)
			if convertErr != nil {
				errorsChannel <- convertErr
				return
			}
			if len(diagnostics) != 0 {
				errorsChannel <- errors.New("并发转换产生非预期诊断")
				return
			}
			results <- snapshotAST(tree)
		}()
	}
	waitGroup.Wait()
	close(results)
	close(errorsChannel)
	for convertErr := range errorsChannel {
		t.Fatalf("并发转换失败: %v", convertErr)
	}
	var expected string
	for result := range results {
		if expected == "" {
			expected = result
			continue
		}
		if result != expected {
			t.Fatal("并发转换结果不一致")
		}
	}
}

// TestUpstreamSyntaxError 验证上游错误节点只产生诊断，不阻止输出部分 Common AST。
func TestUpstreamSyntaxError(t *testing.T) {
	parsed, err := parser.New().Parse(context.Background(), loadSourceFile(t, "c/negative/broken.c"), parser.ParseOptions{CollectDiagnostics: true})
	if err != nil {
		t.Fatalf("解析错误 fixture 失败: %v", err)
	}
	defer parsed.Tree.Close()
	if len(parsed.Diagnostics) == 0 {
		t.Fatal("错误 fixture 未产生 Parser 诊断")
	}
	tree, diagnostics, err := commonast.New().Convert(context.Background(), parsed.Tree)
	if err != nil {
		t.Fatalf("错误 CST 不应导致系统失败: %v", err)
	}
	if tree.Root().Kind() != commonast.KindFile {
		t.Fatalf("部分 AST 根节点 = %s，期望 File", tree.Root().Kind())
	}
	found := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == commonast.DiagnosticUpstreamSyntax {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("未记录上游语法错误诊断: %+v", diagnostics)
	}
}
