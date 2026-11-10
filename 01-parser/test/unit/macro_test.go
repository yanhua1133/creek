package unit

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	parser "creek/01-parser"
)

// macroProjectDir 是宏中和单元测试使用的小项目目录。
const macroProjectDir = "fixture/macro"

// macroLegalFiles 是带索引解析后必须没有任何诊断的宏小项目使用文件。
var macroLegalFiles = []string{
	"fixture/macro/lib/helper.c",
	"fixture/macro/lib/regression/debug_assert.c",
	"fixture/macro/lib/sub/use.c",
	"fixture/macro/lib/sub/use.h",
	"fixture/macro/other/use.cpp",
}

// macroProjectFiles 返回宏小项目中的全部文件路径（含 .inc），按字典序排列。
func macroProjectFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(macroProjectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历宏小项目失败: %v", err)
	}
	sort.Strings(files)
	return files
}

// macroSourceFiles 返回宏小项目中可直接解析的 C/C++ 源码文件。
func macroSourceFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, path := range macroProjectFiles(t) {
		if fixtureSourceExts[filepath.Ext(path)] {
			files = append(files, path)
		}
	}
	return files
}

// buildMacroIndex 读取给定文件并建立宏定义索引；非宏定义文件由索引自行忽略。
func buildMacroIndex(t *testing.T, paths []string) *parser.MacroIndex {
	t.Helper()
	files := make([]parser.SourceFile, 0, len(paths))
	for i, path := range paths {
		file := loadFixture(t, path)
		file.ID = parser.FileID(i + 1)
		files = append(files, file)
	}
	return parser.BuildMacroIndex(files)
}

// macroSnapshot 把宏中和报告序列化为逐行文本，行号和列号从 1 开始。
func macroSnapshot(report *parser.MacroReport) string {
	if report == nil {
		return "nil\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "rewrites %d\n", len(report.Rewrites))
	fmt.Fprintf(&b, "unsupported %d\n", report.Unsupported)
	fmt.Fprintf(&b, "too_long %d\n", report.TooLong)
	for _, rewrite := range report.Rewrites {
		fmt.Fprintf(&b, "rewrite line=%d column=%d name=%s replacement=%q definition=%s:%d\n",
			rewrite.Range.StartPoint.Row+1, rewrite.Range.StartPoint.Column+1, rewrite.Name,
			rewrite.Replacement, rewrite.DefinitionPath, rewrite.DefinitionLine)
	}
	return b.String()
}

// formName 返回占位符形态在快照中的名称。
func formName(form parser.PlaceholderForm) string {
	switch form {
	case parser.PlaceholderExpression:
		return "expression"
	case parser.PlaceholderStatement:
		return "statement"
	case parser.PlaceholderDefinition:
		return "definition"
	case parser.PlaceholderOmitted:
		return "omitted"
	case parser.PlaceholderName:
		return "name"
	default:
		return fmt.Sprintf("invalid(%d)", form)
	}
}

// resultSnapshot 把宏中和报告和宏占位符序列化为逐行文本；没有占位符时只输出宏中和报告。
func resultSnapshot(res parser.ParseResult) string {
	var b strings.Builder
	b.WriteString(macroSnapshot(res.Macros))
	if len(res.Placeholders) == 0 {
		return b.String()
	}
	fmt.Fprintf(&b, "placeholders %d\n", len(res.Placeholders))
	for _, placeholder := range res.Placeholders {
		fmt.Fprintf(&b, "placeholder line=%d column=%d name=%s form=%s text=%q defined=%t\n",
			placeholder.Range.StartPoint.Row+1, placeholder.Range.StartPoint.Column+1, placeholder.Name,
			formName(placeholder.Form), placeholder.Text, placeholder.Defined)
	}
	return b.String()
}

// TestMacroReportSnapshot 不带索引解析全部 C/C++ fixture，宏报告和宏占位符与 .macro golden 逐字节比较，只有本文件定义参与宏中和。
func TestMacroReportSnapshot(t *testing.T) {
	p := parser.New()
	for _, path := range cFamilyFixtures(t) {
		t.Run(path, func(t *testing.T) {
			res, err := p.Parse(context.Background(), loadFixture(t, path), parser.ParseOptions{})
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			defer res.Tree.Close()
			assertGolden(t, expectedPath(path, ".macro"), resultSnapshot(res))
		})
	}
}

// TestMacroIndexedSnapshot 用宏小项目全部文件建索引后解析每个源码文件，语法树、宏报告和诊断分别与 golden 逐字节比较。
func TestMacroIndexedSnapshot(t *testing.T) {
	index := buildMacroIndex(t, macroProjectFiles(t))
	p := parser.New()
	for _, path := range macroSourceFiles(t) {
		t.Run(path, func(t *testing.T) {
			res, err := p.Parse(context.Background(), loadFixture(t, path), parser.ParseOptions{CollectDiagnostics: true, Macros: index})
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			defer res.Tree.Close()
			assertGolden(t, expectedPath(path, ".indexed.macro"), resultSnapshot(res))
			assertGolden(t, expectedPath(path, ".indexed.sexp"), treeSexpr(t, res.Tree.Root()))
			assertGolden(t, expectedPath(path, ".indexed.diag"), diagnosticsSnapshot(res.Diagnostics))
		})
	}
}

// TestMacroLegalFilesHaveNoDiagnostics 验证宏小项目中的合法使用文件带索引解析后没有诊断，只做分支选择时有诊断。
func TestMacroLegalFilesHaveNoDiagnostics(t *testing.T) {
	index := buildMacroIndex(t, macroProjectFiles(t))
	p := parser.New()
	fixed := 0
	for _, path := range macroLegalFiles {
		file := loadFixture(t, path)
		full, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true, Macros: index})
		if err != nil {
			t.Fatalf("解析失败 %s: %v", path, err)
		}
		if len(full.Diagnostics) != 0 {
			t.Errorf("宏中和后合法文件仍有诊断: %s %s", path, diagnosticsSnapshot(full.Diagnostics))
		}
		full.Tree.Close()
		conditional, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true, ConditionalOnly: true})
		if err != nil {
			t.Fatalf("只做分支选择解析失败 %s: %v", path, err)
		}
		if conditional.Macros != nil {
			t.Errorf("ConditionalOnly 为真时宏报告应为空: %s", path)
		}
		if len(conditional.Diagnostics) != 0 {
			fixed++
		}
		conditional.Tree.Close()
	}
	t.Logf("指标 | 宏中和消除诊断的文件数=%d 合法文件数=%d", fixed, len(macroLegalFiles))
	if fixed == 0 {
		t.Error("宏小项目中没有任何文件依赖宏中和才能无诊断解析，fixture 无法证明宏中和有效")
	}
}

