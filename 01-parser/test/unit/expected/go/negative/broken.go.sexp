(source_file
  (package_clause
    (package_identifier))
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
            (identifier)))
        (ERROR)))))