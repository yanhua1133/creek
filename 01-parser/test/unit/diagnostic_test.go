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

// legalFixtures 是各语言合法源码 fixture，用作误报统计的合法文件总体。
var legalFixtures = []string{
	"fixture/c/positive/basic.c",
	"fixture/c/positive/header_c_style.h",
	"fixture/c/boundary/empty.c",
	"fixture/c/boundary/comment_only.c",
	"fixture/cpp/positive/basic.cpp",
	"fixture/cpp/positive/header_cpp_style.h",
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

// TestFalsePositiveRate 遍历合法与错误 fixture，统计正报数、合法文件数、误报数与误报率，
// 校验最低验收标准：五种语言均有正报、总体误报率低于 30%。此测试提供指标口径，功能正确性由快照测试保证。
func TestFalsePositiveRate(t *testing.T) {
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

	falsePositives := 0
	for _, path := range legalFixtures {
		file := loadFixture(t, path)
		res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
		if err != nil {
			t.Fatalf("解析合法 fixture 失败 %s: %v", path, err)
		}
		if len(res.Diagnostics) > 0 {
			falsePositives++
			t.Logf("合法文件产生非预期诊断: %s (%d 条)", path, len(res.Diagnostics))
		}
		res.Tree.Close()
	}

	total := len(legalFixtures)
	rate := float64(falsePositives) / float64(total)
	t.Logf("指标 | 正报数=%d 合法文件数=%d 误报数=%d 误报率=%.1f%%",
		truePositives, total, falsePositives, rate*100)

	for _, lang := range []string{"c", "cpp", "java", "go", "python"} {
		if perLang[lang] == 0 {
			t.Errorf("语言 %s 缺少有效正报", lang)
		}
	}
	if truePositives == 0 {
		t.Error("必须存在有效正报")
	}
	if rate >= 0.30 {
		t.Errorf("误报率 %.1f%% 不得达到或超过 30%%", rate*100)
	}
}