// TestMacroViewLayout 验证宏中和视图与原文件等长、换行位置不变，且改动字节全部落在报告的改写范围内。
func TestMacroViewLayout(t *testing.T) {
	index := buildMacroIndex(t, macroProjectFiles(t))
	paths := append(cFamilyFixtures(t), macroSourceFiles(t)...)
	for _, path := range paths {
		for _, lang := range []parser.Language{parser.LanguageC, parser.LanguageCPP} {
			t.Run(fmt.Sprintf("%s/%v", path, lang), func(t *testing.T) {
				content := loadFixture(t, path).Content
				conditional, _ := parser.BuildConditionalView(content, lang)
				view, report := parser.BuildMacroView(conditional, path, lang, index)
				if len(view) != len(content) {
					t.Fatalf("宏中和视图长度=%d，原文件长度=%d", len(view), len(content))
				}
				for i := range conditional {
					if view[i] == conditional[i] {
						continue
					}
					if conditional[i] == '\n' || conditional[i] == '\r' || view[i] == '\n' || view[i] == '\r' {
						t.Fatalf("偏移 %d 改动了换行符", i)
					}
					inside := false
					for _, rewrite := range report.Rewrites {
						if uint(i) >= rewrite.Range.StartByte && uint(i) < rewrite.Range.EndByte {
							inside = true
							break
						}
					}
					if !inside {
						t.Fatalf("偏移 %d 的改动不在任何改写范围内", i)
					}
				}
			})
		}
	}
}

// TestMacroIndexOrderIndependent 验证索引构建结果与输入文件顺序无关。
func TestMacroIndexOrderIndependent(t *testing.T) {
	files := macroProjectFiles(t)
	reversed := make([]string, len(files))
	for i, path := range files {
		reversed[len(files)-1-i] = path
	}
	forward := buildMacroIndex(t, files)
	backward := buildMacroIndex(t, reversed)
	for _, path := range macroSourceFiles(t) {
		for _, lang := range []parser.Language{parser.LanguageC, parser.LanguageCPP} {
			conditional, _ := parser.BuildConditionalView(loadFixture(t, path).Content, lang)
			_, first := parser.BuildMacroView(conditional, path, lang, forward)
			_, second := parser.BuildMacroView(conditional, path, lang, backward)
			if macroSnapshot(&first) != macroSnapshot(&second) {
				t.Errorf("索引输入顺序改变了宏报告 %s/%v\n正序:\n%s倒序:\n%s", path, lang, macroSnapshot(&first), macroSnapshot(&second))
			}
		}
	}
}

