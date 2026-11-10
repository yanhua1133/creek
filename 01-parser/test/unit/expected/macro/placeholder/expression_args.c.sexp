(translation_unit
  (comment)
  (function_definition
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (parameter_declaration
          type: (primitive_type))))
    body: (compound_statement
      (declaration
        type: (primitive_type)
        declarator: (init_declarator
          declarator: (identifier)
          value: (call_expression
            function: (identifier)
            arguments: (argument_list))))
      (return_statement
        (binary_expression
          left: (identifier)
          right: (call_expression
            function: (identifier)
            arguments: (argument_list
              (identifier)
              (number_literal))))))))