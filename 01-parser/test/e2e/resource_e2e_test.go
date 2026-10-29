//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"testing"
	"time"

	parser "creek/01-parser"
	"creek/internal/e2etest"
)

// resourceRoundMetrics 保存一轮全量解析的正确性与资源数据。
type resourceRoundMetrics struct {
	// SourceFiles 是本轮读取并尝试解析的源码文件数。
	SourceFiles int
	// SuccessfulTrees 是本轮成功生成并访问的语法树数量。
	SuccessfulTrees int
	// SystemFailures 是本轮读取、解析、访问或关闭失败的文件数。
	SystemFailures int
	// Elapsed 是本轮全量解析的总耗时。
	Elapsed time.Duration
	// HeapSysBytes 是本轮结束并执行垃圾回收后 Go 堆保留的字节数。
	HeapSysBytes uint64
}

// resourceBaselineMetrics 汇总多轮全量解析的资源退化指标。
type resourceBaselineMetrics struct {
	// Rounds 保存每轮全量解析的独立指标。
	Rounds []resourceRoundMetrics
	// TotalSystemFailures 是全部轮次的系统失败总数。
	TotalSystemFailures int
	// ElapsedRatio 是最长轮次与最短轮次的耗时比。
	ElapsedRatio float64
	// PeakHeapSysBytes 是各轮结束时观测到的最大 Go 堆保留字节数。
	PeakHeapSysBytes uint64
}

// TestE2EResourceBaseline 连续多轮解析全部共享语料，验证 Parser 的资源退化边界。
func TestE2EResourceBaseline(t *testing.T) {
	expected := loadResourceBaselineExpected(t)
	actual := runParserResourceBaseline(t, expected)
	assertResourceBaseline(t, expected, actual)
}

// runParserResourceBaseline 校验并收集共享语料，然后连续执行指定轮数的全量解析。
func runParserResourceBaseline(t *testing.T, expected resourceBaselineExpected) resourceBaselineMetrics {
	t.Helper()
	manifest, repositoryRoot, err := e2etest.LoadManifest()
	if err != nil {
		t.Fatalf("读取共享 E2E 语料失败: %v", err)
	}
	projectIDs := make([]string, 0, len(manifest.Projects))
	allFiles := make([]string, 0, expected.Assertions.SourceFilesPerRound)
	for _, project := range manifest.Projects {
		projectIDs = append(projectIDs, project.ID)
		if err := e2etest.VerifyProject(repositoryRoot, project); err != nil {
			t.Fatal(err)
		}
		files, err := e2etest.TrackedSourceFiles(repositoryRoot, project)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != project.SourceFileCount {
			t.Fatalf("共享项目 %s 源码文件数=%d，清单值=%d", project.Repository, len(files), project.SourceFileCount)
		}
		allFiles = append(allFiles, files...)
	}
	if !reflect.DeepEqual(projectIDs, expected.ProjectIDs) {
		t.Fatalf("资源基线覆盖项目与 expected 不一致: 实际=%v 期望=%v", projectIDs, expected.ProjectIDs)
	}
	if len(allFiles) != expected.Assertions.SourceFilesPerRound {
		t.Fatalf("资源基线每轮源码文件数=%d，期望=%d", len(allFiles), expected.Assertions.SourceFilesPerRound)
	}

	parserInstance := parser.New()
	actual := resourceBaselineMetrics{Rounds: make([]resourceRoundMetrics, 0, expected.Assertions.Rounds)}
	for round := 1; round <= expected.Assertions.Rounds; round++ {
		runtime.GC()
		metrics := runParserResourceRound(t, parserInstance, allFiles)
		runtime.GC()
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		metrics.HeapSysBytes = memory.HeapSys
		actual.Rounds = append(actual.Rounds, metrics)
		actual.TotalSystemFailures += metrics.SystemFailures
		if metrics.HeapSysBytes > actual.PeakHeapSysBytes {
			actual.PeakHeapSysBytes = metrics.HeapSysBytes
		}
		t.Logf("资源轮次=%d 文件=%d 成功树=%d 系统失败=%d 耗时=%s HeapSys=%d", round, metrics.SourceFiles,
			metrics.SuccessfulTrees, metrics.SystemFailures, metrics.Elapsed, metrics.HeapSysBytes)
	}
	actual.ElapsedRatio = calculateElapsedRatio(t, actual.Rounds)
	return actual
}

