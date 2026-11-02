package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	parser "creek/01-parser"
)

// cFamilyExts 是需要做条件编译分支选择的 C/C++ fixture 扩展名。
var cFamilyExts = map[string]bool{".c": true, ".h": true, ".cpp": true}

// cFamilyFixtures 返回全部 C/C++ fixture 路径。
func cFamilyFixtures(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, path := range sourceFixtureFiles(t) {
		if cFamilyExts[path[strings.LastIndex(path, "."):]] {
			files = append(files, path)
		}
	}
	return files
}

// conditionValueName 返回条件求值结果在报告快照中的名称。
func conditionValueName(value parser.ConditionValue) string {
	switch value {
	case parser.ConditionTrue:
		return "true"
	case parser.ConditionFalse:
		return "false"
	case parser.ConditionUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("invalid(%d)", value)
	}
}

// preprocessSnapshot 把条件编译报告序列化为逐行文本，行号从 1 开始。
func preprocessSnapshot(report *parser.PreprocessReport) string {
	if report == nil {
		return "nil\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "blanked_bytes %d\n", report.BlankedBytes)
	fmt.Fprintf(&b, "unterminated %d\n", report.Unterminated)
	for _, branch := range report.Branches {
		fmt.Fprintf(&b, "branch line=%d keyword=%s value=%s taken=%t condition=%q\n",
			branch.Directive.StartPoint.Row+1, branch.Keyword, conditionValueName(branch.Value), branch.Taken, branch.Condition)
	}
	return b.String()
}

// TestPreprocessReportSnapshot 把每个 C/C++ fixture 的条件编译报告与 .pp golden 逐字节比较。
func TestPreprocessReportSnapshot(t *testing.T) {
	p := parser.New()
	for _, path := range cFamilyFixtures(t) {
		t.Run(path, func(t *testing.T) {
			res, err := p.Parse(context.Background(), loadFixture(t, path), parser.ParseOptions{})
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			defer res.Tree.Close()
			assertGolden(t, expectedPath(path, ".pp"), preprocessSnapshot(res.Preprocess))
		})
	}
}

// TestConditionalViewLayout 验证解析视图与原文件等长，且只把非换行字节替换为空格。
func TestConditionalViewLayout(t *testing.T) {
	for _, path := range cFamilyFixtures(t) {
		for _, lang := range []parser.Language{parser.LanguageC, parser.LanguageCPP} {
			t.Run(fmt.Sprintf("%s/%v", path, lang), func(t *testing.T) {
				content := loadFixture(t, path).Content
				view, report := parser.BuildConditionalView(content, lang)
				if len(view) != len(content) {
					t.Fatalf("解析视图长度=%d，原文件长度=%d", len(view), len(content))
				}
				changed := 0
				for i := range content {
					if view[i] == content[i] {
						continue
					}
					changed++
					if view[i] != ' ' || content[i] == '\n' || content[i] == '\r' {
						t.Fatalf("偏移 %d 的替换不合法：原字节=%q 视图字节=%q", i, content[i], view[i])
					}
				}
				if report.BlankedBytes < changed {
					t.Fatalf("报告替换字节数=%d 小于实际改动字节数=%d", report.BlankedBytes, changed)
				}
			})
		}
	}
}

// TestCplusplusDependsOnGrammar 验证 __cplusplus 按本次使用的 grammar 求值。
func TestCplusplusDependsOnGrammar(t *testing.T) {
	content := loadFixture(t, "fixture/c/positive/cond_cplusplus.c").Content
	_, cReport := parser.BuildConditionalView(content, parser.LanguageC)
	_, cppReport := parser.BuildConditionalView(content, parser.LanguageCPP)
	if len(cReport.Branches) != 2 || len(cppReport.Branches) != 2 {
		t.Fatalf("分支数 C=%d C++=%d，期望均为 2", len(cReport.Branches), len(cppReport.Branches))
	}
	for i := range cReport.Branches {
		if cReport.Branches[i].Value != parser.ConditionFalse || cReport.Branches[i].Taken {
			t.Errorf("C grammar 第 %d 个分支应为假且不保留: %+v", i, cReport.Branches[i])
		}
		if cppReport.Branches[i].Value != parser.ConditionTrue || !cppReport.Branches[i].Taken {
			t.Errorf("C++ grammar 第 %d 个分支应为真且保留: %+v", i, cppReport.Branches[i])
		}
	}
}

// TestRawSourceSkipsPreprocess 验证 RawSource 选项直接解析原始源码，不生成报告。
func TestRawSourceSkipsPreprocess(t *testing.T) {
	p := parser.New()
	file := loadFixture(t, "fixture/c/positive/cond_if_zero.c")
	raw, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true, RawSource: true})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	defer raw.Tree.Close()
	if raw.Preprocess != nil {
		t.Errorf("RawSource 为真时报告应为空")
	}
	if len(raw.Diagnostics) == 0 {
		t.Errorf("原始源码中 #if 0 内的非法代码应产生诊断")
	}
}

// TestPreprocessOnlyForCFamily 验证非 C/C++ 文件不生成条件编译报告。
func TestPreprocessOnlyForCFamily(t *testing.T) {
	p := parser.New()
	res, err := p.Parse(context.Background(), loadFixture(t, "fixture/go/positive/basic.go"), parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	defer res.Tree.Close()
	if res.Preprocess != nil {
		t.Errorf("Go 文件不应生成条件编译报告")
	}
}

// TestNodeTextUsesOriginalSource 验证节点文本取自原始源码，而不是替换后的解析视图。
func TestNodeTextUsesOriginalSource(t *testing.T) {
	p := parser.New()
	file := loadFixture(t, "fixture/c/positive/cond_in_function.c")
	res, err := p.Parse(context.Background(), file, parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	defer res.Tree.Close()
	function, ok, err := res.Tree.Root().Child(0)
	if err != nil || !ok {
		t.Fatalf("读取函数定义节点失败: ok=%v err=%v", ok, err)
	}
	text, err := function.Text()
	if err != nil {
		t.Fatalf("读取函数定义文本失败: %v", err)
	}
	if !strings.Contains(string(text), "garbage {") {
		t.Errorf("函数定义文本应包含原始源码中被丢弃分支的内容，实际为 %q", text)
	}
}
