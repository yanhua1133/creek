//go:build e2e

// Package e2e 使用项目根目录共享的固定开源项目语料验证 Parser 的端到端行为。
package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// parserExpected 描述 Parser 对单个共享项目的明确预期。
type parserExpected struct {
	// TestPurpose 直接说明该 expected 验证的功能。
	TestPurpose string `json:"test_purpose"`
	// ProjectID 指向共享语料中的项目标识。
	ProjectID string `json:"project_id"`
	// Assertions 保存必须逐项满足的 Parser 结果。
	Assertions parserAssertions `json:"assertions"`
}

// parserAssertions 保存 Parser E2E 的稳定断言字段。
type parserAssertions struct {
	// SourceFiles 是必须纳入解析的源码文件数。
	SourceFiles int `json:"source_files"`
	// SuccessfulTrees 是必须成功生成的语法树数量。
	SuccessfulTrees int `json:"successful_trees"`
	// SystemFailures 是允许的系统失败数量。
	SystemFailures int `json:"system_failures"`
	// TotalBytes 是固定语料纳入文件的总字节数。
	TotalBytes int64 `json:"total_bytes"`
	// DiagnosticFiles 是包含 ERROR 或 MISSING 诊断的文件数。
	DiagnosticFiles int `json:"diagnostic_files"`
	// DiagnosticFilesByExtension 按扩展名锁定诊断文件数。
	DiagnosticFilesByExtension map[string]int `json:"diagnostic_files_by_extension"`
	// SyntaxTreeSHA256 是全部规范化语法树的稳定摘要。
	SyntaxTreeSHA256 string `json:"syntax_tree_sha256"`
	// DiagnosticsSHA256 是全部规范化诊断的稳定摘要。
	DiagnosticsSHA256 string `json:"diagnostics_sha256"`
	// PreprocessBranches 是全部 C/C++ 文件条件编译分支总数。
	PreprocessBranches int `json:"preprocess_branches"`
	// PreprocessTakenBranches 是保留在解析视图中的条件分支数。
	PreprocessTakenBranches int `json:"preprocess_taken_branches"`
	// PreprocessUnknownBranches 是条件求值结果为未知的分支数。
	PreprocessUnknownBranches int `json:"preprocess_unknown_branches"`
	// PreprocessBlankedBytes 是替换为空格、不参与解析的字节总数。
	PreprocessBlankedBytes int64 `json:"preprocess_blanked_bytes"`
	// PreprocessUnterminated 是缺少 #endif 的条件块总数。
	PreprocessUnterminated int `json:"preprocess_unterminated"`
	// PreprocessSHA256 是全部条件编译报告的稳定摘要。
	PreprocessSHA256 string `json:"preprocess_sha256"`
	// NewDiagnosticFiles 是按原样解析不含诊断、完整预处理后反而含诊断的文件数，必须为零。
	NewDiagnosticFiles int `json:"new_diagnostic_files"`
	// FixedByPreprocessFiles 是按原样解析含诊断、完整预处理后不含诊断的文件数。
	FixedByPreprocessFiles int `json:"fixed_by_preprocess_files"`
	// IncreasedDiagnosticFiles 是按原样解析已含诊断、完整预处理后诊断条数更多的文件数。
	IncreasedDiagnosticFiles int `json:"increased_diagnostic_files"`
	// MacroIndexFiles 是建立宏定义索引时读取的宏定义文件数，包含排除目录中的文件。
	MacroIndexFiles int `json:"macro_index_files"`
	// MacroRewrites 是全部 C/C++ 文件的宏改写总数。
	MacroRewrites int `json:"macro_rewrites"`
	// MacroUnsupported 是找到定义但展开不可改写的宏调用总数。
	MacroUnsupported int `json:"macro_unsupported"`
	// MacroTooLong 是展开可改写但关键字放不下的宏调用总数。
	MacroTooLong int `json:"macro_too_long"`
	// MacroSHA256 是全部宏中和报告的稳定摘要。
	MacroSHA256 string `json:"macro_sha256"`
	// FixedByMacroFiles 是只做分支选择时含诊断、宏中和后不含诊断的文件数。
	FixedByMacroFiles int `json:"fixed_by_macro_files"`
	// NewDiagnosticByMacroFiles 是只做分支选择时不含诊断、宏中和后反而含诊断的文件数，必须为零。
	NewDiagnosticByMacroFiles int `json:"new_diagnostic_by_macro_files"`
	// IncreasedByMacroFiles 是只做分支选择时已含诊断、宏中和后诊断条数更多的文件数。
	IncreasedByMacroFiles int `json:"increased_by_macro_files"`
	// NonBlankLines 是全部纳入文件的非空行数，是行错误率的分母。
	NonBlankLines int `json:"non_blank_lines"`
	// Raw 是按原样解析时的错误规模。
	Raw errorStats `json:"raw"`
	// Conditional 是只做条件编译分支选择时的错误规模。
	Conditional errorStats `json:"conditional"`
	// Full 是完整预处理（分支选择加宏中和）后的错误规模，即默认解析结果。
	Full errorStats `json:"full"`
}

