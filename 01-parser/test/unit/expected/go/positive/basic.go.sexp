(source_file
  (package_clause
    (package_identifier))
  (import_declaration
    (import_spec
      path: (interpreted_string_literal
        (interpreted_string_literal_content))))
  (comment)
  (function_declaration
    name: (identifier)
    parameters: (parameter_list
      (parameter_declaration
        name: (identifier)
        name: (identifier)
        type: (type_identifier)))
    result: (type_identifier)
    body: (block
      (statement_list
        (return_statement
          (expression_list
            (binary_expression
              left: (identifier)
              right: (identifier)))))))
  (comment)
  (function_declaration
    name: (identifier)
    parameters: (parameter_list
      (parameter_declaration
        name: (identifier)
        type: (type_identifier)))
    body: (block
      (statement_list
        (expression_statement
          (call_expression
            function: (selector_expression
              operand: (identifier)
              field: (field_identifier))
            arguments: (argument_list
              (interpreted_string_literal
                (interpreted_string_literal_content)
                (escape_sequence))
              (identifier))))))))