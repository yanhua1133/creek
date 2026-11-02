//go:build e2e

package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	parser "creek/01-parser"
	"creek/internal/e2etest"
)

// minSourceFiles 是每个共享项目过滤后源码文件数的最低要求。
const minSourceFiles = 100

// projectMetrics 汇总单个项目的 Parser E2E 结果。
type projectMetrics struct {
	// Assertions 是必须与 expected 精确一致的稳定结果。
	Assertions parserAssertions
	// Elapsed 是项目全量解析总耗时，仅用于性能观察。
	Elapsed time.Duration
	// MaxFileElapsed 是单文件最大解析耗时，仅用于性能观察。
	MaxFileElapsed time.Duration
}

// TestE2EParseProjects 使用一份共享项目语料验证 Parser，并逐字段比较模块自己的 expected。
func TestE2EParseProjects(t *testing.T) {
	manifest, repositoryRoot, err := e2etest.LoadManifest()
	if err != nil {
		t.Fatalf("读取共享 E2E 语料失败: %v", err)
	}
	if len(manifest.Projects) != 5 {
		t.Fatalf("共享 E2E 项目数 = %d，期望 5", len(manifest.Projects))
	}
	parserInstance := parser.New()
	for _, project := range manifest.Projects {
		project := project
		t.Run(project.ID, func(t *testing.T) {
			expected := loadParserExpected(t, project.ID)
			if err := e2etest.VerifyProject(repositoryRoot, project); err != nil {
				t.Fatal(err)
			}
			repositoryDirectory := e2etest.ProjectDirectory(repositoryRoot, project)
			files, err := e2etest.TrackedSourceFiles(repositoryRoot, project)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) < minSourceFiles || len(files) != project.SourceFileCount {
				t.Fatalf("共享项目 %s 源码文件数=%d，清单值=%d，最低要求=%d", project.Repository, len(files), project.SourceFileCount, minSourceFiles)
			}
			metrics := parseAll(t, parserInstance, repositoryDirectory, files)
			assertParserExpected(t, project, expected, metrics.Assertions)
			t.Logf("测试目的=%s 项目=%s 文件=%d 成功树=%d 诊断文件=%d 系统失败=%d 总字节=%d 耗时=%s 最大单文件=%s 语法树摘要=%s 诊断摘要=%s",
				expected.TestPurpose, project.Repository, metrics.Assertions.SourceFiles, metrics.Assertions.SuccessfulTrees,
				metrics.Assertions.DiagnosticFiles, metrics.Assertions.SystemFailures, metrics.Assertions.TotalBytes,
				metrics.Elapsed, metrics.MaxFileElapsed, metrics.Assertions.SyntaxTreeSHA256, metrics.Assertions.DiagnosticsSHA256)
		})
	}
}

