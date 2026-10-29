package commonast

// languageRules 定义一种语言的 CST 节点映射、透明包装和扩展分类。
type languageRules struct {
	// kinds 把 CST 节点类型映射为 Common AST 节点类型。
	kinds map[string]NodeKind
	// transparent 保存只负责语法分组、不建立 AST 节点的 CST 类型。
	transparent map[string]bool
	// skipped 保存注释等不进入核心 AST 的 CST 类型。
	skipped map[string]bool
	// extensions 保存需要附加语言扩展的 CST 类型及扩展名称。
	extensions map[string]string
}

// languageConverter 提供单一语言的映射规则。
type languageConverter interface {
	// Rules 返回该语言不可变的转换规则。
	Rules() languageRules
}

// baseTransparent 返回五种语言共用的透明语法包装节点。
func baseTransparent() map[string]bool {
	return map[string]bool{
		"argument_list": true, "parameter_list": true, "formal_parameters": true, "parameters": true,
		"expression_list": true, "declaration_list": true, "parenthesized_expression": true,
		"parenthesized_declarator": true, "function_declarator": true, "method_declaration_body": true,
		"class_body": true, "interface_body": true, "enum_body": true, "field_declaration_list": true,
		"modifiers": true, "type_arguments": true, "type_parameters": true, "type_parameter_list": true,
		"init_declarator": true, "variable_declarator": true, "identifier_list": true,
		"condition_clause": true, "statement_list": true, "type_parameter": true,
		"type_descriptor": true, "preproc_params": true, "enumerator_list": true,
		"else_clause": true, "subscript_argument_list": true,
		"template_argument_list": true, "template_parameter_list": true, "field_initializer_list": true,
		"annotation_argument_list": true, "throws": true, "type_list": true,
		"super_interfaces": true, "superclass": true, "type_bound": true,
		"import_spec_list": true, "literal_element": true, "literal_value": true,
		"variadic_argument": true, "for_clause": true, "var_spec_list": true,
		"with_item": true, "with_clause": true, "pattern_list": true,
		"relative_import": true, "import_prefix": true, "lambda_parameters": true,
		"as_pattern_target": true,
	}
}

// baseSkipped 返回五种语言共用的非语义节点。
func baseSkipped() map[string]bool {
	return map[string]bool{
		"comment": true, "line_comment": true, "block_comment": true,
		"string_content": true, "escape_sequence": true, "interpreted_string_literal_content": true,
		"raw_string_delimiter": true, "raw_string_content": true, "character": true,
		"string_fragment": true, "multiline_string_fragment": true, "raw_string_literal_content": true,
		"string_start": true, "string_end": true,
	}
}

// mergeKinds 合并公共节点映射与语言专用映射。
func mergeKinds(specific map[string]NodeKind) map[string]NodeKind {
	result := map[string]NodeKind{
		"identifier": KindNameRef, "field_identifier": KindNameRef, "type_identifier": KindNamedType,
		"namespace_identifier": KindNameRef, "package_identifier": KindNameRef,
		"primitive_type": KindNamedType, "scoped_type_identifier": KindNamedType,
		"integral_type": KindNamedType, "floating_point_type": KindNamedType, "boolean_type": KindNamedType,
		"string_literal": KindLiteral, "character_literal": KindLiteral, "number_literal": KindLiteral,
		"integer_literal": KindLiteral, "float_literal": KindLiteral, "true": KindLiteral, "false": KindLiteral,
		"decimal_integer_literal": KindLiteral, "hex_integer_literal": KindLiteral, "octal_integer_literal": KindLiteral,
		"int_literal": KindLiteral, "interpreted_string_literal": KindLiteral, "raw_string_literal": KindLiteral,
		"system_lib_string": KindLiteral, "rune_literal": KindLiteral, "concatenated_string": KindLiteral,
		"char_literal": KindLiteral, "null": KindLiteral, "this": KindNameRef, "super": KindNameRef,
		"blank_identifier": KindNameRef,
		"type_qualifier":   KindQualifierType, "storage_class_specifier": KindQualifierType,
		"sized_type_specifier": KindNamedType, "abstract_pointer_declarator": KindPointerType,
		"abstract_array_declarator": KindArrayType, "abstract_function_declarator": KindFunctionType,
		"field_declaration": KindFieldDecl, "case_statement": KindCaseClause,
		"sizeof_expression": KindUnaryExpr, "comma_expression": KindBinaryExpr,
		"compound_literal_expression": KindCompositeLiteral, "initializer_pair": KindAssignExpr,
		"variadic_parameter": KindParameterDecl, "statement_identifier": KindNameRef,
		"null_literal": KindLiteral, "none": KindLiteral, "nil": KindLiteral,
		"binary_expression": KindBinaryExpr, "unary_expression": KindUnaryExpr,
		"assignment_expression": KindAssignExpr, "conditional_expression": KindTernaryExpr,
		"call_expression": KindCallExpr, "subscript_expression": KindIndexAccess,
		"return_statement": KindReturnStmt, "break_statement": KindBreakStmt,
		"continue_statement": KindContinueStmt, "if_statement": KindIfStmt,
		"while_statement": KindWhileStmt, "expression_statement": KindExprStmt,
		"compound_statement": KindBlock, "block": KindBlock,
	}
	for kind, mapped := range specific {
		result[kind] = mapped
	}
	return result
}

// roleForField 把 Tree-sitter 字段名转换为跨语言语义角色。
func roleForField(field string) Role {
	switch field {
	case "name":
		return RoleName
	case "body", "consequence":
		return RoleBody
	case "condition":
		return RoleCondition
	case "initializer", "init":
		return RoleInit
	case "update":
		return RoleUpdate
	case "alternative":
		return RoleElse
	case "function":
		return RoleCallee
	case "arguments", "argument":
		return RoleArgument
	case "receiver":
		return RoleReceiver
	case "return_type":
		return RoleReturnType
	case "type":
		return RoleType
	case "value":
		return RoleValue
	case "left":
		return RoleLeft
	case "right":
		return RoleRight
	case "object":
		return RoleObject
	case "field", "member":
		return RoleMember
	case "parameters", "parameter":
		return RoleParameter
	case "target":
		return RoleTarget
	case "iterable":
		return RoleIterable
	default:
		return RoleNone
	}
}
