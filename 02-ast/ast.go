// Package commonast 把 Parser 输出的语言相关 CST 转换为跨语言统一命名的 Common AST。
package commonast

import parser "creek/01-parser"

// Language 复用 Parser 定义的源码语言枚举。
type Language = parser.Language

// FileID 复用 Parser 定义的源码文件稳定标识。
type FileID = parser.FileID

// SourceRange 复用 Parser 定义的源码范围。
type SourceRange = parser.SourceRange

// NodeID 是 AST 节点在单个文件 AST 中的稳定标识。
type NodeID uint64

// NodeKind 表示 Common AST 节点的语法种类。
type NodeKind uint16

const (
	// KindUnknown 表示当前版本尚未归类但已完整保留的语法节点。
	KindUnknown NodeKind = iota
	// KindFile 表示单个源码文件的根节点。
	KindFile
	// KindImportDecl 表示依赖引入声明。
	KindImportDecl
	// KindPackageDecl 表示包声明。
	KindPackageDecl
	// KindNamespaceDecl 表示命名空间声明。
	KindNamespaceDecl
	// KindFunctionDecl 表示函数、方法或构造函数定义。
	KindFunctionDecl
	// KindClassDecl 表示类或记录类型声明。
	KindClassDecl
	// KindStructDecl 表示结构体声明。
	KindStructDecl
	// KindInterfaceDecl 表示接口声明。
	KindInterfaceDecl
	// KindEnumDecl 表示枚举声明。
	KindEnumDecl
	// KindUnionDecl 表示联合体声明。
	KindUnionDecl
	// KindTypeAliasDecl 表示类型别名声明。
	KindTypeAliasDecl
	// KindFieldDecl 表示字段或成员变量声明。
	KindFieldDecl
	// KindVariableDecl 表示变量声明。
	KindVariableDecl
	// KindConstDecl 表示常量声明。
	KindConstDecl
	// KindParameterDecl 表示形式参数声明。
	KindParameterDecl
	// KindEnumMemberDecl 表示枚举成员声明。
	KindEnumMemberDecl
	// KindBlock 表示语句块。
	KindBlock
	// KindExprStmt 表示表达式语句。
	KindExprStmt
	// KindVarDeclStmt 表示局部变量声明语句。
	KindVarDeclStmt
	// KindIfStmt 表示条件语句。
	KindIfStmt
	// KindForStmt 表示 C 风格三段式循环。
	KindForStmt
	// KindForEachStmt 表示遍历集合或区间的循环。
	KindForEachStmt
	// KindWhileStmt 表示前测循环。
	KindWhileStmt
	// KindDoWhileStmt 表示后测循环。
	KindDoWhileStmt
	// KindSwitchStmt 表示值分支选择语句。
	KindSwitchStmt
	// KindMatchStmt 表示结构化模式匹配语句。
	KindMatchStmt
	// KindReturnStmt 表示返回语句。
	KindReturnStmt
	// KindBreakStmt 表示跳出循环或分支。
	KindBreakStmt
	// KindContinueStmt 表示继续下一次循环。
	KindContinueStmt
	// KindLabelStmt 表示标签语句。
	KindLabelStmt
	// KindGotoStmt 表示无条件跳转语句。
	KindGotoStmt
	// KindTryStmt 表示异常处理语句。
	KindTryStmt
	// KindThrowStmt 表示抛出异常语句。
	KindThrowStmt
	// KindWithStmt 表示 Python 资源上下文语句。
	KindWithStmt
	// KindDeferStmt 表示 Go 延迟执行语句。
	KindDeferStmt
	// KindGoStmt 表示 Go 启动 goroutine 的语句。
	KindGoStmt
	// KindSyncStmt 表示同步语句块。
	KindSyncStmt
	// KindCaseClause 表示 switch 或 match 中的单个分支子句。
	KindCaseClause
	// KindLiteral 表示字面量。
	KindLiteral
	// KindNameRef 表示名称引用。
	KindNameRef
	// KindMemberAccess 表示成员访问表达式。
	KindMemberAccess
	// KindIndexAccess 表示下标访问表达式。
	KindIndexAccess
	// KindCallExpr 表示调用表达式。
	KindCallExpr
	// KindNewExpr 表示显式对象构造表达式。
	KindNewExpr
	// KindUnaryExpr 表示一元运算表达式。
	KindUnaryExpr
	// KindBinaryExpr 表示二元运算表达式。
	KindBinaryExpr
	// KindAssignExpr 表示赋值或复合赋值表达式。
	KindAssignExpr
	// KindTernaryExpr 表示三元条件表达式。
	KindTernaryExpr
	// KindLambdaExpr 表示匿名函数或闭包表达式。
	KindLambdaExpr
	// KindCastExpr 表示显式类型转换表达式。
	KindCastExpr
	// KindCompositeLiteral 表示聚合或集合字面量。
	KindCompositeLiteral
	// KindComprehensionExpr 表示 Python 推导式。
	KindComprehensionExpr
	// KindAwaitExpr 表示等待异步结果的表达式。
	KindAwaitExpr
	// KindYieldExpr 表示生成值的表达式。
	KindYieldExpr
	// KindNamedType 表示具名类型引用。
	KindNamedType
	// KindPointerType 表示指针类型。
	KindPointerType
	// KindReferenceType 表示 C++ 引用类型。
	KindReferenceType
	// KindArrayType 表示定长数组类型。
	KindArrayType
	// KindSliceType 表示 Go 切片类型。
	KindSliceType
	// KindMapType 表示映射类型。
	KindMapType
	// KindFunctionType 表示函数类型。
	KindFunctionType
	// KindGenericType 表示泛型实例化类型。
	KindGenericType
	// KindQualifierType 表示带限定符的类型。
	KindQualifierType
)