// runParserResourceRound 执行一轮全量解析，并保证每棵语法树都被访问和关闭。
func runParserResourceRound(t *testing.T, parserInstance parser.Parser, files []string) resourceRoundMetrics {
	t.Helper()
	metrics := resourceRoundMetrics{SourceFiles: len(files)}
	started := time.Now()
	for index, path := range files {
		if err := parseResourceFile(parserInstance, parser.FileID(index+1), path); err != nil {
			metrics.SystemFailures++
			if metrics.SystemFailures <= 20 {
				t.Logf("资源基线解析失败 %s: %v", path, err)
			}
			continue
		}
		metrics.SuccessfulTrees++
	}
	metrics.Elapsed = time.Since(started)
	return metrics
}

// parseResourceFile 解析一个共享源码文件，访问根节点后显式关闭语法树。
func parseResourceFile(parserInstance parser.Parser, fileID parser.FileID, path string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("发生 panic: %v", recovered)
		}
	}()
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取源码失败: %w", err)
	}
	result, err := parserInstance.Parse(context.Background(), parser.SourceFile{ID: fileID, Path: path, Content: content}, parser.ParseOptions{CollectDiagnostics: false})
	if err != nil {
		return fmt.Errorf("解析源码失败: %w", err)
	}
	if result.Tree == nil {
		return fmt.Errorf("解析成功但语法树为空")
	}
	defer func() {
		if closeErr := result.Tree.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("关闭语法树失败: %w", closeErr)
		}
	}()
	rootKind, err := result.Tree.Root().Kind()
	if err != nil {
		return fmt.Errorf("访问语法树根节点失败: %w", err)
	}
	if rootKind == "" {
		return fmt.Errorf("语法树根节点类型为空")
	}
	return nil
}

// calculateElapsedRatio 计算最长轮次与最短轮次的耗时比。
func calculateElapsedRatio(t *testing.T, rounds []resourceRoundMetrics) float64 {
	t.Helper()
	if len(rounds) < 2 {
		t.Fatal("资源基线至少需要两轮数据")
	}
	minimum := rounds[0].Elapsed
	maximum := rounds[0].Elapsed
	for _, round := range rounds[1:] {
		if round.Elapsed < minimum {
			minimum = round.Elapsed
		}
		if round.Elapsed > maximum {
			maximum = round.Elapsed
		}
	}
	if minimum <= 0 {
		t.Fatalf("资源基线最短轮次耗时无效: %s", minimum)
	}
	return float64(maximum) / float64(minimum)
}

// assertResourceBaseline 逐项验证多轮解析的正确性、耗时波动和 Go 堆上限。
func assertResourceBaseline(t *testing.T, expected resourceBaselineExpected, actual resourceBaselineMetrics) {
	t.Helper()
	if len(actual.Rounds) != expected.Assertions.Rounds {
		t.Fatalf("资源基线实际轮数=%d，期望=%d", len(actual.Rounds), expected.Assertions.Rounds)
	}
	for index, round := range actual.Rounds {
		if round.SourceFiles != expected.Assertions.SourceFilesPerRound {
			t.Errorf("资源基线第 %d 轮源码文件数=%d，期望=%d", index+1, round.SourceFiles, expected.Assertions.SourceFilesPerRound)
		}
		if round.SuccessfulTrees != expected.Assertions.SuccessfulTreesPerRound {
			t.Errorf("资源基线第 %d 轮成功语法树数=%d，期望=%d", index+1, round.SuccessfulTrees, expected.Assertions.SuccessfulTreesPerRound)
		}
	}
	if actual.TotalSystemFailures != expected.Assertions.SystemFailures {
		t.Errorf("资源基线系统失败总数=%d，期望=%d", actual.TotalSystemFailures, expected.Assertions.SystemFailures)
	}
	if actual.ElapsedRatio >= expected.Assertions.MaxElapsedRatio {
		t.Errorf("资源基线耗时波动=%.2fx，要求小于 %.2fx", actual.ElapsedRatio, expected.Assertions.MaxElapsedRatio)
	}
	if actual.PeakHeapSysBytes >= expected.Assertions.MaxHeapSysBytes {
		t.Errorf("资源基线峰值 HeapSys=%d，要求小于 %d", actual.PeakHeapSysBytes, expected.Assertions.MaxHeapSysBytes)
	}
	t.Logf("测试目的=%s 轮数=%d 每轮文件=%d 系统失败=%d 耗时波动=%.2fx 峰值HeapSys=%d",
		expected.TestPurpose, len(actual.Rounds), expected.Assertions.SourceFilesPerRound, actual.TotalSystemFailures,
		actual.ElapsedRatio, actual.PeakHeapSysBytes)
}
