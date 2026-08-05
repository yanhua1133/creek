//go:build e2e

// Package e2e 是 Parser 模块的端到端测试包，对固定 commit 的真实开源项目 submodule 做全量解析。
// 测试运行时不访问网络，全部输入来自已初始化的 submodule。使用 e2e build tag 隔离，默认不执行。
package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// supportedExts 是 Parser 支持解析的扩展名集合，E2E 只纳入其中的文件，避免对不支持扩展名产生系统失败。
var supportedExts = map[string]bool{
	".c": true, ".h": true, ".cc": true, ".cpp": true, ".cxx": true, ".c++": true,
	".hh": true, ".hpp": true, ".hxx": true, ".ipp": true, ".tpp": true,
	".java": true, ".go": true, ".py": true, ".pyi": true,
}

// projectLock 对应 projects.lock.json 的顶层结构，锁定 E2E 使用的开源项目集合。
type projectLock struct {
	// VerifiedAt 是清单核验日期。
	VerifiedAt string `json:"verified_at"`
	// Projects 是锁定的项目条目列表。
	Projects []projectEntry `json:"projects"`
}

// projectEntry 描述一个被锁定的开源项目 submodule 及其校验信息。
type projectEntry struct {
	// Language 是项目主语言标识。
	Language string `json:"language"`
	// Path 是相对 E2E 测试目录的 submodule 路径。
	Path string `json:"path"`
	// Repository 是 GitHub 仓库标识。
	Repository string `json:"repository"`
	// Commit 是锁定的完整提交哈希。
	Commit string `json:"commit"`
	// StarsAtVerification 是核验时的 Star 数。
	StarsAtVerification int `json:"stars_at_verification"`
	// SourceFileCount 是过滤后目标语言源码文件数。
	SourceFileCount int `json:"source_file_count"`
	// Extensions 是计入统计的源码扩展名。
	Extensions []string `json:"extensions"`
	// ExcludedPaths 是排除的路径前缀及其原因由清单另行说明。
	ExcludedPaths []string `json:"excluded_paths"`
}

// loadLock 读取并解析 projects.lock.json。
func loadLock(t *testing.T) projectLock {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("fixture", "projects.lock.json"))
	if err != nil {
		t.Fatalf("读取锁定清单失败: %v", err)
	}
	var lock projectLock
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatalf("解析锁定清单失败: %v", err)
	}
	return lock
}

// headCommit 返回指定仓库目录当前 HEAD 的完整提交哈希。
func headCommit(t *testing.T, repoDir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("读取 %s HEAD 失败: %v", repoDir, err)
	}
	return strings.TrimSpace(string(out))
}

// trackedSourceFiles 用 git ls-files 收集仓库中受跟踪的目标语言源码文件（绝对路径），
// 仅纳入既在项目声明扩展名内、又被 Parser 支持的扩展名，并排除声明的路径前缀。
func trackedSourceFiles(t *testing.T, repoDir string, declaredExts []string, excluded []string) []string {
	t.Helper()
	extSet := map[string]bool{}
	for _, e := range declaredExts {
		e = strings.ToLower(e)
		if supportedExts[e] {
			extSet[e] = true
		}
	}
	out, err := exec.Command("git", "-C", repoDir, "ls-files").Output()
	if err != nil {
		t.Fatalf("git ls-files %s 失败: %v", repoDir, err)
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		rel := strings.TrimSpace(line)
		if rel == "" {
			continue
		}
		if !extSet[strings.ToLower(filepath.Ext(rel))] {
			continue
		}
		if isExcluded(rel, excluded) {
			continue
		}
		files = append(files, filepath.Join(repoDir, rel))
	}
	return files
}

// isExcluded 报告相对路径是否命中任一排除前缀。
func isExcluded(rel string, excluded []string) bool {
	for _, prefix := range excluded {
		if prefix != "" && strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}