// parseAll 全量解析文件并生成计数、诊断分布和结构化摘要。
func parseAll(t *testing.T, parserInstance parser.Parser, repositoryDirectory string, files []string) projectMetrics {
	t.Helper()
	treeHasher := sha256.New()
	diagnosticHasher := sha256.New()
	preprocessHasher := sha256.New()
	metrics := projectMetrics{Assertions: parserAssertions{
		SourceFiles:                len(files),
		DiagnosticFilesByExtension: map[string]int{},
	}}
	started := time.Now()
	for index, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取共享源码失败 %s: %v", path, err)
		}
		metrics.Assertions.TotalBytes += int64(len(content))
		relative, err := filepath.Rel(repositoryDirectory, path)
		if err != nil {
			t.Fatalf("计算共享源码相对路径失败 %s: %v", path, err)
		}
		relative = filepath.ToSlash(relative)
		writeHashString(treeHasher, relative)
		writeHashString(diagnosticHasher, relative)
		writeHashString(preprocessHasher, relative)
		success, diagnostics, report, elapsed := parseAndHashOne(parserInstance, parser.FileID(index+1), relative, content, treeHasher, diagnosticHasher)
		hashPreprocessReport(preprocessHasher, report)
		addPreprocessMetrics(&metrics.Assertions, report)
		rawDiagnostics, rawErr := parseRawDiagnostics(parserInstance, parser.FileID(index+1), relative, content)
		if rawErr != nil {
			t.Fatalf("按原样解析失败 %s: %v", relative, rawErr)
		}
		if rawDiagnostics != 0 {
			metrics.Assertions.RawDiagnosticFiles++
		}
		if success && rawDiagnostics != 0 && diagnostics == 0 {
			metrics.Assertions.FixedByPreprocessFiles++
		}
		if success && rawDiagnostics == 0 && diagnostics != 0 {
			metrics.Assertions.NewDiagnosticFiles++
			t.Logf("分支选择后新增诊断: %s", relative)
		}
		if success && rawDiagnostics != 0 && diagnostics > rawDiagnostics {
			metrics.Assertions.IncreasedDiagnosticFiles++
			t.Logf("分支选择后诊断增多: %s 原样=%d 处理后=%d", relative, rawDiagnostics, diagnostics)
		}
		if elapsed > metrics.MaxFileElapsed {
			metrics.MaxFileElapsed = elapsed
		}
		if !success {
			metrics.Assertions.SystemFailures++
			continue
		}
		metrics.Assertions.SuccessfulTrees++
		if diagnostics != 0 {
			metrics.Assertions.DiagnosticFiles++
			metrics.Assertions.DiagnosticFilesByExtension[strings.ToLower(filepath.Ext(path))]++
		}
	}
	metrics.Elapsed = time.Since(started)
	metrics.Assertions.SyntaxTreeSHA256 = hex.EncodeToString(treeHasher.Sum(nil))
	metrics.Assertions.DiagnosticsSHA256 = hex.EncodeToString(diagnosticHasher.Sum(nil))
	metrics.Assertions.PreprocessSHA256 = hex.EncodeToString(preprocessHasher.Sum(nil))
	return metrics
}

// addPreprocessMetrics 把单个文件的条件编译报告计入项目统计。
func addPreprocessMetrics(assertions *parserAssertions, report *parser.PreprocessReport) {
	if report == nil {
		return
	}
	assertions.PreprocessBlankedBytes += int64(report.BlankedBytes)
	assertions.PreprocessUnterminated += report.Unterminated
	for _, branch := range report.Branches {
		assertions.PreprocessBranches++
		if branch.Taken {
			assertions.PreprocessTakenBranches++
		}
		if branch.Value == parser.ConditionUnknown {
			assertions.PreprocessUnknownBranches++
		}
	}
}

// hashPreprocessReport 按固定字段顺序写入单个文件的条件编译报告。
func hashPreprocessReport(hasher hash.Hash, report *parser.PreprocessReport) {
	if report == nil {
		writeHashString(hasher, "nil")
		return
	}
	writeHashString(hasher, fmt.Sprintf("blanked:%d|unterminated:%d|branches:%d", report.BlankedBytes, report.Unterminated, len(report.Branches)))
	for _, branch := range report.Branches {
		writeHashString(hasher, fmt.Sprintf("%d|%d|%s|%d|%t", branch.Directive.StartByte, branch.Directive.EndByte, branch.Keyword, branch.Value, branch.Taken))
		writeHashString(hasher, branch.Condition)
	}
}

// parseRawDiagnostics 按原样解析单个文件并返回诊断数，用于对照条件编译分支选择的效果。
func parseRawDiagnostics(parserInstance parser.Parser, fileID parser.FileID, path string, content []byte) (int, error) {
	result, err := parserInstance.Parse(context.Background(), parser.SourceFile{ID: fileID, Path: path, Content: content}, parser.ParseOptions{CollectDiagnostics: true, RawSource: true})
	if err != nil {
		return 0, err
	}
	defer result.Tree.Close()
	return len(result.Diagnostics), nil
}

