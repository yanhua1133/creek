package commonast

import (
	"context"
	"fmt"
	"runtime"
	"runtime/trace"
	"time"

	parser "creek/01-parser"
)

// Converter 定义单个源文件从 CST 到 Common AST 的转换能力。
type Converter interface {
	// Convert 将一个已解析的语法树转换为 Common AST，并返回转换诊断。
	Convert(ctx context.Context, tree parser.SyntaxTree) (CommonAST, []Diagnostic, error)
	// ConvertWithOptions 按选项转换语法树，并返回 AST、诊断和可选观测数据。
	ConvertWithOptions(ctx context.Context, tree parser.SyntaxTree, options ConversionOptions) (ConversionResult, error)
}

// dispatcher 按源码语言选择独立语言转换器，并复用公共遍历实现。
type dispatcher struct {
	// maxDepth 是允许转换的最大 CST 嵌套深度。
	maxDepth int
}

// builder 保存单棵 CST 转换过程中的局部状态。
type builder struct {
	// ctx 用于响应调用方取消请求。
	ctx context.Context
	// language 是当前源码语言。
	language Language
	// rules 是当前语言的转换规则。
	rules languageRules
	// maxDepth 是允许转换的最大 CST 嵌套深度。
	maxDepth int
	// nextID 是下一个待分配的稳定节点标识。
	nextID NodeID
	// nodes 保存已生成节点的 NodeID 索引。
	nodes map[NodeID]*astNode
	// cstNodes 保存 CST 范围到 AST 节点的反向索引。
	cstNodes map[rangeKey][]*astNode
	// diagnostics 保存按源码顺序产生的转换诊断。
	diagnostics []Diagnostic
}

// New 返回默认 Common AST 转换器。
func New() Converter {
	return &dispatcher{maxDepth: 4096}
}

// BuildCommonAST 使用默认转换器把语法树转换为 Common AST。
func BuildCommonAST(ctx context.Context, tree parser.SyntaxTree) (CommonAST, []Diagnostic, error) {
	return New().Convert(ctx, tree)
}

// Convert 按语言选择独立规则并转换整棵语法树。
func (d *dispatcher) Convert(ctx context.Context, tree parser.SyntaxTree) (CommonAST, []Diagnostic, error) {
	result, err := d.ConvertWithOptions(ctx, tree, ConversionOptions{})
	return result.AST, result.Diagnostics, err
}

// ConvertWithOptions 按语言选择独立规则，并按需采集指标、运行时跟踪和结构化快照。
func (d *dispatcher) ConvertWithOptions(ctx context.Context, tree parser.SyntaxTree, options ConversionOptions) (ConversionResult, error) {
	var result ConversionResult
	var started time.Time
	var before runtime.MemStats
	if options.CollectMetrics {
		started = time.Now()
		runtime.ReadMemStats(&before)
	}
	if options.EnableTrace {
		region := trace.StartRegion(ctx, "commonast.convert")
		defer region.End()
	}
	maxDepth := d.maxDepth
	if options.MaxDepth > 0 {
		maxDepth = options.MaxDepth
	}
	ast, diagnostics, inputBytes, err := d.convert(ctx, tree, maxDepth)
	result.AST = ast
	result.Diagnostics = diagnostics
	if err == nil && options.CollectSnapshot {
		result.Snapshot, err = MarshalSnapshot(ast)
	}
	if options.CollectMetrics {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		result.Metrics = &ConversionMetrics{
			Elapsed:         time.Since(started),
			AllocatedBytes:  after.TotalAlloc - before.TotalAlloc,
			InputBytes:      inputBytes,
			DiagnosticCount: len(diagnostics),
		}
		if ast != nil {
			if internal, ok := ast.(*commonAST); ok {
				result.Metrics.NodeCount = len(internal.nodes)
			}
		}
	}
	return result, err
}