// TestMacroIndexConcurrent 验证多个 goroutine 共享同一索引并发解析时结果一致，供 Race Detector 检查数据竞争。
func TestMacroIndexConcurrent(t *testing.T) {
	index := buildMacroIndex(t, macroProjectFiles(t))
	p := parser.New()
	paths := macroSourceFiles(t)
	want := make([]string, len(paths))
	for i, path := range paths {
		res, err := p.Parse(context.Background(), loadFixture(t, path), parser.ParseOptions{Macros: index})
		if err != nil {
			t.Fatalf("解析失败 %s: %v", path, err)
		}
		want[i] = macroSnapshot(res.Macros)
		res.Tree.Close()
	}
	var wg sync.WaitGroup
	errs := make(chan string, 8*len(paths))
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i, path := range paths {
				res, err := p.Parse(context.Background(), loadFixture(t, path), parser.ParseOptions{Macros: index})
				if err != nil {
					errs <- fmt.Sprintf("并发解析失败 %s: %v", path, err)
					continue
				}
				if got := macroSnapshot(res.Macros); got != want[i] {
					errs <- fmt.Sprintf("并发解析宏报告不一致 %s", path)
				}
				res.Tree.Close()
			}
		}()
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Error(msg)
	}
}

// TestIsMacroDefinitionFile 验证索引只收录头文件类扩展名，实现文件不收录。
func TestIsMacroDefinitionFile(t *testing.T) {
	for _, path := range macroProjectFiles(t) {
		ext := filepath.Ext(path)
		want := ext == ".h" || ext == ".inc"
		if got := parser.IsMacroDefinitionFile(path); got != want {
			t.Errorf("IsMacroDefinitionFile(%s)=%v，期望 %v", path, got, want)
		}
	}
}

// TestParseMetricsToggle 验证耗时指标默认关闭，打开后返回非负的各环节耗时。
func TestParseMetricsToggle(t *testing.T) {
	p := parser.New()
	file := loadFixture(t, "fixture/macro/lib/sub/use.h")
	off, err := p.Parse(context.Background(), file, parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	off.Tree.Close()
	if off.Metrics != nil {
		t.Errorf("CollectMetrics 为假时 Metrics 应为空")
	}
	on, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectMetrics: true})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	defer on.Tree.Close()
	if on.Metrics == nil {
		t.Fatalf("CollectMetrics 为真时 Metrics 不应为空")
	}
	if on.Metrics.Conditional < 0 || on.Metrics.Macro < 0 || on.Metrics.TreeSitter <= 0 {
		t.Errorf("耗时指标不合法: %+v", *on.Metrics)
	}
}

// findNode 按源码顺序返回第一个类型为 kind 的节点。
func findNode(t *testing.T, node parser.Node, kind string) (parser.Node, bool) {
	t.Helper()
	got, err := node.Kind()
	if err != nil {
		t.Fatalf("读取节点类型失败: %v", err)
	}
	if got == kind {
		return node, true
	}
	count, err := node.ChildCount()
	if err != nil {
		t.Fatalf("读取子节点数失败: %v", err)
	}
	for i := 0; i < count; i++ {
		child, ok, err := node.Child(i)
		if err != nil {
			t.Fatalf("读取子节点失败: %v", err)
		}
		if ok {
			if found, ok := findNode(t, child, kind); ok {
				return found, true
			}
		}
	}
	return nil, false
}

// TestMacroRewrittenNodeText 验证宏改写生成的关键字节点返回写入的关键字，包含宏调用的外层节点返回原始源码。
func TestMacroRewrittenNodeText(t *testing.T) {
	index := buildMacroIndex(t, macroProjectFiles(t))
	res, err := parser.New().Parse(context.Background(), loadFixture(t, "fixture/macro/lib/sub/use.c"), parser.ParseOptions{Macros: index})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	defer res.Tree.Close()
	function, ok := findNode(t, res.Tree.Root(), "function_definition")
	if !ok {
		t.Fatal("未找到函数定义节点")
	}
	specifier, ok := findNode(t, function, "storage_class_specifier")
	if !ok {
		t.Fatal("未找到 LIB_INLINE 改写生成的存储类说明符节点")
	}
	if text, err := specifier.Text(); err != nil || string(text) != "static" {
		t.Errorf("改写生成的说明符节点文本=%q err=%v，期望 \"static\"", text, err)
	}
	text, err := function.Text()
	if err != nil {
		t.Fatalf("读取函数定义文本失败: %v", err)
	}
	if !strings.HasPrefix(string(text), "LIB_INLINE int c_twice") {
		t.Errorf("函数定义文本应保留原始源码中的宏名，实际为 %q", text)
	}
}