// errorStats 汇总一种解析方式下的语法错误规模。
type errorStats struct {
	// DiagnosticFiles 是含 ERROR 或 MISSING 节点的文件数。
	DiagnosticFiles int `json:"diagnostic_files"`
	// ErrorLines 是被 ERROR 节点覆盖或含 MISSING 节点的非空行数。
	ErrorLines int `json:"error_lines"`
	// ErrorNodes 是位于 ERROR 或 MISSING 子树中的语法树节点数，含匿名节点。
	ErrorNodes int `json:"error_nodes"`
	// TotalNodes 是语法树节点总数，含匿名节点。
	TotalNodes int `json:"total_nodes"`
}

// resourceBaselineExpected 描述 Parser 多轮资源测试的目的、语料范围和硬性断言。
type resourceBaselineExpected struct {
	// TestPurpose 直接说明资源基线验证的功能。
	TestPurpose string `json:"test_purpose"`
	// ProjectIDs 按共享清单顺序列出必须覆盖的项目。
	ProjectIDs []string `json:"project_ids"`
	// Assertions 保存必须满足的资源退化边界。
	Assertions resourceBaselineAssertions `json:"assertions"`
}

// resourceBaselineAssertions 保存 Parser 多轮资源测试的固定断言。
type resourceBaselineAssertions struct {
	// Rounds 是必须连续执行的全量解析轮数。
	Rounds int `json:"rounds"`
	// SourceFilesPerRound 是每轮必须读取并解析的源码文件数。
	SourceFilesPerRound int `json:"source_files_per_round"`
	// SuccessfulTreesPerRound 是每轮必须成功生成并访问的语法树数量。
	SuccessfulTreesPerRound int `json:"successful_trees_per_round"`
	// SystemFailures 是全部轮次允许的系统失败总数。
	SystemFailures int `json:"system_failures"`
	// MaxElapsedRatio 是最长轮次与最短轮次允许的最大耗时比。
	MaxElapsedRatio float64 `json:"max_elapsed_ratio"`
	// MaxHeapSysBytes 是各轮结束时允许的最大 Go 堆保留字节数。
	MaxHeapSysBytes uint64 `json:"max_heap_sys_bytes"`
}

// loadParserExpected 读取指定共享项目对应的 Parser expected。
func loadParserExpected(t *testing.T, projectID string) parserExpected {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("fixture", "expected", projectID+".json"))
	if err != nil {
		t.Fatalf("读取 Parser E2E expected 失败 %s: %v", projectID, err)
	}
	var expected parserExpected
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatalf("解析 Parser E2E expected 失败 %s: %v", projectID, err)
	}
	if expected.ProjectID != projectID || expected.TestPurpose == "" {
		t.Fatalf("Parser E2E expected 缺少明确测试目的或项目标识 %s", projectID)
	}
	return expected
}

// loadResourceBaselineExpected 读取 Parser 多轮资源测试的独立 expected。
func loadResourceBaselineExpected(t *testing.T) resourceBaselineExpected {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("fixture", "expected", "resource-baseline.json"))
	if err != nil {
		t.Fatalf("读取 Parser 资源基线 expected 失败: %v", err)
	}
	var expected resourceBaselineExpected
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatalf("解析 Parser 资源基线 expected 失败: %v", err)
	}
	if expected.TestPurpose == "" || len(expected.ProjectIDs) == 0 || expected.Assertions.Rounds < 2 ||
		expected.Assertions.SourceFilesPerRound <= 0 || expected.Assertions.SuccessfulTreesPerRound <= 0 ||
		expected.Assertions.MaxElapsedRatio <= 1 || expected.Assertions.MaxHeapSysBytes == 0 {
		t.Fatal("Parser 资源基线 expected 缺少明确测试目的、项目范围或多轮断言")
	}
	return expected
}
