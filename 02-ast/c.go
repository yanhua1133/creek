package commonast

// cConverter 提供 C 语言的 CST 节点映射。
type cConverter struct{}

// Rules 返回 C 语言转换规则。
func (cConverter) Rules() languageRules {
	return languageRules{
		kinds: mergeKinds(map[string]NodeKind{
			"translation_unit": KindFile, "preproc_include": KindImportDecl, "function_definition": KindFunctionDecl,
			"preproc_arg": KindLiteral,
			"declaration": KindVariableDecl, "type_definition": KindTypeAliasDecl, "parameter_declaration": KindParameterDecl,
			"struct_specifier": KindStructDecl, "union_specifier": KindUnionDecl, "enum_specifier": KindEnumDecl,
			"enumerator": KindEnumMemberDecl, "for_statement": KindForStmt, "do_statement": KindDoWhileStmt,
			"switch_statement": KindSwitchStmt, "labeled_statement": KindLabelStmt, "goto_statement": KindGotoStmt,
			"field_expression": KindMemberAccess, "pointer_expression": KindUnaryExpr, "update_expression": KindUnaryExpr,
			"cast_expression": KindCastExpr, "initializer_list": KindCompositeLiteral, "pointer_declarator": KindPointerType,
			"array_declarator": KindArrayType, "function_declarator": KindFunctionType,
			"macro_type_specifier": KindNamedType, "ms_pointer_modifier": KindQualifierType,
			"ms_restrict_modifier": KindQualifierType, "optional_parameter_declaration": KindParameterDecl,
		}),
		transparent: cTransparent(),
		skipped:     baseSkipped(),
		extensions: map[string]string{
			"preproc_include": "预处理指令", "preproc_def": "宏定义", "preproc_function_def": "函数式宏定义",
			"preproc_if": "条件编译", "preproc_ifdef": "条件编译", "preproc_elif": "条件编译",
			"preproc_else": "条件编译", "preproc_defined": "宏条件", "preproc_call": "宏调用",
			"preproc_directive": "预处理指令", "field_designator": "指定初始化器",
			"attribute_specifier": "编译器属性", "bitfield_clause": "位域",
			"offsetof_expression": "字段偏移计算", "static_assert_declaration": "静态断言",
			"gnu_asm_expression": "GNU 内联汇编", "gnu_asm_qualifier": "GNU 汇编限定符",
			"gnu_asm_output_operand": "GNU 汇编输出操作数", "gnu_asm_input_operand": "GNU 汇编输入操作数",
			"gnu_asm_clobber_list": "GNU 汇编破坏列表", "subscript_designator": "数组指定初始化器",
			"linkage_specification": "链接约定", "friend_declaration": "友元声明", "delete_expression": "删除表达式",
		},
	}
}

// cTransparent 返回 C 语言只负责语法分组的节点类型。
func cTransparent() map[string]bool {
	transparent := baseTransparent()
	transparent["gnu_asm_output_operand_list"] = true
	transparent["gnu_asm_input_operand_list"] = true
	transparent["abstract_parenthesized_declarator"] = true
	return transparent
}
