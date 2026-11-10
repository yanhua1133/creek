(translation_unit
  (comment)
  (preproc_function_def
    name: (identifier)
    parameters: (preproc_params
      (identifier))
    value: (preproc_arg))
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
  (function_definition
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (parameter_declaration
          type: (primitive_type))))
    body: (compound_statement
      (expression_statement
        (call_expression
          function: (identifier)
          arguments: (argument_list)))
      (compound_statement
        (declaration
          type: (primitive_type)
          declarator: (init_declarator
            declarator: (identifier)
            value: (number_literal)))
        (expression_statement
          (cast_expression
            type: (type_descriptor
              type: (primitive_type))
            value: (identifier))))
      (return_statement
        (number_literal)))))