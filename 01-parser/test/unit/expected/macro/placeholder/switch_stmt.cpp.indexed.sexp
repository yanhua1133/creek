(translation_unit
  (comment)
  (preproc_function_def
    name: (identifier)
    parameters: (preproc_params
      (identifier)
      (identifier))
    value: (preproc_arg))
  (function_definition
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (parameter_declaration
          type: (primitive_type)
          declarator: (identifier))))
    body: (compound_statement
      (switch_statement
        condition: (condition_clause
          value: (identifier))
        body: (compound_statement
          (expression_statement
            (call_expression
              function: (identifier)
              arguments: (argument_list)))
          (expression_statement
            (call_expression
              function: (identifier)
              arguments: (argument_list)))
          (case_statement
            (return_statement
              (number_literal))))))))