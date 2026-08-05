//go:build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	parser "creek/01-parser"
)

// minSourceFiles 是每个 E2E 项目过滤后目标语言源码文件数的最低要求。
const minSourceFiles = 100

// projectMetrics 汇总单个项目的 E2E 解析指标。
type projectMetrics struct {
	// files 是纳入解析的文件数。
	files int
	// successTrees 是成功生成非空根节点的文件数。
	successTrees int
	// diagnosticFiles 是产生至少一条诊断的文件数。
	diagnosticFiles int
	// systemFailures 是发生系统失败（错误、空树或 panic）的文件数。
	systemFailures int
	// totalBytes 是纳入文件的总字节数。
	totalBytes int64
	// elapsed 是该项目全量解析总耗时。
	elapsed time.Duration
	// maxFileElapsed 是单文件最大解析耗时。
	maxFileElapsed time.Duration
	// diagnosticByExt 按扩展名统计产生诊断的文件数，用于定位误报来源。
	diagnosticByExt map[string]int
}

// TestE2EParseProjects 对锁定清单中的每个项目全量解析其目标语言源码，校验 submodule 一致性、
// 文件数下限与系统失败为零，并输出解析指标。submodule 未初始化时跳过并提示初始化命令。
func TestE2EParseProjects(t *testing.T) {
	lock := loadLock(t)
	if len(lock.Projects) == 0 {
		t.Fatal("锁定清单为空")
	}
	p := parser.New()
	for _, proj := range lock.Projects {
		proj := proj
		t.Run(proj.Language, func(t *testing.T) {
			if _, err := os.Stat(proj.Path); os.IsNotExist(err) {
				t.Skipf("submodule %s 未初始化，请运行：git submodule update --init --recursive 01-parser/test/e2e/fixture/projects", proj.Path)
			}

			if got := headCommit(t, proj.Path); got != proj.Commit {
				t.Fatalf("submodule %s HEAD=%s 与锁定 commit=%s 不一致", proj.Path, got, proj.Commit)
			}

			files := trackedSourceFiles(t, proj.Path, proj.Extensions, proj.ExcludedPaths)
			if len(files) < minSourceFiles {
				t.Fatalf("项目 %s 目标语言源码文件数 %d 少于下限 %d", proj.Repository, len(files), minSourceFiles)
			}

			m := parseAll(t, p, files)
			t.Logf("项目=%s 文件数=%d 成功树=%d 含ERROR或MISSING节点的文件数=%d 系统失败=%d 总字节=%d 总耗时=%s 最大单文件=%s",
				proj.Repository, m.files, m.successTrees, m.diagnosticFiles, m.systemFailures,
				m.totalBytes, m.elapsed, m.maxFileElapsed)
			t.Logf("项目=%s 含ERROR或MISSING节点的文件数按扩展名: %s", proj.Repository, formatExtCounts(m.diagnosticByExt))

			// Parser 作为中间步骤的正确性红线：不得有任何系统失败，且每个纳入文件都必须生成语法树。
			if m.systemFailures != 0 {
				t.Errorf("项目 %s 存在 %d 个系统失败，Parser 要求系统失败为零", proj.Repository, m.systemFailures)
			}
			if m.successTrees != m.files {
				t.Errorf("项目 %s 有 %d 个文件未生成语法树（成功树 %d / 文件 %d）",
					proj.Repository, m.files-m.successTrees, m.successTrees, m.files)
			}
		})
	}
}

// parseAll 全量解析给定文件集合并汇总指标，对单文件 panic 做恢复并计入系统失败，绝不中断整体。
func parseAll(t *testing.T, p parser.Parser, files []string) projectMetrics {
	t.Helper()
	m := projectMetrics{files: len(files), diagnosticByExt: map[string]int{}}
	start := time.Now()
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取源码文件失败 %s: %v", path, err)
		}
		m.totalBytes += int64(len(content))
		if ok, diag, dur := parseOne(p, path, content); ok {
			m.successTrees++
			if diag {
				m.diagnosticFiles++
				m.diagnosticByExt[strings.ToLower(filepath.Ext(path))]++
			}
			if dur > m.maxFileElapsed {
				m.maxFileElapsed = dur
			}
		} else {
			m.systemFailures++
		}
	}
	m.elapsed = time.Since(start)
	return m
}

// formatExtCounts 把按扩展名的诊断计数格式化为稳定排序的字符串。
func formatExtCounts(m map[string]int) string {
	exts := make([]string, 0, len(m))
	for e := range m {
		exts = append(exts, e)
	}
	sort.Strings(exts)
	parts := make([]string, 0, len(exts))
	for _, e := range exts {
		parts = append(parts, e+"="+strconv.Itoa(m[e]))
	}
	return strings.Join(parts, " ")
}

// parseOne 解析单个文件，返回是否成功、是否有诊断、耗时；对 panic 与系统错误统一记为失败。
func parseOne(p parser.Parser, path string, content []byte) (ok bool, hasDiag bool, dur time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	file := parser.SourceFile{ID: 1, Path: path, Content: content}
	start := time.Now()
	res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
	dur = time.Since(start)
	if err != nil {
		return false, false, dur
	}
	defer res.Tree.Close()
	if _, kerr := res.Tree.Root().Kind(); kerr != nil {
		return false, false, dur
	}
	return true, len(res.Diagnostics) > 0, dur
}
