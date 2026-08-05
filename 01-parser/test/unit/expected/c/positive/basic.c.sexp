(translation_unit
  (preproc_include
    path: (system_lib_string))
  (comment)
  (function_definition
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (parameter_declaration
          type: (primitive_type)
          declarator: (identifier))
        (parameter_declaration
          type: (primitive_type)
          declarator: (identifier))))
    body: (compound_statement
      (return_statement
        (binary_expression
          left: (identifier)
          right: (identifier)))))
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
              (number_literal)))))
      (expression_statement
        (call_expression
          function: (identifier)
          arguments: (argument_list
            (string_literal
              (string_content)
              (escape_sequence))
            (identifier))))
      (return_statement
        (number_literal)))))