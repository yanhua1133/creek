package parser

import (
	"sort"
	"sync"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// syntaxTree 包装底层 Tree-sitter 语法树，独占其所有权并统一管理生命周期与并发访问。
type syntaxTree struct {
	// mu 串行化对底层树的所有只读访问与关闭操作，保证并发安全且防止关闭后访问已释放内存。
	mu sync.Mutex
	// lang 是该语法树对应的编程语言。
	lang Language
	// fileID 是该语法树对应的文件编号。
	fileID FileID
	// path 是解析时传入的项目内相对路径。
	path string
	// content 是解析所用的源码内容，在树存活期间保持不可变，供节点取源码切片。
	content []byte
	// view 是实际交给 Tree-sitter 的解析视图，与 content 等长，只用于取宏改写范围内的节点文本。
	view []byte
	// rewrites 是按源码顺序排列、互不重叠的宏改写，完全落在其中的节点取 view 中的文本。
	rewrites []MacroRewrite
	// tree 是底层 Tree-sitter 树，关闭后置为 nil。
	tree *ts.Tree
	// closed 标记该语法树是否已关闭。
	closed bool
}

// Language 返回该语法树对应的编程语言。
func (t *syntaxTree) Language() Language {
	return t.lang
}

// FileID 返回该语法树对应的文件编号。
func (t *syntaxTree) FileID() FileID {
	return t.fileID
}

// Path 返回解析时传入的项目内相对路径。
func (t *syntaxTree) Path() string {
	return t.path
}

// Root 返回语法树的根节点；若语法树已关闭，返回的节点在访问时统一报告 ErrTreeClosed。
func (t *syntaxTree) Root() Node {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.tree == nil {
		return &node{owner: t, raw: nil}
	}
	return &node{owner: t, raw: t.tree.RootNode()}
}

// Close 释放底层树资源，重复调用安全并返回稳定结果。
func (t *syntaxTree) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	if t.tree != nil {
		t.tree.Close()
		t.tree = nil
	}
	t.closed = true
	return nil
}

// textSource 返回节点 [start,end) 取文本所用的字节：完全落在一次宏改写范围内时为解析视图，否则为原始源码。
func (t *syntaxTree) textSource(start, end uint) []byte {
	i := sort.Search(len(t.rewrites), func(i int) bool { return t.rewrites[i].Range.StartByte > start }) - 1
	if i >= 0 && end <= t.rewrites[i].Range.EndByte && len(t.view) == len(t.content) {
		return t.view
	}
	return t.content
}
