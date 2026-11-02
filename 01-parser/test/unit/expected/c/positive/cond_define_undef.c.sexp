(translation_unit
  (preproc_def
    name: (identifier))
  (preproc_call
    directive: (preproc_directive)
    argument: (preproc_arg))
  (function_definition
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
          type: (primitive_type))))
    body: (compound_statement
      (return_statement
        (number_literal)))))