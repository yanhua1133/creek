package commonast

// Node 提供对 Common AST 节点的只读访问能力。
type Node interface {
	// ID 返回节点的稳定标识。
	ID() NodeID
	// Kind 返回节点的核心语法种类。
	Kind() NodeKind
	// Range 返回节点覆盖的源码范围。
	Range() SourceRange
	// CSTRange 返回构建该节点的原始 CST 范围。
	CSTRange() SourceRange
	// Text 返回节点覆盖的源码文本副本。
	Text() string
	// ChildCount 返回直接子节点数量。
	ChildCount() int
	// Child 返回指定位置的直接子节点。
	Child(index int) (Node, bool)
	// ChildByRole 返回指定语义角色对应的第一个直接子节点。
	ChildByRole(role Role) (Node, bool)
	// ChildRole 返回指定位置子节点的语义角色。
	ChildRole(index int) Role
	// Extension 返回节点关联的语言扩展。
	Extension() (LanguageExtension, bool)
	// Attribute 返回节点指定名称的语法属性。
	Attribute(name string) (string, bool)
}

// childLink 保存子节点及其在父节点中的语义角色。
type childLink struct {
	// node 是直接子节点。
	node *astNode
	// role 是子节点在父节点中的语义角色。
	role Role
}

// astNode 是 Common AST 节点的不可变实现。
type astNode struct {
	// id 是节点的稳定标识。
	id NodeID
	// kind 是节点的核心语法种类。
	kind NodeKind
	// sourceRange 是节点覆盖的源码范围。
	sourceRange SourceRange
	// cstRange 是构建节点的原始 CST 范围。
	cstRange SourceRange
	// text 是节点覆盖的源码文本副本。
	text string
	// children 是按源码顺序排列的直接子节点。
	children []childLink
	// extension 是节点关联的语言扩展。
	extension LanguageExtension
	// incomingRole 是节点在父节点中的语义角色，仅在构建当前 AST 时设置。
	incomingRole Role
	// attributes 保存运算符等不适合作为子节点的语法属性。
	attributes map[string]string
}

// ID 返回节点的稳定标识。
func (n *astNode) ID() NodeID {
	return n.id
}

// Kind 返回节点的核心语法种类。
func (n *astNode) Kind() NodeKind {
	return n.kind
}

// Range 返回节点覆盖的源码范围。
func (n *astNode) Range() SourceRange {
	return n.sourceRange
}

// CSTRange 返回构建该节点的原始 CST 范围。
func (n *astNode) CSTRange() SourceRange {
	return n.cstRange
}

// Text 返回节点覆盖的源码文本副本。
func (n *astNode) Text() string {
	return n.text
}

// ChildCount 返回直接子节点数量。
func (n *astNode) ChildCount() int {
	return len(n.children)
}

// Child 返回指定位置的直接子节点。
func (n *astNode) Child(index int) (Node, bool) {
	if index < 0 || index >= len(n.children) {
		return nil, false
	}
	return n.children[index].node, true
}

// ChildByRole 返回指定语义角色对应的第一个直接子节点。
func (n *astNode) ChildByRole(role Role) (Node, bool) {
	for _, child := range n.children {
		if child.role == role {
			return child.node, true
		}
	}
	return nil, false
}

// ChildRole 返回指定位置子节点的语义角色。
func (n *astNode) ChildRole(index int) Role {
	if index < 0 || index >= len(n.children) {
		return RoleNone
	}
	return n.children[index].role
}

// Extension 返回节点关联的语言扩展。
func (n *astNode) Extension() (LanguageExtension, bool) {
	return n.extension, n.extension != nil
}

// Attribute 返回节点指定名称的语法属性。
func (n *astNode) Attribute(name string) (string, bool) {
	value, ok := n.attributes[name]
	return value, ok
}
