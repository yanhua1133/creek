(translation_unit
  (comment)
  (function_definition
    type: (struct_specifier
      name: (type_identifier)
      body: (field_declaration_list
        (field_declaration
          type: (primitive_type)
          declarator: (field_identifier))
        (field_declaration
          type: (type_identifier)
          (ERROR))
        (field_declaration
          type: (type_identifier)
          (ERROR
            (field_identifier))
          declarator: (function_declarator
            declarator: (field_identifier)
            parameters: (parameter_list
              (parameter_declaration
                (type_qualifier)
                type: (primitive_type)
                declarator: (pointer_declarator
                  declarator: (identifier)))
              (ERROR
                (identifier)))))
        (field_declaration
          type: (type_identifier)
          (ERROR
            (field_identifier)
            (function_declarator
              declarator: (field_identifier)
              parameters: (parameter_list
                (parameter_declaration
                  type: (primitive_type)
                  declarator: (identifier)))))
          declarator: (field_identifier)
          (ERROR
            (number_literal)))))
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (ERROR
          (number_literal)
          (number_literal))))
    (declaration
      type: (primitive_type)
      declarator: (function_declarator
        declarator: (identifier)
        parameters: (parameter_list
          (parameter_declaration
            (type_qualifier)
            type: (primitive_type)
            declarator: (pointer_declarator
              declarator: (identifier)))
          (variadic_parameter))))
    (declaration
      type: (type_identifier)
      (ERROR
        (identifier))
      declarator: (identifier))
    (ERROR
      (storage_class_specifier)
      type: (primitive_type)
      (function_declarator
        declarator: (identifier)
        parameters: (parameter_list
          (parameter_declaration
            type: (struct_specifier
              name: (type_identifier))
            declarator: (identifier)))))
    body: (compound_statement
      (return_statement
        (binary_expression
          left: (field_expression
            argument: (identifier)
            field: (field_identifier))
          right: (field_expression
            argument: (identifier)
            field: (field_identifier)))))))