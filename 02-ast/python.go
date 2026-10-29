package commonast

// pythonConverter 提供 Python 语言的 CST 节点映射。
type pythonConverter struct{}

// Rules 返回 Python 语言转换规则。
func (pythonConverter) Rules() languageRules {
	return languageRules{
		kinds: mergeKinds(map[string]NodeKind{
			"module": KindFile, "import_statement": KindImportDecl, "import_from_statement": KindImportDecl,
			"function_definition": KindFunctionDecl, "class_definition": KindClassDecl,
			"typed_parameter": KindParameterDecl, "default_parameter": KindParameterDecl,
			"typed_default_parameter": KindParameterDecl, "list_splat_pattern": KindParameterDecl,
			"dictionary_splat_pattern": KindParameterDecl, "for_statement": KindForEachStmt,
			"match_statement": KindMatchStmt, "try_statement": KindTryStmt, "raise_statement": KindThrowStmt,
			"with_statement": KindWithStmt, "call": KindCallExpr, "attribute": KindMemberAccess, "subscript": KindIndexAccess,
			"assignment": KindAssignExpr, "augmented_assignment": KindAssignExpr, "named_expression": KindAssignExpr,
			"comparison_operator": KindBinaryExpr, "binary_operator": KindBinaryExpr, "unary_operator": KindUnaryExpr,
			"lambda": KindLambdaExpr, "list": KindCompositeLiteral, "dictionary": KindCompositeLiteral,
			"set": KindCompositeLiteral, "tuple": KindCompositeLiteral,
			"list_comprehension": KindComprehensionExpr, "dictionary_comprehension": KindComprehensionExpr,
			"set_comprehension": KindComprehensionExpr, "generator_expression": KindComprehensionExpr,
			"await": KindAwaitExpr, "yield": KindYieldExpr, "yield_expression": KindYieldExpr,
			"integer": KindLiteral, "float": KindLiteral, "string": KindLiteral,
			"type": KindNamedType, "generic_type": KindGenericType, "dotted_name": KindMemberAccess,
			"keyword_argument": KindAssignExpr, "pair": KindAssignExpr, "as_pattern": KindAssignExpr,
			"boolean_operator": KindBinaryExpr, "not_operator": KindUnaryExpr,
			"for_in_clause": KindForEachStmt, "except_clause": KindCaseClause,
			"finally_clause": KindCaseClause, "slice": KindIndexAccess,
			"elif_clause": KindIfStmt, "if_clause": KindIfStmt, "aliased_import": KindImportDecl,
			"type_conversion": KindCastExpr, "tuple_pattern": KindCompositeLiteral,
			"wildcard_import": KindImportDecl, "case_clause": KindCaseClause,
			"type_alias_statement": KindTypeAliasDecl, "ellipsis": KindLiteral,
		}),
		transparent: baseTransparent(),
		skipped:     baseSkipped(),
		extensions: map[string]string{
			"decorated_definition": "装饰器", "walrus_operator": "海象运算符", "string": "字符串插值",
			"decorator": "装饰器", "interpolation": "字符串插值", "dictionary_splat": "字典展开",
			"list_splat": "列表展开", "pass_statement": "空语句", "delete_statement": "删除语句",
			"keyword_separator": "仅关键字参数分隔符",
			"assert_statement":  "断言语句", "format_specifier": "格式说明",
			"format_expression": "格式化表达式", "case_pattern": "分支模式", "class_pattern": "类模式",
			"union_pattern": "联合模式", "list_pattern": "列表模式", "nonlocal_statement": "外层作用域声明",
			"global_statement": "全局作用域声明", "positional_separator": "仅位置参数分隔符",
		},
	}
}
