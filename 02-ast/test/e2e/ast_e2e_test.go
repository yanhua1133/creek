//go:build e2e

// Package e2e 使用项目根目录共享的固定开源项目语料验证 Common AST。
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
	"testing"
	"time"

	parser "creek/01-parser"
	commonast "creek/02-ast"
	"creek/internal/e2etest"
)

// astExpected 描述 Common AST 对单个共享项目的明确预期。
type astExpected struct {
	// TestPurpose 直接说明该 expected 验证的功能。
	TestPurpose string `json:"test_purpose"`
	// ProjectID 指向共享语料中的项目标识。
	ProjectID string `json:"project_id"`
	// Assertions 保存必须逐项满足的 Common AST 结果。
	Assertions astAssertions `json:"assertions"`
}

// astAssertions 保存 Common AST E2E 的稳定断言字段。
type astAssertions struct {
	// SourceFiles 是必须纳入转换的源码文件数。
	SourceFiles int `json:"source_files"`
	// ConvertedFiles 是必须成功生成 Common AST 的文件数。
	ConvertedFiles int `json:"converted_files"`
	// RequiredRootKind 是每棵 Common AST 必须使用的根节点类型。
	RequiredRootKind string `json:"required_root_kind"`
	// WrongRootFiles 是根节点类型不符合要求的文件数。
	WrongRootFiles int `json:"wrong_root_files"`
	// ParserDiagnosticFiles 是上游 Parser 产生诊断的文件数。
	ParserDiagnosticFiles int `json:"parser_diagnostic_files"`
	// ASTDiagnosticFiles 是 Common AST 产生诊断的文件数。
	ASTDiagnosticFiles int `json:"ast_diagnostic_files"`
	// UnmappedNodes 是尚未映射到核心节点或明确扩展的节点数。
	UnmappedNodes int `json:"unmapped_nodes"`
	// SystemFailures 是允许的解析或转换系统失败数。
	SystemFailures int `json:"system_failures"`
	// TotalBytes 是固定语料纳入文件的总字节数。
	TotalBytes int64 `json:"total_bytes"`
	// CommonASTSHA256 是全部规范化 Common AST 快照的稳定摘要。
	CommonASTSHA256 string `json:"common_ast_sha256"`
	// DiagnosticsSHA256 是全部 Common AST 诊断的稳定摘要。
	DiagnosticsSHA256 string `json:"diagnostics_sha256"`
}

// projectMetrics 保存单个项目的稳定断言结果和性能数据。
type projectMetrics struct {
	// Assertions 是必须与 expected 精确一致的稳定结果。
	Assertions astAssertions
	// Elapsed 是项目全量转换耗时，仅用于性能观察。
	Elapsed time.Duration
}

// TestE2ECommonASTProjects 使用共享项目语料验证 Common AST，并逐字段比较模块自己的 expected。
func TestE2ECommonASTProjects(t *testing.T) {
	manifest, repositoryRoot, err := e2etest.LoadManifest()
	if err != nil {
		t.Fatalf("读取共享 E2E 语料失败: %v", err)
	}
	if len(manifest.Projects) != 5 {
		t.Fatalf("共享 E2E 项目数 = %d，期望 5", len(manifest.Projects))
	}
	for _, project := range manifest.Projects {
		project := project
		t.Run(project.ID, func(t *testing.T) {
			expected := loadASTExpected(t, project.ID)
			if err := e2etest.VerifyProject(repositoryRoot, project); err != nil {
				t.Fatal(err)
			}
			repositoryDirectory := e2etest.ProjectDirectory(repositoryRoot, project)
			files, err := e2etest.TrackedSourceFiles(repositoryRoot, project)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != project.SourceFileCount {
				t.Fatalf("共享项目 %s 源码文件数=%d，清单值=%d", project.Repository, len(files), project.SourceFileCount)
			}
			metrics := convertProject(t, repositoryDirectory, files, expected.Assertions.RequiredRootKind)
			assertASTExpected(t, project, expected, metrics.Assertions)
			t.Logf("测试目的=%s 项目=%s 文件=%d AST成功=%d 错误根节点=%d Parser诊断文件=%d AST诊断文件=%d 未映射节点=%d 系统失败=%d 总字节=%d 耗时=%s AST摘要=%s 诊断摘要=%s",
				expected.TestPurpose, project.Repository, metrics.Assertions.SourceFiles, metrics.Assertions.ConvertedFiles,
				metrics.Assertions.WrongRootFiles, metrics.Assertions.ParserDiagnosticFiles, metrics.Assertions.ASTDiagnosticFiles,
				metrics.Assertions.UnmappedNodes, metrics.Assertions.SystemFailures, metrics.Assertions.TotalBytes,
				metrics.Elapsed, metrics.Assertions.CommonASTSHA256, metrics.Assertions.DiagnosticsSHA256)
		})
	}
}

