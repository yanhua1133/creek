(translation_unit
  (comment)
  (function_definition
    type: (class_specifier
      name: (type_identifier))
    (ERROR
      (identifier))
    declarator: (identifier)
    body: (compound_statement
      (labeled_statement
        label: (statement_identifier)
        (declaration
          type: (type_identifier)
          (ERROR
            (identifier))
          declarator: (init_declarator
            declarator: (identifier)
            value: (number_literal))))
      (declaration
        type: (type_identifier)
        (ERROR
          (identifier))
        declarator: (init_declarator
          declarator: (identifier)
          value: (number_literal)))
      (expression_statement
        (call_expression
          function: (identifier)
          arguments: (argument_list
            (string_literal
              (string_content)))))
      (declaration
        type: (primitive_type)
        declarator: (function_declarator
          declarator: (identifier)
          parameters: (parameter_list)
          (type_qualifier)))
      (declaration
        type: (primitive_type)
        (ERROR
          (init_declarator
            declarator: (identifier)
            value: (argument_list)))
        declarator: (identifier))))
  (expression_statement)
  (function_definition
    type: (type_identifier)
    (ERROR
      (identifier))
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
          right: (number_literal)))))
  (declaration
    type: (primitive_type)
    declarator: (init_declarator
      declarator: (identifier)
      value: (qualified_identifier
        scope: (namespace_identifier)
        name: (identifier)))))