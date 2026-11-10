//go:build e2e

package e2e

import (
	"bytes"
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
			index, indexFiles := buildProjectMacroIndex(t, repositoryRoot, project)
			metrics := parseAll(t, parserInstance, repositoryDirectory, files, index)
			metrics.Assertions.MacroIndexFiles = indexFiles
			if metrics.Assertions.NewDiagnosticFiles != 0 || metrics.Assertions.NewDiagnosticByMacroFiles != 0 {
				t.Errorf("预处理使原本无诊断的文件新增诊断: 相对原样=%d 相对分支选择=%d",
					metrics.Assertions.NewDiagnosticFiles, metrics.Assertions.NewDiagnosticByMacroFiles)
			}
			logErrorRates(t, metrics.Assertions)
			assertParserExpected(t, project, expected, metrics.Assertions)
			t.Logf("测试目的=%s 项目=%s 文件=%d 成功树=%d 诊断文件=%d 系统失败=%d 总字节=%d 耗时=%s 最大单文件=%s 语法树摘要=%s 诊断摘要=%s",
				expected.TestPurpose, project.Repository, metrics.Assertions.SourceFiles, metrics.Assertions.SuccessfulTrees,
				metrics.Assertions.DiagnosticFiles, metrics.Assertions.SystemFailures, metrics.Assertions.TotalBytes,
				metrics.Elapsed, metrics.MaxFileElapsed, metrics.Assertions.SyntaxTreeSHA256, metrics.Assertions.DiagnosticsSHA256)
		})
	}
}

// parseAll 全量解析文件并生成计数、诊断分布、结构化摘要，以及按原样、只做分支选择、完整预处理三种方式的错误规模。
func parseAll(t *testing.T, parserInstance parser.Parser, repositoryDirectory string, files []string, index *parser.MacroIndex) projectMetrics {
	t.Helper()
	treeHasher := sha256.New()
	diagnosticHasher := sha256.New()
	preprocessHasher := sha256.New()
	macroHasher := sha256.New()
	metrics := projectMetrics{Assertions: parserAssertions{
		SourceFiles:                len(files),
		DiagnosticFilesByExtension: map[string]int{},
	}}
	started := time.Now()
	for position, path := range files {
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
		writeHashString(macroHasher, relative)
		blank := blankLines(content)
		for _, isBlank := range blank {
			if !isBlank {
				metrics.Assertions.NonBlankLines++
			}
		}
		file := parser.SourceFile{ID: parser.FileID(position + 1), Path: relative, Content: content}
		full := parseAndHashOne(parserInstance, file, index, blank, treeHasher, diagnosticHasher)
		hashPreprocessReport(preprocessHasher, full.report)
		addPreprocessMetrics(&metrics.Assertions, full.report)
		hashMacroReport(macroHasher, full.macros)
		addMacroMetrics(&metrics.Assertions, full.macros)
		if full.elapsed > metrics.MaxFileElapsed {
			metrics.MaxFileElapsed = full.elapsed
		}
		if !full.success {
			metrics.Assertions.SystemFailures++
			continue
		}
		metrics.Assertions.SuccessfulTrees++
		raw, conditional := full.stats, full.stats
		if full.report != nil {
			raw = parseErrorStats(t, parserInstance, file, blank, parser.ParseOptions{CollectDiagnostics: true, RawSource: true})
			conditional = parseErrorStats(t, parserInstance, file, blank, parser.ParseOptions{CollectDiagnostics: true, ConditionalOnly: true})
		}
		addErrorStats(&metrics.Assertions.Raw, raw)
		addErrorStats(&metrics.Assertions.Conditional, conditional)
		addErrorStats(&metrics.Assertions.Full, full.stats)
		compareDiagnostics(t, relative, raw.Diagnostics, full.stats.Diagnostics,
			&metrics.Assertions.FixedByPreprocessFiles, &metrics.Assertions.NewDiagnosticFiles, &metrics.Assertions.IncreasedDiagnosticFiles, "预处理")
		compareDiagnostics(t, relative, conditional.Diagnostics, full.stats.Diagnostics,
			&metrics.Assertions.FixedByMacroFiles, &metrics.Assertions.NewDiagnosticByMacroFiles, &metrics.Assertions.IncreasedByMacroFiles, "宏中和")
		if full.stats.DiagnosticFiles != 0 {
			metrics.Assertions.DiagnosticFiles++
			metrics.Assertions.DiagnosticFilesByExtension[strings.ToLower(filepath.Ext(path))]++
		}
	}
	metrics.Elapsed = time.Since(started)
	metrics.Assertions.SyntaxTreeSHA256 = hex.EncodeToString(treeHasher.Sum(nil))
	metrics.Assertions.DiagnosticsSHA256 = hex.EncodeToString(diagnosticHasher.Sum(nil))
	metrics.Assertions.PreprocessSHA256 = hex.EncodeToString(preprocessHasher.Sum(nil))
	metrics.Assertions.MacroSHA256 = hex.EncodeToString(macroHasher.Sum(nil))
	return metrics
}

