package commonast

// javaConverter 提供 Java 语言的 CST 节点映射。
type javaConverter struct{}

// Rules 返回 Java 语言转换规则。
func (javaConverter) Rules() languageRules {
	return languageRules{
		kinds: mergeKinds(map[string]NodeKind{
			"program": KindFile, "package_declaration": KindPackageDecl, "import_declaration": KindImportDecl,
			"class_declaration": KindClassDecl, "record_declaration": KindClassDecl, "interface_declaration": KindInterfaceDecl,
			"enum_declaration": KindEnumDecl, "enum_constant": KindEnumMemberDecl, "method_declaration": KindFunctionDecl,
			"constructor_declaration": KindFunctionDecl, "field_declaration": KindFieldDecl,
			"local_variable_declaration": KindVarDeclStmt, "formal_parameter": KindParameterDecl,
			"spread_parameter": KindParameterDecl, "for_statement": KindForStmt, "enhanced_for_statement": KindForEachStmt,
			"do_statement": KindDoWhileStmt, "switch_expression": KindSwitchStmt, "try_statement": KindTryStmt,
			"throw_statement": KindThrowStmt, "synchronized_statement": KindSyncStmt,
			"method_invocation": KindCallExpr, "object_creation_expression": KindNewExpr,
			"field_access": KindMemberAccess, "array_access": KindIndexAccess, "ternary_expression": KindTernaryExpr,
			"lambda_expression": KindLambdaExpr, "cast_expression": KindCastExpr, "array_initializer": KindCompositeLiteral,
			"generic_type": KindGenericType, "array_type": KindArrayType, "void_type": KindNamedType,
			"scoped_identifier": KindMemberAccess, "class_literal": KindLiteral,
			"element_value_pair": KindAssignExpr, "wildcard": KindGenericType,
			"dimensions": KindArrayType, "dimensions_expr": KindArrayType, "method_reference": KindMemberAccess,
			"array_creation_expression": KindNewExpr, "constructor_body": KindBlock,
			"decimal_floating_point_literal": KindLiteral, "catch_formal_parameter": KindParameterDecl,
			"catch_clause": KindCaseClause, "catch_type": KindNamedType,
			"element_value_array_initializer": KindCompositeLiteral,
			"explicit_constructor_invocation": KindCallExpr, "instanceof_expression": KindBinaryExpr,
			"annotation_type_element_declaration": KindFieldDecl, "resource": KindVariableDecl,
			"switch_label": KindCaseClause, "switch_rule": KindCaseClause,
			"try_with_resources_statement": KindTryStmt, "update_expression": KindUnaryExpr,
			"constant_declaration": KindConstDecl, "finally_clause": KindCaseClause,
			"module_declaration": KindNamespaceDecl, "static_initializer": KindBlock,
			"compact_constructor_declaration": KindFunctionDecl, "yield_statement": KindYieldExpr,
			"labeled_statement": KindLabelStmt, "asterisk": KindNameRef,
		}),
		transparent: javaTransparent(),
		skipped:     baseSkipped(),
		extensions: map[string]string{
			"marker_annotation": "注解", "annotation": "注解", "annotation_type_declaration": "注解类型声明",
			"annotated_type": "带注解类型", "annotation_type_body": "注解类型主体",
			"requires_module_directive": "模块依赖", "requires_modifier": "模块依赖限定符",
			"exports_module_directive": "模块导出", "uses_module_directive": "模块服务使用",
			"provides_module_directive": "模块服务提供", "opens_module_directive": "模块开放",
			"pattern": "模式", "type_pattern": "类型模式", "guard": "模式守卫",
			"record_pattern_body": "记录模式主体", "record_pattern_component": "记录模式成员",
			"record_pattern": "记录模式",
		},
	}
}

// javaTransparent 返回 Java 语言只负责语法分组的节点类型。
func javaTransparent() map[string]bool {
	transparent := baseTransparent()
	transparent["resource_specification"] = true
	transparent["inferred_parameters"] = true
	transparent["extends_interfaces"] = true
	transparent["switch_block"] = true
	transparent["enum_body_declarations"] = true
	transparent["module_body"] = true
	transparent["permits"] = true
	return transparent
}