// convert 执行单棵 CST 到 Common AST 的核心转换并返回输入字节数。
func (d *dispatcher) convert(ctx context.Context, tree parser.SyntaxTree, maxDepth int) (CommonAST, []Diagnostic, uint, error) {
	if tree == nil {
		return nil, nil, 0, ErrInvalidSyntaxTree
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, 0, err
	}
	converter, ok := converterFor(tree.Language())
	if !ok {
		return nil, nil, 0, ErrUnsupportedLanguage
	}
	root := tree.Root()
	if _, err := root.Kind(); err != nil {
		return nil, nil, 0, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
	}
	rootRange, err := root.Range()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
	}
	b := &builder{
		ctx:      ctx,
		language: tree.Language(),
		rules:    converter.Rules(),
		maxDepth: maxDepth,
		nextID:   1,
		nodes:    make(map[NodeID]*astNode),
		cstNodes: make(map[rangeKey][]*astNode),
	}
	nodes, err := b.convertNode(root, 0, RoleNone)
	if err != nil {
		return nil, b.diagnostics, rootRange.EndByte - rootRange.StartByte, err
	}
	if len(nodes) != 1 || nodes[0].kind != KindFile {
		return nil, b.diagnostics, rootRange.EndByte - rootRange.StartByte, fmt.Errorf("Common AST 根节点无效")
	}
	result := &commonAST{
		language: tree.Language(),
		fileID:   tree.FileID(),
		path:     tree.Path(),
		root:     nodes[0],
		nodes:    b.nodes,
		cstNodes: b.cstNodes,
	}
	return result, append([]Diagnostic(nil), b.diagnostics...), rootRange.EndByte - rootRange.StartByte, nil
}

// converterFor 返回指定语言的独立转换器。
func converterFor(language Language) (languageConverter, bool) {
	switch language {
	case parser.LanguageC:
		return cConverter{}, true
	case parser.LanguageCPP:
		return cppConverter{}, true
	case parser.LanguageJava:
		return javaConverter{}, true
	case parser.LanguageGo:
		return goConverter{}, true
	case parser.LanguagePython:
		return pythonConverter{}, true
	default:
		return nil, false
	}
}

// convertNode 把一个 CST 节点转换为零个、一个或多个 AST 节点。
func (b *builder) convertNode(source parser.Node, depth int, inheritedRole Role) ([]*astNode, error) {
	if err := b.ctx.Err(); err != nil {
		return nil, err
	}
	if depth > b.maxDepth {
		sourceRange, _ := source.Range()
		kind, _ := source.Kind()
		b.diagnostics = append(b.diagnostics, Diagnostic{Kind: DiagnosticDepthLimit, Message: "语法树嵌套深度超过限制", Range: sourceRange, CSTKind: kind})
		return nil, nil
	}
	kind, err := source.Kind()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
	}
	isError, err := source.IsError()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
	}
	isMissing, err := source.IsMissing()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
	}
	rootError := depth == 0 && isError
	if (isError || isMissing) && !rootError {
		sourceRange, _ := source.Range()
		b.diagnostics = append(b.diagnostics, Diagnostic{Kind: DiagnosticUpstreamSyntax, Message: "跳过上游语法错误节点", Range: sourceRange, CSTKind: kind})
		return nil, nil
	}
	if rootError {
		sourceRange, _ := source.Range()
		b.diagnostics = append(b.diagnostics, Diagnostic{Kind: DiagnosticUpstreamSyntax, Message: "上游根节点是语法错误，继续构建可恢复的文件 AST", Range: sourceRange, CSTKind: kind})
	}
	if b.rules.skipped[kind] {
		return nil, nil
	}
	if b.rules.transparent[kind] {
		return b.convertChildren(source, depth, inheritedRole)
	}
	mappedKind, mapped := b.mappedKind(source, kind)
	if rootError {
		mappedKind, mapped = KindFile, true
	}
	if !mapped {
		mappedKind = KindUnknown
	}
	sourceRange, err := source.Range()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
	}
	var text []byte
	if shouldStoreText(mappedKind) {
		text, err = source.Text()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
		}
	}
	node := &astNode{
		id:           b.nextID,
		kind:         mappedKind,
		sourceRange:  sourceRange,
		cstRange:     sourceRange,
		text:         string(text),
		incomingRole: inheritedRole,
		attributes:   extractAttributes(source, mappedKind),
	}
	b.nextID++
	b.nodes[node.id] = node
	key := rangeKey{start: sourceRange.StartByte, end: sourceRange.EndByte}
	b.cstNodes[key] = append(b.cstNodes[key], node)
	if extensionKind, ok := b.rules.extensions[kind]; ok {
		node.extension = languageExtension{language: b.language, kind: extensionKind, sourceRange: sourceRange}
	}
	if !mapped {
		node.extension = languageExtension{language: b.language, kind: "未映射语法:" + kind, sourceRange: sourceRange}
		b.diagnostics = append(b.diagnostics, Diagnostic{Kind: DiagnosticUnmappedSyntax, Message: "语法节点尚未映射到核心节点", Range: sourceRange, CSTKind: kind})
	}
	children, err := b.convertChildren(source, depth, RoleNone)
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		role := childRoleFromSource(source, child, inheritedRole)
		node.children = append(node.children, childLink{node: child, role: role})
	}
	return []*astNode{node}, nil
}

