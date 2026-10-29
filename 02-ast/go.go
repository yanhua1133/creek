package commonast

// goConverter 提供 Go 语言的 CST 节点映射。
type goConverter struct{}

// Rules 返回 Go 语言转换规则。
func (goConverter) Rules() languageRules {
	transparent := baseTransparent()
	transparent["range_clause"] = true
	transparent["import_spec"] = true
	transparent["negated_type"] = true
	transparent["parenthesized_type"] = true
	return languageRules{
		kinds: mergeKinds(map[string]NodeKind{
			"source_file": KindFile, "package_clause": KindPackageDecl, "import_declaration": KindImportDecl,
			"function_declaration": KindFunctionDecl, "method_declaration": KindFunctionDecl,
			"type_declaration": KindTypeAliasDecl, "type_spec": KindTypeAliasDecl, "struct_type": KindStructDecl,
			"interface_type": KindInterfaceDecl, "field_declaration": KindFieldDecl,
			"var_declaration": KindVariableDecl, "const_declaration": KindConstDecl,
			"parameter_declaration": KindParameterDecl, "short_var_declaration": KindVarDeclStmt,
			"for_statement": KindForStmt, "expression_switch_statement": KindSwitchStmt,
			"type_switch_statement": KindSwitchStmt, "defer_statement": KindDeferStmt, "go_statement": KindGoStmt,
			"selector_expression": KindMemberAccess, "index_expression": KindIndexAccess,
			"assignment_statement": KindAssignExpr, "inc_statement": KindUnaryExpr, "dec_statement": KindUnaryExpr,
			"composite_literal": KindCompositeLiteral, "func_literal": KindLambdaExpr,
			"pointer_type": KindPointerType, "array_type": KindArrayType, "slice_type": KindSliceType,
			"map_type": KindMapType, "function_type": KindFunctionType, "generic_type": KindGenericType,
			"keyed_element": KindAssignExpr, "qualified_type": KindMemberAccess,
			"var_spec": KindVariableDecl, "type_conversion_expression": KindCastExpr,
			"const_spec": KindConstDecl, "expression_case": KindCaseClause, "default_case": KindCaseClause,
			"slice_expression": KindIndexAccess, "type_assertion_expression": KindCastExpr,
			"method_elem": KindFunctionDecl, "type_elem": KindTypeAliasDecl,
			"variadic_parameter_declaration": KindParameterDecl, "type_parameter_declaration": KindParameterDecl,
			"communication_case": KindCaseClause, "implicit_length_array_type": KindArrayType,
			"label_name": KindNameRef, "iota": KindLiteral, "type_case": KindCaseClause,
			"type_constraint": KindGenericType, "type_alias": KindTypeAliasDecl,
			"labeled_statement": KindLabelStmt,
		}),
		transparent: transparent,
		skipped:     baseSkipped(),
		extensions: map[string]string{
			"channel_type": "通道类型", "select_statement": "通道选择", "receive_statement": "通道接收",
			"send_statement": "通道发送", "dot": "类型推断占位符",
			"fallthrough_statement": "贯穿下一分支",
		},
	}
}