// parseAndHashOne 解析单个文件并把规范化语法树和诊断写入项目摘要，path 为项目内相对路径。
func parseAndHashOne(parserInstance parser.Parser, fileID parser.FileID, path string, content []byte, treeHasher hash.Hash, diagnosticHasher hash.Hash) (success bool, diagnosticCount int, report *parser.PreprocessReport, elapsed time.Duration) {
	defer func() {
		if recover() != nil {
			success = false
		}
	}()
	started := time.Now()
	result, err := parserInstance.Parse(context.Background(), parser.SourceFile{ID: fileID, Path: path, Content: content}, parser.ParseOptions{CollectDiagnostics: true})
	elapsed = time.Since(started)
	if err != nil {
		writeHashString(treeHasher, "系统失败:"+err.Error())
		return false, 0, nil, elapsed
	}
	defer result.Tree.Close()
	if err := hashSyntaxNode(treeHasher, result.Tree.Root()); err != nil {
		writeHashString(treeHasher, "语法树访问失败:"+err.Error())
		return false, 0, result.Preprocess, elapsed
	}
	hashParserDiagnostics(diagnosticHasher, result.Diagnostics)
	return true, len(result.Diagnostics), result.Preprocess, elapsed
}

// hashSyntaxNode 按固定字段顺序递归写入完整 CST 节点结构。
func hashSyntaxNode(hasher hash.Hash, node parser.Node) error {
	kind, err := node.Kind()
	if err != nil {
		return err
	}
	named, err := node.IsNamed()
	if err != nil {
		return err
	}
	isError, err := node.IsError()
	if err != nil {
		return err
	}
	isMissing, err := node.IsMissing()
	if err != nil {
		return err
	}
	sourceRange, err := node.Range()
	if err != nil {
		return err
	}
	childCount, err := node.ChildCount()
	if err != nil {
		return err
	}
	writeHashString(hasher, kind)
	writeHashString(hasher, fmt.Sprintf("%t|%t|%t|%d|%d|%d", named, isError, isMissing, sourceRange.StartByte, sourceRange.EndByte, childCount))
	if childCount == 0 {
		text, err := node.Text()
		if err != nil {
			return err
		}
		writeHashBytes(hasher, text)
	}
	for index := 0; index < childCount; index++ {
		field, err := node.FieldNameForChild(index)
		if err != nil {
			return err
		}
		writeHashString(hasher, field)
		child, ok, err := node.Child(index)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("第 %d 个子节点不存在", index)
		}
		if err := hashSyntaxNode(hasher, child); err != nil {
			return err
		}
	}
	return nil
}

// hashParserDiagnostics 按稳定顺序写入 Parser 诊断。
func hashParserDiagnostics(hasher hash.Hash, diagnostics []parser.Diagnostic) {
	writeHashString(hasher, fmt.Sprintf("count:%d", len(diagnostics)))
	for _, diagnostic := range diagnostics {
		writeHashString(hasher, fmt.Sprintf("%d|%d|%d|%d|%d|%d|%d", diagnostic.Kind,
			diagnostic.Range.StartByte, diagnostic.Range.EndByte,
			diagnostic.Range.StartPoint.Row, diagnostic.Range.StartPoint.Column,
			diagnostic.Range.EndPoint.Row, diagnostic.Range.EndPoint.Column))
		writeHashString(hasher, diagnostic.NodeKind)
		writeHashString(hasher, diagnostic.Message)
	}
}

// writeHashString 使用长度前缀把字符串无歧义写入摘要。
func writeHashString(hasher hash.Hash, value string) {
	writeHashBytes(hasher, []byte(value))
}

// writeHashBytes 使用长度前缀把字节序列无歧义写入摘要。
func writeHashBytes(hasher hash.Hash, value []byte) {
	_, _ = fmt.Fprintf(hasher, "%d:", len(value))
	_, _ = hasher.Write(value)
}

// assertParserExpected 逐字段比较 Parser 实际结果与独立 expected。
func assertParserExpected(t *testing.T, project e2etest.Project, expected parserExpected, actual parserAssertions) {
	t.Helper()
	if expected.Assertions.SourceFiles != project.SourceFileCount {
		t.Fatalf("Parser expected 的源码文件数与共享清单不一致 %s: expected=%d manifest=%d", project.ID, expected.Assertions.SourceFiles, project.SourceFileCount)
	}
	if reflect.DeepEqual(actual, expected.Assertions) {
		return
	}
	actualJSON, _ := json.MarshalIndent(actual, "", "  ")
	expectedJSON, _ := json.MarshalIndent(expected.Assertions, "", "  ")
	t.Fatalf("Parser E2E 结果与 expected 不一致 %s\n=== 实际 ===\n%s\n=== 期望 ===\n%s", project.ID, actualJSON, expectedJSON)
}
