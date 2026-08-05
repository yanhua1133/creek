(module
  (expression_statement
    (string
      (string_start)
      (string_content)
      (string_end)))
  (function_definition
    name: (identifier)
    parameters: (parameters
      (identifier)
      (identifier))
    (comment)
    body: (block
      (return_statement
        (binary_operator
          left: (identifier)
          right: (identifier)))))
  (class_definition
    name: (identifier)
    body: (block
      (function_definition
        name: (identifier)
        parameters: (parameters
          (identifier))
        body: (block
          (expression_statement
            (assignment
              left: (attribute
                object: (identifier)
                attribute: (identifier))
              right: (integer)))))
      (function_definition
        name: (identifier)
        parameters: (parameters
          (identifier))
        body: (block
          (expression_statement
            (augmented_assignment
              left: (attribute
                object: (identifier)
                attribute: (identifier))
              right: (integer)))
          (return_statement
            (attribute
              object: (identifier)
              attribute: (identifier)))))))
  (if_statement
    condition: (comparison_operator
      (identifier)
      (string
        (string_start)
        (string_content)
        (string_end)))
    consequence: (block
      (expression_statement
        (call
          function: (identifier)
          arguments: (argument_list
            (call
              function: (identifier)
              arguments: (argument_list
                (integer)
                (integer)))))))))