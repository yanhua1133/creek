// Package e2etest 提供项目内各模块共用的 E2E 语料读取与校验能力。
package e2etest

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest 描述项目根目录共享的 E2E 输入语料。
type Manifest struct {
	// VerifiedAt 是共享语料最后核验日期。
	VerifiedAt string `json:"verified_at"`
	// Projects 是共享项目条目。
	Projects []Project `json:"projects"`
}

// Project 描述一个固定提交的共享开源项目。
type Project struct {
	// ID 是跨模块稳定使用的项目标识。
	ID string `json:"id"`
	// Language 是项目在本次语料中的目标语言。
	Language string `json:"language"`
	// Path 是项目相对仓库根目录的路径。
	Path string `json:"path"`
	// Repository 是 GitHub 仓库名称。
	Repository string `json:"repository"`
	// Commit 是固定的完整提交哈希。
	Commit string `json:"commit"`
	// StarsAtVerification 是引入项目时核验的 Star 数。
	StarsAtVerification int `json:"stars_at_verification"`
	// SourceFileCount 是过滤后的源码文件数。
	SourceFileCount int `json:"source_file_count"`
	// Extensions 是纳入测试的源码扩展名。
	Extensions []string `json:"extensions"`
	// ExcludedPaths 是明确排除的路径前缀。
	ExcludedPaths []string `json:"excluded_paths"`
}

// RepositoryRoot 从当前工作目录向上查找包含共享语料清单的仓库根目录。
func RepositoryRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("读取当前工作目录失败: %w", err)
	}
	for {
		manifestPath := filepath.Join(current, "e2e-corpus", "projects.json")
		if _, err := os.Stat(manifestPath); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("未找到共享 E2E 语料清单")
		}
		current = parent
	}
}

// LoadManifest 读取项目根目录的共享 E2E 语料清单。
func LoadManifest() (Manifest, string, error) {
	root, err := RepositoryRoot()
	if err != nil {
		return Manifest{}, "", err
	}
	raw, err := os.ReadFile(filepath.Join(root, "e2e-corpus", "projects.json"))
	if err != nil {
		return Manifest{}, "", fmt.Errorf("读取共享 E2E 语料清单失败: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, "", fmt.Errorf("解析共享 E2E 语料清单失败: %w", err)
	}
	return manifest, root, nil
}

// ProjectDirectory 返回共享项目的绝对路径。
func ProjectDirectory(repositoryRoot string, project Project) string {
	return filepath.Join(repositoryRoot, filepath.FromSlash(project.Path))
}

// VerifyProject 校验共享项目存在且当前提交与清单一致。
func VerifyProject(repositoryRoot string, project Project) error {
	directory := ProjectDirectory(repositoryRoot, project)
	if _, err := os.Stat(directory); err != nil {
		return fmt.Errorf("共享 E2E 项目不存在 %s，请运行 git submodule update --init --recursive e2e-corpus/projects: %w", project.Path, err)
	}
	output, err := exec.Command("git", "-C", directory, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("读取共享项目提交失败 %s: %w", project.Repository, err)
	}
	actual := strings.TrimSpace(string(output))
	if actual != project.Commit {
		return fmt.Errorf("共享项目 %s HEAD=%s，与清单 commit=%s 不一致", project.Repository, actual, project.Commit)
	}
	return nil
}

// TrackedSourceFiles 返回共享项目中符合扩展名和排除规则的受跟踪源码文件。
func TrackedSourceFiles(repositoryRoot string, project Project) ([]string, error) {
	directory := ProjectDirectory(repositoryRoot, project)
	extensions := make(map[string]bool, len(project.Extensions))
	for _, extension := range project.Extensions {
		extensions[strings.ToLower(extension)] = true
	}
	output, err := exec.Command("git", "-C", directory, "ls-files").Output()
	if err != nil {
		return nil, fmt.Errorf("列出共享项目文件失败 %s: %w", project.Repository, err)
	}
	var files []string
	for _, line := range strings.Split(string(output), "\n") {
		relative := strings.TrimSpace(line)
		if relative == "" || !extensions[strings.ToLower(filepath.Ext(relative))] || IsExcluded(relative, project.ExcludedPaths) {
			continue
		}
		files = append(files, filepath.Join(directory, relative))
	}
	sort.Strings(files)
	return files, nil
}

// IsExcluded 报告相对路径是否命中任一排除前缀。
func IsExcluded(relative string, excludedPaths []string) bool {
	for _, prefix := range excludedPaths {
		if prefix != "" && strings.HasPrefix(relative, prefix) {
			return true
		}
	}
	return false
}
