package parser

import (
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsc "github.com/tree-sitter/tree-sitter-c/bindings/go"
	tscpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tsjava "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tspython "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// Language 表示源码使用的编程语言。
type Language uint8

const (
	// LanguageC 表示 C 语言。
	LanguageC Language = iota + 1
	// LanguageCPP 表示 C++ 语言。
	LanguageCPP
	// LanguageJava 表示 Java 语言。
	LanguageJava
	// LanguageGo 表示 Go 语言。
	LanguageGo
	// LanguagePython 表示 Python 语言。
	LanguagePython
)

// String 返回语言的稳定短名称，用于诊断与测试。
func (l Language) String() string {
	switch l {
	case LanguageC:
		return "c"
	case LanguageCPP:
		return "cpp"
	case LanguageJava:
		return "java"
	case LanguageGo:
		return "go"
	case LanguagePython:
		return "python"
	default:
		return "unknown"
	}
}

// grammarMu 保护 grammar 缓存的并发访问。
var grammarMu sync.Mutex

// grammarCache 缓存每种语言已构建的 Tree-sitter grammar，grammar 一经构建即不可变可复用。
var grammarCache = map[Language]*ts.Language{}

// grammarFor 返回指定语言对应的 Tree-sitter grammar，按需惰性构建并缓存。
func grammarFor(lang Language) *ts.Language {
	grammarMu.Lock()
	defer grammarMu.Unlock()
	if g, ok := grammarCache[lang]; ok {
		return g
	}
	var ptr unsafe.Pointer
	switch lang {
	case LanguageC:
		ptr = tsc.Language()
	case LanguageCPP:
		ptr = tscpp.Language()
	case LanguageJava:
		ptr = tsjava.Language()
	case LanguageGo:
		ptr = tsgo.Language()
	case LanguagePython:
		ptr = tspython.Language()
	default:
		return nil
	}
	g := ts.NewLanguage(ptr)
	grammarCache[lang] = g
	return g
}

// headerExtensions 是必须同时用 C 与 C++ grammar 双解析的头文件扩展名集合。
var headerExtensions = map[string]bool{
	".h": true,
}

// singleLanguageExtensions 把明确对应单一语言的扩展名映射到该语言。
var singleLanguageExtensions = map[string]Language{
	".c":    LanguageC,
	".cc":   LanguageCPP,
	".cpp":  LanguageCPP,
	".cxx":  LanguageCPP,
	".c++":  LanguageCPP,
	".hh":   LanguageCPP,
	".hpp":  LanguageCPP,
	".hxx":  LanguageCPP,
	".ipp":  LanguageCPP,
	".tpp":  LanguageCPP,
	".java": LanguageJava,
	".go":   LanguageGo,
	".py":   LanguagePython,
	".pyi":  LanguagePython,
}

// candidateLanguages 根据文件路径的扩展名返回需要尝试解析的语言列表。
// .h 头文件返回 C 与 C++ 两个候选以便双解析择优；其他受支持扩展名返回唯一候选；
// 未知扩展名返回 ErrUnsupportedExtension，过程不读取任何编译信息或项目配置。
func candidateLanguages(path string) ([]Language, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if headerExtensions[ext] {
		return []Language{LanguageC, LanguageCPP}, nil
	}
	if lang, ok := singleLanguageExtensions[ext]; ok {
		return []Language{lang}, nil
	}
	return nil, ErrUnsupportedExtension
}
