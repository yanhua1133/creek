(translation_unit
  (comment)
  (struct_specifier
    name: (type_identifier)
    body: (field_declaration_list
      (field_declaration
        type: (primitive_type)
        declarator: (field_identifier))
      (field_declaration
        type: (primitive_type)
        declarator: (pointer_declarator
          declarator: (field_identifier)))
      (field_declaration
        type: (primitive_type)
        declarator: (field_identifier))
      (field_declaration
        (type_qualifier)
        type: (primitive_type)
        declarator: (pointer_declarator
          declarator: (field_identifier)))
      (field_declaration
        type: (primitive_type)
        declarator: (field_identifier))))
  (declaration
    (storage_class_specifier)
    type: (struct_specifier
      name: (type_identifier))
    declarator: (init_declarator
      declarator: (identifier)
      value: (initializer_list
        (call_expression
          function: (identifier)
          arguments: (argument_list))
        (string_literal
          (string_content))
        (number_literal)))))