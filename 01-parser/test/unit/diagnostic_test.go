package unit

import (
	"context"
	"testing"

	parser "creek/01-parser"
)

// invalidFixtures 是各语言故意包含语法错误的 fixture，用作正报来源与诊断快照对象。
var invalidFixtures = map[string]string{
	"c":      "fixture/c/negative/broken.c",
	"cpp":    "fixture/cpp/negative/broken.cpp",
	"java":   "fixture/java/negative/Broken.java",
	"go":     "fixture/go/negative/broken.go",
	"python": "fixture/python/negative/broken.py",
}

// legalFixtures 是各语言合法源码 fixture，解析后必须不产生任何诊断。
var legalFixtures = []string{
	"fixture/c/positive/basic.c",
	"fixture/c/positive/header_c_style.h",
	"fixture/c/positive/cond_cplusplus.c",
	"fixture/c/positive/cond_define_undef.c",
	"fixture/c/positive/cond_elif_chain.c",
	"fixture/c/positive/cond_extern_c.h",
	"fixture/c/positive/cond_if_zero.c",
	"fixture/c/positive/cond_in_function.c",
	"fixture/c/positive/cond_nested.c",
	"fixture/c/regression/cond_continuation.c",
	"fixture/c/boundary/cond_comment_string.c",
	"fixture/c/boundary/cond_unterminated.c",
	"fixture/c/boundary/empty.c",
	"fixture/c/boundary/comment_only.c",
	"fixture/cpp/positive/basic.cpp",
	"fixture/cpp/positive/header_cpp_style.h",
	"fixture/cpp/positive/cond_extern_c.cpp",
	"fixture/cpp/regression/cond_raw_string.cpp",
	"fixture/cpp/boundary/empty.cpp",
	"fixture/cpp/boundary/comment_only.cpp",
	"fixture/java/positive/Basic.java",
	"fixture/java/boundary/Empty.java",
	"fixture/java/boundary/CommentOnly.java",
	"fixture/go/positive/basic.go",
	"fixture/go/boundary/empty.go",
	"fixture/go/boundary/minimal.go",
	"fixture/python/positive/basic.py",
	"fixture/python/boundary/empty.py",
	"fixture/python/boundary/comment_only.py",
}

// TestDiagnosticSnapshot 对每个故意错误 fixture 的诊断做结构级 golden 比较：把诊断序列化为含类型、
// 字节范围、行列范围和节点类型的规范化快照，与 .diag golden 逐字节比对，取代仅断言诊断数量的浅层检查。
func TestDiagnosticSnapshot(t *testing.T) {
	p := parser.New()
	for lang, path := range invalidFixtures {
		t.Run(lang, func(t *testing.T) {
			file := loadFixture(t, path)
			res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			defer res.Tree.Close()

			got := diagnosticsSnapshot(res.Diagnostics)
			if got == "" {
				t.Fatalf("故意错误 fixture 未产生任何诊断（漏报）")
			}
			assertGolden(t, expectedPath(path, ".diag"), got)
		})
	}
}

// TestLegalFixturesHaveNoDiagnostics 验证每个故意错误 fixture 都产生诊断、每个合法 fixture 都不产生诊断。
// Parser 是中间步骤，合法源码的诊断数必须为零，不使用任何比例阈值。
func TestLegalFixturesHaveNoDiagnostics(t *testing.T) {
	p := parser.New()

	truePositives := 0
	perLang := map[string]int{}
	for lang, path := range invalidFixtures {
		file := loadFixture(t, path)
		res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
		if err != nil {
			t.Fatalf("解析故意错误 fixture 失败 %s: %v", path, err)
		}
		if len(res.Diagnostics) > 0 {
			truePositives++
			perLang[lang]++
		}
		res.Tree.Close()
	}

	unexpected := 0
	for _, path := range legalFixtures {
		file := loadFixture(t, path)
		res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
		if err != nil {
			t.Fatalf("解析合法 fixture 失败 %s: %v", path, err)
		}
		if len(res.Diagnostics) > 0 {
			unexpected++
			t.Errorf("合法文件产生诊断: %s (%d 条)", path, len(res.Diagnostics))
		}
		res.Tree.Close()
	}
	t.Logf("指标 | 正报数=%d 合法文件数=%d 合法文件诊断数=%d", truePositives, len(legalFixtures), unexpected)

	for _, lang := range []string{"c", "cpp", "java", "go", "python"} {
		if perLang[lang] == 0 {
			t.Errorf("语言 %s 缺少有效正报", lang)
		}
	}
}