// nodeKindNames 保存 NodeKind 对应的稳定名称。
var nodeKindNames = map[NodeKind]string{
	KindUnknown: "Unknown", KindFile: "File", KindImportDecl: "ImportDecl", KindPackageDecl: "PackageDecl",
	KindNamespaceDecl: "NamespaceDecl", KindFunctionDecl: "FunctionDecl", KindClassDecl: "ClassDecl",
	KindStructDecl: "StructDecl", KindInterfaceDecl: "InterfaceDecl", KindEnumDecl: "EnumDecl",
	KindUnionDecl: "UnionDecl", KindTypeAliasDecl: "TypeAliasDecl", KindFieldDecl: "FieldDecl",
	KindVariableDecl: "VariableDecl", KindConstDecl: "ConstDecl", KindParameterDecl: "ParameterDecl",
	KindEnumMemberDecl: "EnumMemberDecl", KindBlock: "Block", KindExprStmt: "ExprStmt",
	KindVarDeclStmt: "VarDeclStmt", KindIfStmt: "IfStmt", KindForStmt: "ForStmt",
	KindForEachStmt: "ForEachStmt", KindWhileStmt: "WhileStmt", KindDoWhileStmt: "DoWhileStmt",
	KindSwitchStmt: "SwitchStmt", KindMatchStmt: "MatchStmt", KindReturnStmt: "ReturnStmt",
	KindBreakStmt: "BreakStmt", KindContinueStmt: "ContinueStmt", KindLabelStmt: "LabelStmt",
	KindGotoStmt: "GotoStmt", KindTryStmt: "TryStmt", KindThrowStmt: "ThrowStmt",
	KindWithStmt: "WithStmt", KindDeferStmt: "DeferStmt", KindGoStmt: "GoStmt", KindSyncStmt: "SyncStmt",
	KindCaseClause: "CaseClause",
	KindLiteral:    "Literal", KindNameRef: "NameRef", KindMemberAccess: "MemberAccess",
	KindIndexAccess: "IndexAccess", KindCallExpr: "CallExpr", KindNewExpr: "NewExpr",
	KindUnaryExpr: "UnaryExpr", KindBinaryExpr: "BinaryExpr", KindAssignExpr: "AssignExpr",
	KindTernaryExpr: "TernaryExpr", KindLambdaExpr: "LambdaExpr", KindCastExpr: "CastExpr",
	KindCompositeLiteral: "CompositeLiteral", KindComprehensionExpr: "ComprehensionExpr",
	KindAwaitExpr: "AwaitExpr", KindYieldExpr: "YieldExpr", KindNamedType: "NamedType",
	KindPointerType: "PointerType", KindReferenceType: "ReferenceType", KindArrayType: "ArrayType",
	KindSliceType: "SliceType", KindMapType: "MapType", KindFunctionType: "FunctionType",
	KindGenericType: "GenericType", KindQualifierType: "QualifierType",
}

// String 返回节点种类的稳定名称。
func (k NodeKind) String() string {
	if name, ok := nodeKindNames[k]; ok {
		return name
	}
	return "Unknown"
}

// IsStructuralKind 报告节点是否属于跨语言控制结构骨架的一部分。
func IsStructuralKind(kind NodeKind) bool {
	switch kind {
	case KindFunctionDecl, KindBlock, KindExprStmt, KindIfStmt, KindForStmt, KindForEachStmt,
		KindWhileStmt, KindDoWhileStmt, KindSwitchStmt, KindMatchStmt, KindReturnStmt,
		KindBreakStmt, KindContinueStmt, KindTryStmt, KindThrowStmt, KindWithStmt,
		KindDeferStmt, KindGoStmt, KindSyncStmt, KindCaseClause, KindCallExpr:
		return true
	default:
		return false
	}
}