// shouldStoreText 报告节点是否需要保存自身源码文本。
func shouldStoreText(kind NodeKind) bool {
	switch kind {
	case KindUnknown, KindLiteral, KindNameRef, KindNamedType, KindQualifierType:
		return true
	default:
		return false
	}
}

// extractAttributes 从非命名语法子节点中提取需要保真的运算符属性。
func extractAttributes(source parser.Node, kind NodeKind) map[string]string {
	if kind != KindMemberAccess && kind != KindAssignExpr && kind != KindBinaryExpr && kind != KindUnaryExpr && kind != KindTernaryExpr {
		return nil
	}
	count, err := source.ChildCount()
	if err != nil {
		return nil
	}
	for index := 0; index < count; index++ {
		child, ok, err := source.Child(index)
		if err != nil || !ok {
			continue
		}
		named, err := child.IsNamed()
		if err != nil || named {
			continue
		}
		text, err := child.Text()
		if err != nil {
			continue
		}
		operator := string(text)
		if isOperatorToken(operator) {
			return map[string]string{"operator": operator}
		}
	}
	return nil
}

// isOperatorToken 报告文本是否是需要保留的运算符。
func isOperatorToken(text string) bool {
	switch text {
	case ".", "->", "::", "=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=",
		"+", "-", "*", "/", "%", "==", "!=", "<", "<=", ">", ">=", "&&", "||", "!", "~", "&", "|", "^", "<<", ">>", "?":
		return true
	default:
		return false
	}
}

// mappedKind 返回当前 CST 节点对应的核心节点类型，并处理需要查看子节点的语言规则。
func (b *builder) mappedKind(source parser.Node, cstKind string) (NodeKind, bool) {
	if b.language == parser.LanguageGo && cstKind == "for_statement" {
		if hasDirectChildKind(source, "range_clause") {
			return KindForEachStmt, true
		}
	}
	kind, ok := b.rules.kinds[cstKind]
	if !ok {
		if _, extension := b.rules.extensions[cstKind]; extension {
			return KindUnknown, true
		}
	}
	return kind, ok
}

// hasDirectChildKind 报告节点是否包含指定类型的直接命名子节点。
func hasDirectChildKind(source parser.Node, expected string) bool {
	count, err := source.ChildCount()
	if err != nil {
		return false
	}
	for index := 0; index < count; index++ {
		child, ok, err := source.Child(index)
		if err != nil || !ok {
			continue
		}
		kind, err := child.Kind()
		if err == nil && kind == expected {
			return true
		}
	}
	return false
}

// convertChildren 按源码顺序转换一个 CST 节点的所有命名子节点。
func (b *builder) convertChildren(source parser.Node, depth int, inheritedRole Role) ([]*astNode, error) {
	count, err := source.ChildCount()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
	}
	var result []*astNode
	for index := 0; index < count; index++ {
		child, ok, err := source.Child(index)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
		}
		if !ok {
			continue
		}
		named, err := child.IsNamed()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
		}
		if !named {
			continue
		}
		field, err := source.FieldNameForChild(index)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidSyntaxTree, err)
		}
		role := roleForField(field)
		if role == RoleNone {
			role = inheritedRole
		}
		converted, err := b.convertNode(child, depth+1, role)
		if err != nil {
			return nil, err
		}
		result = append(result, converted...)
	}
	return result, nil
}

// childRoleFromSource 返回子节点在当前父节点中的语义角色。
func childRoleFromSource(_ parser.Node, child *astNode, fallback Role) Role {
	if child.incomingRole != RoleNone {
		return child.incomingRole
	}
	return fallback
}
