(translation_unit
  (preproc_ifdef
    name: (identifier)
    (preproc_def
      name: (identifier))
    (comment)
    (struct_specifier
      name: (type_identifier)
      body: (field_declaration_list
        (field_declaration
          type: (primitive_type)
          declarator: (field_identifier))
        (field_declaration
          type: (primitive_type)
          declarator: (field_identifier))))
    (declaration
      type: (primitive_type)
      declarator: (function_declarator
        declarator: (identifier)
        parameters: (parameter_list
          (parameter_declaration
            type: (primitive_type)
            declarator: (identifier))
          (parameter_declaration
            type: (primitive_type)
            declarator: (identifier))))))
  (comment))