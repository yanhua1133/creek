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
            arguments: (argument_list
              (number_literal)
              (number_literal))))
        (ERROR))
      (return_statement
        (identifier)))))