// buildProjectMacroIndex 读取共享项目全部受跟踪的宏定义文件（含排除目录）并建立宏定义索引，返回索引和文件数。
func buildProjectMacroIndex(t *testing.T, repositoryRoot string, project e2etest.Project) (*parser.MacroIndex, int) {
	t.Helper()
	relatives, err := e2etest.TrackedFiles(repositoryRoot, project)
	if err != nil {
		t.Fatal(err)
	}
	directory := e2etest.ProjectDirectory(repositoryRoot, project)
	var files []parser.SourceFile
	for _, relative := range relatives {
		if !parser.IsMacroDefinitionFile(relative) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("读取宏定义文件失败 %s: %v", relative, err)
		}
		files = append(files, parser.SourceFile{ID: parser.FileID(len(files) + 1), Path: relative, Content: content})
	}
	return parser.BuildMacroIndex(files), len(files)
}

// compareDiagnostics 比较同一文件在基准方式和完整预处理下的诊断条数（不是含错标记），分别计入消除、新增和增多计数。
func compareDiagnostics(t *testing.T, relative string, before, after int, fixed, added, increased *int, stage string) {
	t.Helper()
	switch {
	case before != 0 && after == 0:
		*fixed++
	case before == 0 && after != 0:
		*added++
		t.Logf("%s后新增诊断: %s", stage, relative)
	case before != 0 && after > before:
		*increased++
		t.Logf("%s后诊断增多: %s 之前=%d 之后=%d", stage, relative, before, after)
	}
}

// logErrorRates 输出三种解析方式的文件、行、节点错误率，仅用于观察，断言以 expected 中的计数为准。
func logErrorRates(t *testing.T, assertions parserAssertions) {
	t.Helper()
	for _, mode := range []struct {
		// name 是解析方式名称。
		name string
		// stats 是该方式的错误规模。
		stats errorStats
	}{{"原样", assertions.Raw}, {"分支选择", assertions.Conditional}, {"完整预处理", assertions.Full}} {
		t.Logf("错误率 %s: 文件=%d/%d(%.2f%%) 行=%d/%d(%.3f%%) 节点=%d/%d(%.3f%%)", mode.name,
			mode.stats.DiagnosticFiles, assertions.SourceFiles, percent(mode.stats.DiagnosticFiles, assertions.SourceFiles),
			mode.stats.ErrorLines, assertions.NonBlankLines, percent(mode.stats.ErrorLines, assertions.NonBlankLines),
			mode.stats.ErrorNodes, mode.stats.TotalNodes, percent(mode.stats.ErrorNodes, mode.stats.TotalNodes))
	}
}

// percent 返回 part 占 total 的百分比，total 为零时返回零。
func percent(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(part) / float64(total)
}

// addErrorStats 把单个文件的错误规模累加到项目统计。
func addErrorStats(total *errorStats, file errorStats) {
	total.DiagnosticFiles += file.DiagnosticFiles
	total.Diagnostics += file.Diagnostics
	total.ErrorLines += file.ErrorLines
	total.ErrorNodes += file.ErrorNodes
	total.TotalNodes += file.TotalNodes
}

// blankLines 按换行符拆分源码，返回每行是否只含空白；文件末尾换行符之后的空串不计为一行。
func blankLines(content []byte) []bool {
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	blank := make([]bool, len(lines))
	for i, line := range lines {
		blank[i] = len(bytes.TrimSpace(line)) == 0
	}
	return blank
}

// parseErrorStats 按指定方式解析单个文件并统计错误规模；系统失败直接终止测试。
func parseErrorStats(t *testing.T, parserInstance parser.Parser, file parser.SourceFile, blank []bool, options parser.ParseOptions) errorStats {
	t.Helper()
	result, err := parserInstance.Parse(context.Background(), file, options)
	if err != nil {
		t.Fatalf("对照解析失败 %s: %v", file.Path, err)
	}
	defer result.Tree.Close()
	stats, err := treeErrorStats(result.Tree.Root(), blank, len(result.Diagnostics))
	if err != nil {
		t.Fatalf("对照解析语法树访问失败 %s: %v", file.Path, err)
	}
	return stats
}

