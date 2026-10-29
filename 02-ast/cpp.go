package commonast

// cppConverter 提供 C++ 语言的 CST 节点映射。
type cppConverter struct{}

// Rules 返回 C++ 语言转换规则。
func (cppConverter) Rules() languageRules {
	transparent := baseTransparent()
	transparent["variadic_declarator"] = true
	transparent["attributed_declarator"] = true
	transparent["trailing_return_type"] = true
	transparent["abstract_parenthesized_declarator"] = true
	transparent["new_declarator"] = true
	transparent["init_statement"] = true
	transparent["gnu_asm_output_operand_list"] = true
	transparent["gnu_asm_input_operand_list"] = true
	return languageRules{
		kinds: mergeKinds(map[string]NodeKind{
			"translation_unit": KindFile, "preproc_include": KindImportDecl, "namespace_definition": KindNamespaceDecl,
			"preproc_arg":         KindLiteral,
			"function_definition": KindFunctionDecl, "declaration": KindVariableDecl, "parameter_declaration": KindParameterDecl,
			"class_specifier": KindClassDecl, "struct_specifier": KindStructDecl, "union_specifier": KindUnionDecl,
			"enum_specifier": KindEnumDecl, "enumerator": KindEnumMemberDecl, "field_declaration": KindFieldDecl,
			"for_statement": KindForStmt, "for_range_loop": KindForEachStmt, "do_statement": KindDoWhileStmt,
			"switch_statement": KindSwitchStmt, "try_statement": KindTryStmt, "throw_statement": KindThrowStmt,
			"field_expression": KindMemberAccess, "qualified_identifier": KindMemberAccess, "new_expression": KindNewExpr,
			"lambda_expression": KindLambdaExpr, "cast_expression": KindCastExpr, "update_expression": KindUnaryExpr,
			"initializer_list": KindCompositeLiteral, "pointer_declarator": KindPointerType,
			"reference_declarator": KindReferenceType, "array_declarator": KindArrayType,
			"scoped_identifier": KindMemberAccess, "type_qualifier": KindQualifierType,
			"access_specifier": KindUnknown, "pointer_expression": KindUnaryExpr,
			"template_function": KindGenericType, "template_method": KindGenericType, "template_type": KindGenericType,
			"auto": KindNamedType, "placeholder_type_specifier": KindNamedType,
			"type_parameter_declaration": KindParameterDecl, "field_initializer": KindAssignExpr,
			"labeled_statement": KindLabelStmt, "pointer_type_declarator": KindPointerType,
			"virtual_specifier": KindQualifierType, "explicit_function_specifier": KindQualifierType,
			"abstract_reference_declarator": KindReferenceType, "alias_declaration": KindTypeAliasDecl,
			"type_definition": KindTypeAliasDecl, "operator_name": KindNameRef,
			"destructor_name": KindNameRef, "using_declaration": KindImportDecl,
			"dependent_type": KindGenericType, "optional_parameter_declaration": KindParameterDecl,
			"variadic_parameter_declaration": KindParameterDecl, "variadic_type_parameter_declaration": KindParameterDecl,
			"optional_type_parameter_declaration": KindParameterDecl, "nested_namespace_specifier": KindNameRef,
			"alignof_expression": KindUnaryExpr, "alignas_qualifier": KindQualifierType,
			"dependent_name": KindNameRef, "goto_statement": KindGotoStmt,
			"template_instantiation": KindGenericType, "ref_qualifier": KindQualifierType,
			"operator_cast": KindNameRef, "fold_expression": KindBinaryExpr, "macro_type_specifier": KindNamedType,
			"template_template_parameter_declaration": KindParameterDecl, "catch_clause": KindCaseClause,
		}),
		transparent: transparent,
		skipped:     baseSkipped(),
		extensions: map[string]string{
			"preproc_include": "预处理指令", "template_declaration": "模板声明", "concept_definition": "概念定义",
			"access_specifier": "访问控制",
			"preproc_def":      "宏定义", "preproc_function_def": "函数式宏定义", "preproc_if": "条件编译",
			"preproc_ifdef": "条件编译", "preproc_elif": "条件编译", "preproc_else": "条件编译",
			"preproc_defined": "宏条件", "preproc_call": "宏调用", "preproc_directive": "预处理指令",
			"field_designator": "指定初始化器", "attribute_specifier": "编译器属性", "bitfield_clause": "位域",
			"attribute": "属性", "attribute_declaration": "属性声明", "lambda_capture_specifier": "Lambda 捕获",
			"lambda_default_capture": "Lambda 默认捕获", "base_class_clause": "基类列表",
			"offsetof_expression": "字段偏移计算", "static_assert_declaration": "静态断言",
			"parameter_pack_expansion": "参数包展开", "delete_expression": "删除表达式",
			"attributed_statement": "带属性语句", "noexcept": "不抛异常说明", "decltype": "表达式类型推导",
			"friend_declaration": "友元声明", "delete_method_clause": "删除方法说明",
			"default_method_clause": "默认方法说明", "linkage_specification": "链接约定",
			"structured_binding_declarator": "结构化绑定", "namespace_alias_definition": "命名空间别名",
			"gnu_asm_expression": "GNU 内联汇编", "gnu_asm_output_operand": "GNU 汇编输出操作数",
			"gnu_asm_input_operand": "GNU 汇编输入操作数", "gnu_asm_qualifier": "GNU 汇编限定符",
			"gnu_asm_clobber_list": "GNU 汇编破坏列表", "lambda_capture_initializer": "Lambda 捕获初始化器",
			"pure_virtual_clause": "纯虚函数说明",
		},
	}
}
