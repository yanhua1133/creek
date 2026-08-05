package unit

import (
	"context"
	"errors"
	"testing"

	parser "creek/01-parser"
)

// positiveCase 描述一个合法正例 fixture 及其期望识别出的语言。
type positiveCase struct {
	// name 是子测试名称。
	name string
	// path 是 fixture 相对路径。
	path string
	// lang 是期望识别出的语言。
	lang parser.Language
}

// positiveCases 覆盖五种语言的最小合法正例。
var positiveCases = []positiveCase{
	{"c", "fixture/c/positive/basic.c", parser.LanguageC},
	{"cpp", "fixture/cpp/positive/basic.cpp", parser.LanguageCPP},
	{"java", "fixture/java/positive/Basic.java", parser.LanguageJava},
	{"go", "fixture/go/positive/basic.go", parser.LanguageGo},
	{"python", "fixture/python/positive/basic.py", parser.LanguagePython},
}

// TestParsePositive 验证五种语言的合法正例都能解析出非空根节点、识别出正确语言且不产生诊断。
func TestParsePositive(t *testing.T) {
	p := parser.New()
	for _, c := range positiveCases {
		t.Run(c.name, func(t *testing.T) {
			file := loadFixture(t, c.path)
			res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			defer res.Tree.Close()
			if res.Language != c.lang {
				t.Errorf("识别语言 = %v，期望 %v", res.Language, c.lang)
			}
			root := res.Tree.Root()
			kind, err := root.Kind()
			if err != nil {
				t.Fatalf("读取根节点 kind 失败: %v", err)
			}
			if kind == "" {
				t.Error("根节点 kind 为空")
			}
			cnt, err := root.ChildCount()
			if err != nil {
				t.Fatalf("读取子节点数失败: %v", err)
			}
			if cnt == 0 {
				t.Error("合法正例根节点不应无子节点")
			}
			if len(res.Diagnostics) != 0 {
				t.Errorf("合法正例不应产生诊断，实得 %d 条: %+v", len(res.Diagnostics), res.Diagnostics)
			}
		})
	}
}

// boundaryLegalFixtures 是合法的边界 fixture（空文件、仅注释、最小文件），解析后不应有系统失败。
var boundaryLegalFixtures = []string{
	"fixture/c/boundary/empty.c",
	"fixture/c/boundary/comment_only.c",
	"fixture/cpp/boundary/empty.cpp",
	"fixture/cpp/boundary/comment_only.cpp",
	"fixture/java/boundary/Empty.java",
	"fixture/java/boundary/CommentOnly.java",
	"fixture/go/boundary/empty.go",
	"fixture/go/boundary/minimal.go",
	"fixture/python/boundary/empty.py",
	"fixture/python/boundary/comment_only.py",
}

// TestParseBoundary 验证空文件、仅注释文件等边界输入都能生成可访问的根节点且不 panic。
func TestParseBoundary(t *testing.T) {
	p := parser.New()
	for _, path := range boundaryLegalFixtures {
		t.Run(path, func(t *testing.T) {
			file := loadFixture(t, path)
			res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
			if err != nil {
				t.Fatalf("边界输入不应产生系统错误: %v", err)
			}
			defer res.Tree.Close()
			if _, err := res.Tree.Root().Kind(); err != nil {
				t.Fatalf("边界输入根节点不可访问: %v", err)
			}
		})
	}
}

// TestParseContextCanceled 验证已取消的上下文会使解析返回上下文错误而非语法树。
func TestParseContextCanceled(t *testing.T) {
	p := parser.New()
	file := loadFixture(t, "fixture/c/positive/basic.c")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := p.Parse(ctx, file, parser.ParseOptions{})
	if err == nil {
		res.Tree.Close()
		t.Fatal("已取消上下文应返回错误")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("期望 context.Canceled，实得 %v", err)
	}
}
