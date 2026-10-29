package commonast

// LanguageExtension 承载核心节点无法完整表达的语言特有语法信息。
type LanguageExtension interface {
	// Language 返回扩展所属的编程语言。
	Language() Language
	// Kind 返回扩展类别的稳定名称。
	Kind() string
	// Range 返回扩展对应的源码范围。
	Range() SourceRange
}

// languageExtension 是语言扩展的不可变实现。
type languageExtension struct {
	// language 是扩展所属的编程语言。
	language Language
	// kind 是扩展类别的稳定名称。
	kind string
	// sourceRange 是扩展覆盖的源码范围。
	sourceRange SourceRange
}

// Language 返回扩展所属的编程语言。
func (e languageExtension) Language() Language {
	return e.language
}

// Kind 返回扩展类别的稳定名称。
func (e languageExtension) Kind() string {
	return e.kind
}

// Range 返回扩展对应的源码范围。
func (e languageExtension) Range() SourceRange {
	return e.sourceRange
}
