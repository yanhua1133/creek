(translation_unit
  (preproc_ifdef
    name: (identifier)
    (preproc_def
      name: (identifier))
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
            (function_definition
              (explicit_function_specifier)
              declarator: (function_declarator
                declarator: (identifier)
                parameters: (parameter_list
                  (parameter_declaration
                    type: (qualified_identifier
                      scope: (namespace_identifier)
                      name: (type_identifier))
                    declarator: (identifier))))
              (field_initializer_list
                (field_initializer
                  (field_identifier)
                  (argument_list
                    (identifier))))
              body: (compound_statement))
            (function_definition
              (type_qualifier)
              type: (qualified_identifier
                scope: (namespace_identifier)
                name: (type_identifier))
              declarator: (reference_declarator
                (function_declarator
                  declarator: (field_identifier)
                  parameters: (parameter_list)
                  (type_qualifier)))
              body: (compound_statement
                (return_statement
                  (identifier))))
            (access_specifier)
            (field_declaration
              type: (qualified_identifier
                scope: (namespace_identifier)
                name: (type_identifier))
              declarator: (field_identifier))))
        (comment)
        (template_declaration
          parameters: (template_parameter_list
            (type_parameter_declaration
              (type_identifier)))
          (class_specifier
            name: (type_identifier)
            body: (field_declaration_list
              (access_specifier)
              (field_declaration
                type: (type_identifier)
                declarator: (field_identifier)))))))
    (comment))
  (comment))