(translation_unit
  (comment)
  (namespace_definition
    name: (namespace_identifier)
    body: (declaration_list
      (function_definition
        type: (primitive_type)
        declarator: (function_declarator
          declarator: (identifier)
          parameters: (parameter_list))
        body: (compound_statement
          (declaration
            type: (primitive_type)
            declarator: (init_declarator
              declarator: (identifier)
              value: (number_literal)))
          (expression_statement
            (call_expression
              function: (identifier)
              arguments: (argument_list
                (binary_expression
                  left: (identifier)
                  right: (number_literal)))))))))
  (comment))