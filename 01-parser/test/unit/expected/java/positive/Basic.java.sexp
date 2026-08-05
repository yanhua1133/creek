(program
  (package_declaration
    (identifier))
  (line_comment)
  (class_declaration
    (modifiers)
    name: (identifier)
    body: (class_body
      (line_comment)
      (method_declaration
        (modifiers)
        type: (integral_type)
        name: (identifier)
        parameters: (formal_parameters
          (formal_parameter
            type: (integral_type)
            name: (identifier))
          (formal_parameter
            type: (integral_type)
            name: (identifier)))
        body: (block
          (return_statement
            (binary_expression
              left: (identifier)
              right: (identifier)))))
      (method_declaration
        (modifiers)
        type: (void_type)
        name: (identifier)
        parameters: (formal_parameters
          (formal_parameter
            type: (array_type
              element: (type_identifier)
              dimensions: (dimensions))
            name: (identifier)))
        body: (block
          (local_variable_declaration
            type: (type_identifier)
            declarator: (variable_declarator
              name: (identifier)
              value: (object_creation_expression
                type: (type_identifier)
                arguments: (argument_list))))
          (expression_statement
            (method_invocation
              object: (field_access
                object: (identifier)
                field: (identifier))
              name: (identifier)
              arguments: (argument_list
                (method_invocation
                  object: (identifier)
                  name: (identifier)
                  arguments: (argument_list
                    (decimal_integer_literal)
                    (decimal_integer_literal)))))))))))