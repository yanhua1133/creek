package commonast

// CommonAST 表示单个源文件转换得到的统一抽象语法树。
type CommonAST interface {
	// Language 返回 AST 对应的编程语言。
	Language() Language
	// FileID 返回 AST 对应的文件编号，只在同一分析任务内唯一。
	FileID() FileID
	// Path 返回 AST 对应源码文件的项目内相对路径。
	Path() string
	// Root 返回 AST 根节点。
	Root() Node
	// NodeByID 返回指定标识的节点。
	NodeByID(id NodeID) (Node, bool)
	// NodesByCSTRange 返回由指定 CST 范围生成的 AST 节点。
	NodesByCSTRange(sourceRange SourceRange) []Node
}

// commonAST 是单个文件 Common AST 的不可变实现。
type commonAST struct {
	// language 是源码文件使用的编程语言。
	language Language
	// fileID 是源码文件在当前分析任务内的编号。
	fileID FileID
	// path 是源码文件的项目内相对路径。
	path string
	// root 是 AST 根节点。
	root *astNode
	// nodes 保存 NodeID 到节点的索引。
	nodes map[NodeID]*astNode
	// cstNodes 保存 CST 范围到 AST 节点的反向索引。
	cstNodes map[rangeKey][]*astNode
}

// rangeKey 是 SourceRange 的可比较索引形式。
type rangeKey struct {
	// start 是范围起始字节偏移。
	start uint
	// end 是范围结束字节偏移。
	end uint
}

// Language 返回 AST 对应的编程语言。
func (a *commonAST) Language() Language {
	return a.language
}

// FileID 返回 AST 对应的文件编号。
func (a *commonAST) FileID() FileID {
	return a.fileID
}

// Path 返回 AST 对应源码文件的项目内相对路径。
func (a *commonAST) Path() string {
	return a.path
}

// Root 返回 AST 根节点。
func (a *commonAST) Root() Node {
	return a.root
}

// NodeByID 返回指定标识的节点。
func (a *commonAST) NodeByID(id NodeID) (Node, bool) {
	node, ok := a.nodes[id]
	return node, ok
}

// NodesByCSTRange 返回由指定 CST 范围生成的 AST 节点。
func (a *commonAST) NodesByCSTRange(sourceRange SourceRange) []Node {
	nodes := a.cstNodes[rangeKey{start: sourceRange.StartByte, end: sourceRange.EndByte}]
	result := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node)
	}
	return result
}
