(translation_unit
  (comment)
  (function_definition
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (parameter_declaration
          type: (primitive_type)
          declarator: (identifier))))
    body: (compound_statement
      (expression_statement
        (call_expression
          function: (identifier)
          arguments: (argument_list
            (identifier))))
      (return_statement
        (call_expression
          function: (identifier)
          arguments: (argument_list
            (identifier)))))))