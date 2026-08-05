package unit

import (
	"context"
	"errors"
	"testing"

	parser "creek/01-parser"
)

// TestUnsupportedExtension 验证未知扩展名返回 ErrUnsupportedExtension，且不读取任何编译信息补救。
func TestUnsupportedExtension(t *testing.T) {
	p := parser.New()
	file := parser.SourceFile{ID: 1, Path: "fixture/unknown/sample.unknown", Content: []byte("x = 1")}
	_, err := p.Parse(context.Background(), file, parser.ParseOptions{})
	if !errors.Is(err, parser.ErrUnsupportedExtension) {
		t.Fatalf("期望 ErrUnsupportedExtension，实得 %v", err)
	}
}

// TestHeaderPrefersCPP 验证含 C++ 语法的 .h 头文件双解析后选择 C++，且 C 解析错误多于 C++。
func TestHeaderPrefersCPP(t *testing.T) {
	p := parser.New()
	file := loadFixture(t, "fixture/cpp/positive/header_cpp_style.h")
	res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
	if err != nil {
		t.Fatalf("头文件解析失败: %v", err)
	}
	defer res.Tree.Close()
	if res.Header == nil {
		t.Fatal(".h 文件应记录双解析择优过程")
	}
	if res.Language != parser.LanguageCPP {
		t.Errorf("含 C++ 语法的头文件应选择 C++，实得 %v", res.Language)
	}
	if res.Header.CErrorCount <= res.Header.CPPErrorCount {
		t.Errorf("C 解析错误数(%d)应多于 C++(%d)", res.Header.CErrorCount, res.Header.CPPErrorCount)
	}
}

// TestHeaderCStyleResolves 验证纯 C 风格 .h 头文件双解析都不发生系统失败并能生成非空根节点。
func TestHeaderCStyleResolves(t *testing.T) {
	p := parser.New()
	file := loadFixture(t, "fixture/c/positive/header_c_style.h")
	res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
	if err != nil {
		t.Fatalf("头文件解析失败: %v", err)
	}
	defer res.Tree.Close()
	if res.Header == nil {
		t.Fatal(".h 文件应记录双解析择优过程")
	}
	if res.Language != parser.LanguageC && res.Language != parser.LanguageCPP {
		t.Errorf("头文件应择优为 C 或 C++，实得 %v", res.Language)
	}
	if _, err := res.Tree.Root().Kind(); err != nil {
		t.Fatalf("头文件根节点不可访问: %v", err)
	}
}
