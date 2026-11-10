(translation_unit
  (comment)
  (struct_specifier
    name: (type_identifier)
    body: (field_declaration_list
      (function_definition
        type: (primitive_type)
        declarator: (pointer_declarator
          declarator: (function_declarator
            declarator: (field_identifier)
            parameters: (parameter_list)))
        body: (compound_statement
          (return_statement
            (identifier))))
      (field_declaration
        type: (primitive_type)
        declarator: (array_declarator
          declarator: (field_identifier)
          size: (number_literal)))))
  (declaration
    type: (primitive_type)
    declarator: (init_declarator
      declarator: (identifier)
      value: (identifier))))