// placeholderFixed 是放置宏占位符后必须没有诊断、不放置时必须有诊断的 fixture。
var placeholderFixed = []string{
	"fixture/macro/placeholder/block_in_function.c",
	"fixture/macro/placeholder/catch_def.cpp",
	"fixture/macro/placeholder/expression_args.c",
	"fixture/macro/placeholder/external_attr.cpp",
	"fixture/macro/placeholder/initializer_list.c",
	"fixture/macro/placeholder/struct_name.c",
	"fixture/macro/placeholder/lock_annotation.cpp",
	"fixture/macro/placeholder/enum_name.c",
	"fixture/macro/placeholder/decl_annotation.cpp",
	"fixture/macro/placeholder/switch_stmt.cpp",
}

// TestPlaceholderFixesErrors 验证宏占位符消除错误、SkipPlaceholders 可关闭，以及无错误文件和放置无效时不留下占位符。
func TestPlaceholderFixesErrors(t *testing.T) {
	p := parser.New()
	for _, path := range placeholderFixed {
		file := loadFixture(t, path)
		res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
		if err != nil {
			t.Fatalf("解析失败 %s: %v", path, err)
		}
		if len(res.Diagnostics) != 0 || len(res.Placeholders) == 0 {
			t.Errorf("%s 放置占位符后应无诊断且有占位符: 诊断=%d 占位符=%d", path, len(res.Diagnostics), len(res.Placeholders))
		}
		if !reflect.DeepEqual(res.Placeholders, res.Tree.Placeholders()) {
			t.Errorf("%s ParseResult.Placeholders 与 SyntaxTree.Placeholders() 不一致", path)
		}
		res.Tree.Close()
		skipped, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true, SkipPlaceholders: true})
		if err != nil {
			t.Fatalf("解析失败 %s: %v", path, err)
		}
		if len(skipped.Diagnostics) == 0 || len(skipped.Placeholders) != 0 {
			t.Errorf("%s SkipPlaceholders 时应有诊断且无占位符: 诊断=%d 占位符=%d", path, len(skipped.Diagnostics), len(skipped.Placeholders))
		}
		skipped.Tree.Close()
	}
	for path, wantDiagnostics := range map[string]bool{
		"fixture/macro/placeholder/clean.cpp":         false,
		"fixture/macro/placeholder/unrelated_error.c": true,
	} {
		res, err := p.Parse(context.Background(), loadFixture(t, path), parser.ParseOptions{CollectDiagnostics: true})
		if err != nil {
			t.Fatalf("解析失败 %s: %v", path, err)
		}
		if len(res.Placeholders) != 0 || (len(res.Diagnostics) != 0) != wantDiagnostics {
			t.Errorf("%s 不应留下占位符，诊断存在应为 %t: 诊断=%d 占位符=%d", path, wantDiagnostics, len(res.Diagnostics), len(res.Placeholders))
		}
		res.Tree.Close()
	}
}

// TestPlaceholderNodeText 验证占位符范围内的节点返回写入的文本，外层节点返回原始源码。
func TestPlaceholderNodeText(t *testing.T) {
	res, err := parser.New().Parse(context.Background(), loadFixture(t, "fixture/macro/placeholder/catch_def.cpp"), parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	defer res.Tree.Close()
	function, ok := findNode(t, res.Tree.Root(), "function_definition")
	if !ok {
		t.Fatal("未找到占位符生成的函数定义")
	}
	returnType, ok := findNode(t, function, "primitive_type")
	if !ok {
		t.Fatal("未找到占位符写入的 void")
	}
	if text, err := returnType.Text(); err != nil || string(text) != "void" {
		t.Errorf("占位符写入的返回类型文本=%q err=%v，期望 \"void\"", text, err)
	}
	text, err := function.Text()
	if err != nil {
		t.Fatalf("读取函数定义文本失败: %v", err)
	}
	if !strings.HasPrefix(string(text), `TEST_CASE("parses macro", "[parser]") {`) {
		t.Errorf("函数定义文本应为原始源码，实际为 %q", text)
	}
}
