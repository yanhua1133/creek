(translation_unit
  (preproc_include
    path: (system_lib_string))
  (namespace_definition
    name: (namespace_identifier)
    body: (declaration_list
      (comment)
      (class_specifier
        name: (type_identifier)
        body: (field_declaration_list
          (access_specifier)
          (comment)
          (function_definition
            type: (primitive_type)
            declarator: (function_declarator
              declarator: (field_identifier)
              parameters: (parameter_list))
            body: (compound_statement
              (return_statement
                (update_expression
                  argument: (identifier)))))
          (access_specifier)
          (field_declaration
            type: (primitive_type)
            declarator: (field_identifier)
            default_value: (number_literal))))))
  (comment)
  (comment)
  (template_declaration
    parameters: (template_parameter_list
      (type_parameter_declaration
        (type_identifier)))
    (function_definition
      type: (type_identifier)
      declarator: (function_declarator
        declarator: (identifier)
        parameters: (parameter_list
          (parameter_declaration
            type: (type_identifier)
            declarator: (identifier))
          (parameter_declaration
            type: (type_identifier)
            declarator: (identifier))))
      body: (compound_statement
        (return_statement
          (conditional_expression
            condition: (binary_expression
              left: (identifier)
              right: (identifier))
            consequence: (identifier)
            alternative: (identifier))))))
  (function_definition
    type: (primitive_type)
    declarator: (function_declarator
      declarator: (identifier)
      parameters: (parameter_list))
    body: (compound_statement
      (declaration
        type: (qualified_identifier
          scope: (namespace_identifier)
          name: (type_identifier))
        declarator: (identifier))
      (return_statement
        (call_expression
          function: (identifier)
          arguments: (argument_list
            (call_expression
              function: (field_expression
                argument: (identifier)
                field: (field_identifier))
              arguments: (argument_list))
            (number_literal)))))))