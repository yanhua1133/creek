(translation_unit
  (comment)
  (struct_specifier
    name: (type_identifier)
    body: (field_declaration_list
      (field_declaration
        type: (primitive_type)
        declarator: (field_identifier))
      (field_declaration
        type: (type_identifier))))
  (declaration
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (parameter_declaration
          (type_qualifier)
          type: (primitive_type)
          declarator: (pointer_declarator
            declarator: (identifier))))))
  (function_definition
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (parameter_declaration
          type: (primitive_type)
          declarator: (identifier))))
    body: (compound_statement
      (return_statement
        (binary_expression
          left: (identifier)
          right: (number_literal)))))
  (declaration
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (parameter_declaration
          (type_qualifier)
          type: (primitive_type)
          declarator: (pointer_declarator
            declarator: (identifier))))))
  (declaration
    type: (primitive_type)
    declarator: (identifier))
  (function_definition
    (storage_class_specifier)
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (parameter_declaration
          type: (struct_specifier
            name: (type_identifier))
          declarator: (identifier))))
    body: (compound_statement
      (return_statement
        (binary_expression
          left: (field_expression
            argument: (identifier)
            field: (field_identifier))
          right: (field_expression
            argument: (identifier)
            field: (field_identifier)))))))