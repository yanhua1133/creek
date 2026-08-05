package unit

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	parser "creek/01-parser"
)

// updateGolden 为真时重新生成 golden 快照文件；通过 go test -update 触发。
var updateGolden = flag.Bool("update", false, "重新生成语法树与诊断 golden 快照文件")

// TestSyntaxTreeSnapshot 对 fixture 下**全部**源码文件（正例、负例、边界）解析后把语法树序列化为
// S-expression，与同名 .sexp golden 逐字节比较。负例的部分树同样纳入，从而验证错误恢复产出的结构；
// 这取代了仅断言根节点非空的浅层检查。
func TestSyntaxTreeSnapshot(t *testing.T) {
	p := parser.New()
	for _, path := range sourceFixtureFiles(t) {
		t.Run(path, func(t *testing.T) {
			file := loadFixture(t, path)
			res, err := p.Parse(context.Background(), file, parser.ParseOptions{})
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			defer res.Tree.Close()

			got := treeSexpr(t, res.Tree.Root())
			assertGolden(t, expectedPath(path, ".sexp"), got)
		})
	}
}

// TestSnapshotDeterminism 验证同一输入两次解析产生逐字节一致的语法树快照，取代仅比较根节点类型的弱确定性检查。
func TestSnapshotDeterminism(t *testing.T) {
	p := parser.New()
	file := loadFixture(t, "fixture/cpp/positive/basic.cpp")

	first, err := p.Parse(context.Background(), file, parser.ParseOptions{})
	if err != nil {
		t.Fatalf("首次解析失败: %v", err)
	}
	defer first.Tree.Close()
	second, err := p.Parse(context.Background(), file, parser.ParseOptions{})
	if err != nil {
		t.Fatalf("二次解析失败: %v", err)
	}
	defer second.Tree.Close()

	if treeSexpr(t, first.Tree.Root()) != treeSexpr(t, second.Tree.Root()) {
		t.Error("同一输入两次解析的语法树快照不一致")
	}
}

// assertGolden 在 -update 模式下写入 golden，否则读取 golden 并与实际内容逐字节比较。
func assertGolden(t *testing.T, goldenPath, got string) {
	t.Helper()
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("创建 expected 目录失败: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("写入 golden 失败: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("读取 golden 失败（首次请运行 go test -update 生成）: %v", err)
	}
	if got != string(want) {
		t.Errorf("快照与 golden 不一致 %s\n=== 实际 ===\n%s\n=== 期望 ===\n%s", goldenPath, got, string(want))
	}
}