// treeErrorStats 遍历语法树，统计错误子树节点数、节点总数和错误非空行数；diagnostics 为该文件的诊断条数。
func treeErrorStats(root parser.Node, blank []bool, diagnostics int) (errorStats, error) {
	stats := errorStats{}
	stats.Diagnostics = diagnostics
	if diagnostics != 0 {
		stats.DiagnosticFiles = 1
	}
	badRows := map[uint]bool{}
	var walk func(node parser.Node, inError bool) error
	walk = func(node parser.Node, inError bool) error {
		isError, err := node.IsError()
		if err != nil {
			return err
		}
		isMissing, err := node.IsMissing()
		if err != nil {
			return err
		}
		stats.TotalNodes++
		if isError || isMissing {
			sourceRange, err := node.Range()
			if err != nil {
				return err
			}
			last := sourceRange.EndPoint.Row
			if last > sourceRange.StartPoint.Row && sourceRange.EndPoint.Column == 0 {
				last--
			}
			for row := sourceRange.StartPoint.Row; row <= last; row++ {
				badRows[row] = true
			}
		}
		inError = inError || isError || isMissing
		if inError {
			stats.ErrorNodes++
		}
		childCount, err := node.ChildCount()
		if err != nil {
			return err
		}
		for index := 0; index < childCount; index++ {
			child, ok, err := node.Child(index)
			if err != nil {
				return err
			}
			if ok {
				if err := walk(child, inError); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(root, false); err != nil {
		return errorStats{}, err
	}
	for row := range badRows {
		if int(row) < len(blank) && !blank[row] {
			stats.ErrorLines++
		}
	}
	return stats, nil
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

// addMacroMetrics 把单个文件的宏中和报告计入项目统计。
func addMacroMetrics(assertions *parserAssertions, report *parser.MacroReport) {
	if report == nil {
		return
	}
	assertions.MacroRewrites += len(report.Rewrites)
	assertions.MacroUnsupported += report.Unsupported
	assertions.MacroTooLong += report.TooLong
}

// hashMacroReport 按固定字段顺序写入单个文件的宏中和报告。
func hashMacroReport(hasher hash.Hash, report *parser.MacroReport) {
	if report == nil {
		writeHashString(hasher, "nil")
		return
	}
	writeHashString(hasher, fmt.Sprintf("rewrites:%d|unsupported:%d|too_long:%d", len(report.Rewrites), report.Unsupported, report.TooLong))
	for _, rewrite := range report.Rewrites {
		writeHashString(hasher, fmt.Sprintf("%d|%d|%s|%s|%s|%d", rewrite.Range.StartByte, rewrite.Range.EndByte,
			rewrite.Name, rewrite.Replacement, rewrite.DefinitionPath, rewrite.DefinitionLine))
	}
}

// fullParse 是单个文件按默认方式完整预处理后解析的结果。
type fullParse struct {
	// success 表示是否成功生成并完整访问了语法树。
	success bool
	// stats 是该文件的错误规模。
	stats errorStats
	// report 是条件编译报告，非 C/C++ 文件为空。
	report *parser.PreprocessReport
	// macros 是宏中和报告，非 C/C++ 文件为空。
	macros *parser.MacroReport
	// elapsed 是解析耗时。
	elapsed time.Duration
}

// parseAndHashOne 按默认方式（带宏定义索引）解析单个文件，把规范化语法树和诊断写入项目摘要并统计错误规模。
func parseAndHashOne(parserInstance parser.Parser, file parser.SourceFile, index *parser.MacroIndex, blank []bool, treeHasher hash.Hash, diagnosticHasher hash.Hash) (out fullParse) {
	defer func() {
		if recover() != nil {
			out.success = false
		}
	}()
	started := time.Now()
	result, err := parserInstance.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true, Macros: index})
	out.elapsed = time.Since(started)
	if err != nil {
		writeHashString(treeHasher, "系统失败:"+err.Error())
		return out
	}
	defer result.Tree.Close()
	out.report, out.macros = result.Preprocess, result.Macros
	if err := hashSyntaxNode(treeHasher, result.Tree.Root()); err != nil {
		writeHashString(treeHasher, "语法树访问失败:"+err.Error())
		return out
	}
	hashParserDiagnostics(diagnosticHasher, result.Diagnostics)
	stats, err := treeErrorStats(result.Tree.Root(), blank, len(result.Diagnostics))
	if err != nil {
		return out
	}
	out.stats = stats
	out.success = true
	return out
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
