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
      (return_statement
        (binary_expression
          left: (identifier)
          right: (identifier)))))
  (preproc_def
    name: (identifier)
    value: (preproc_arg))
  (declaration
    type: (primitive_type)
    declarator: (pointer_declarator
      (type_qualifier)
      declarator: (identifier)))
  (preproc_call
    directive: (preproc_directive)
    argument: (preproc_arg))
  (declaration
    type: (primitive_type)
    declarator: (init_declarator
      declarator: (identifier)
      value: (number_literal)))
  (declaration
    type: (primitive_type)
    declarator: (identifier))
  (declaration
    type: (primitive_type)
    declarator: (identifier))
  (declaration
    type: (primitive_type)
    declarator: (init_declarator
      declarator: (identifier)
      value: (identifier)))
  (declaration
    type: (primitive_type)
    declarator: (init_declarator
      declarator: (identifier)
      value: (identifier)))
  (function_definition
    (storage_class_specifier)
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list
        (parameter_declaration
          type: (primitive_type))))
    body: (compound_statement
      (return_statement
        (number_literal))))
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
            (binary_expression
              left: (identifier)
              right: (number_literal))))))))