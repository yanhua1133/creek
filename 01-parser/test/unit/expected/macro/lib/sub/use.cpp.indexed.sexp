(translation_unit
  (comment)
  (class_specifier
    name: (type_identifier)
    (virtual_specifier)
    body: (field_declaration_list
      (access_specifier)
      (field_declaration
        (storage_class_specifier)
        (type_qualifier)
        type: (primitive_type)
        declarator: (field_identifier)
        default_value: (number_literal))
      (field_declaration
        type: (type_identifier)
        (ERROR
          (identifier))
        declarator: (field_identifier)
        default_value: (number_literal))
      (field_declaration
        type: (primitive_type)
        declarator: (function_declarator
          declarator: (field_identifier)
          parameters: (parameter_list)
          (type_qualifier)))
      (field_declaration
        type: (primitive_type)
        declarator: (function_declarator
          declarator: (field_identifier)
          parameters: (parameter_list)
          (virtual_specifier)))))
  (function_definition
    (storage_class_specifier)
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
    declarator: (init_declarator
      declarator: (identifier)
      value: (qualified_identifier
        scope: (namespace_identifier)
        name: (identifier)))))