// loadASTExpected 读取指定共享项目对应的 Common AST expected。
func loadASTExpected(t *testing.T, projectID string) astExpected {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("fixture", "expected", projectID+".json"))
	if err != nil {
		t.Fatalf("读取 Common AST E2E expected 失败 %s: %v", projectID, err)
	}
	var expected astExpected
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatalf("解析 Common AST E2E expected 失败 %s: %v", projectID, err)
	}
	if expected.ProjectID != projectID || expected.TestPurpose == "" || expected.Assertions.RequiredRootKind == "" {
		t.Fatalf("Common AST E2E expected 缺少测试目的、项目标识或根节点要求 %s", projectID)
	}
	return expected
}

// convertProject 解析并转换全部源码，生成计数、诊断和完整 AST 摘要。
func convertProject(t *testing.T, repositoryDirectory string, files []string, requiredRootKind string) projectMetrics {
	t.Helper()
	astHasher := sha256.New()
	diagnosticHasher := sha256.New()
	metrics := projectMetrics{Assertions: astAssertions{SourceFiles: len(files), RequiredRootKind: requiredRootKind}}
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
		writeHashString(astHasher, filepath.ToSlash(relative))
		writeHashString(diagnosticHasher, filepath.ToSlash(relative))
		parsed, parseErr := parser.New().Parse(context.Background(), parser.SourceFile{ID: parser.FileID(index + 1), Path: path, Content: content}, parser.ParseOptions{CollectDiagnostics: true})
		if parseErr != nil {
			metrics.Assertions.SystemFailures++
			writeHashString(astHasher, "Parser系统失败:"+parseErr.Error())
			continue
		}
		if len(parsed.Diagnostics) != 0 {
			metrics.Assertions.ParserDiagnosticFiles++
		}
		tree, diagnostics, convertErr := commonast.New().Convert(context.Background(), parsed.Tree)
		parsed.Tree.Close()
		if convertErr != nil || tree == nil {
			metrics.Assertions.SystemFailures++
			writeHashString(astHasher, fmt.Sprintf("AST系统失败:%v", convertErr))
			continue
		}
		metrics.Assertions.ConvertedFiles++
		if tree.Root().Kind().String() != requiredRootKind {
			metrics.Assertions.WrongRootFiles++
		}
		snapshot, err := commonast.MarshalSnapshot(tree)
		if err != nil {
			metrics.Assertions.SystemFailures++
			writeHashString(astHasher, "AST快照失败:"+err.Error())
			continue
		}
		writeHashBytes(astHasher, snapshot)
		hashASTDiagnostics(diagnosticHasher, diagnostics)
		if len(diagnostics) != 0 {
			metrics.Assertions.ASTDiagnosticFiles++
			for _, diagnostic := range diagnostics {
				if diagnostic.Kind == commonast.DiagnosticUnmappedSyntax {
					metrics.Assertions.UnmappedNodes++
				}
			}
		}
	}
	metrics.Elapsed = time.Since(started)
	metrics.Assertions.CommonASTSHA256 = hex.EncodeToString(astHasher.Sum(nil))
	metrics.Assertions.DiagnosticsSHA256 = hex.EncodeToString(diagnosticHasher.Sum(nil))
	return metrics
}

// hashASTDiagnostics 按稳定顺序写入 Common AST 诊断。
func hashASTDiagnostics(hasher hash.Hash, diagnostics []commonast.Diagnostic) {
	writeHashString(hasher, fmt.Sprintf("count:%d", len(diagnostics)))
	for _, diagnostic := range diagnostics {
		writeHashString(hasher, fmt.Sprintf("%d|%d|%d|%d|%d|%d|%d", diagnostic.Kind,
			diagnostic.Range.StartByte, diagnostic.Range.EndByte,
			diagnostic.Range.StartPoint.Row, diagnostic.Range.StartPoint.Column,
			diagnostic.Range.EndPoint.Row, diagnostic.Range.EndPoint.Column))
		writeHashString(hasher, diagnostic.CSTKind)
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

// assertASTExpected 逐字段比较 Common AST 实际结果与独立 expected。
func assertASTExpected(t *testing.T, project e2etest.Project, expected astExpected, actual astAssertions) {
	t.Helper()
	if expected.Assertions.SourceFiles != project.SourceFileCount {
		t.Fatalf("Common AST expected 的源码文件数与共享清单不一致 %s: expected=%d manifest=%d", project.ID, expected.Assertions.SourceFiles, project.SourceFileCount)
	}
	if reflect.DeepEqual(actual, expected.Assertions) {
		return
	}
	actualJSON, _ := json.MarshalIndent(actual, "", "  ")
	expectedJSON, _ := json.MarshalIndent(expected.Assertions, "", "  ")
	t.Fatalf("Common AST E2E 结果与 expected 不一致 %s\n=== 实际 ===\n%s\n=== 期望 ===\n%s", project.ID, actualJSON, expectedJSON)
}
