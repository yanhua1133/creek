package parser

import (
	"context"
	"sort"
	"strings"
	"time"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// PlaceholderForm 表示宏占位符的语法形态。
type PlaceholderForm uint8

const (
	// PlaceholderExpression 表示写成无参调用表达式 NAME()。
	PlaceholderExpression PlaceholderForm = iota + 1
	// PlaceholderStatement 表示写成无参调用语句 NAME();。
	PlaceholderStatement
	// PlaceholderDefinition 表示写成函数定义头 void NAME()。
	PlaceholderDefinition
	// PlaceholderOmitted 表示整个宏调用替换为空格。
	PlaceholderOmitted
	// PlaceholderName 表示写成名字 NAME，用于 struct、union、enum、class 之后生成类型名的宏。
	PlaceholderName
)

// Placeholder 记录一处宏占位符，即一处宏未展开、语义缺失的位置。
type Placeholder struct {
	// Range 是被替换的宏调用在原始源码中的范围，函数式宏包含实参和右括号。
	Range SourceRange
	// Name 是宏名。
	Name string
	// Form 是占位符形态。
	Form PlaceholderForm
	// Text 是写入的文本，省略形态为空。
	Text string
	// Defined 表示是否在本文件或索引中找到宏定义；为假表示外部宏。
	Defined bool
}

// placeholderMaxRounds 是放置宏占位符的最大轮数。
const placeholderMaxRounds = 3

// expressionPrefix 是表示表达式上下文的前一个非空白字符：其后的函数式宏调用写成表达式形态。
const expressionPrefix = "=([,+-*/%!~?<>&|^"

// annotationBlockers 是注解位置不允许的前一个非空白字符：其后的对象式宏处于表达式上下文，不省略。
const annotationBlockers = "=([,+-/%!~?<|^"

// errorSite 是语法树中一个 ERROR 或 MISSING 节点的字节范围。
type errorSite struct {
	// start 是节点起始字节偏移。
	start uint
	// end 是节点结束字节偏移，不包含。
	end uint
}

// collectErrorSites 返回语法树中全部 ERROR 与 MISSING 节点（含嵌套）的范围，以及位于错误子树中的节点总数。
// 不含错误的子树直接跳过，错误子树的节点数取自 DescendantCount，避免逐个访问全部节点。
func collectErrorSites(tree *ts.Tree) ([]errorSite, int) {
	var sites []errorSite
	errorNodes := 0
	var walk func(n *ts.Node, inError bool)
	walk = func(n *ts.Node, inError bool) {
		if n == nil {
			return
		}
		if n.IsError() || n.IsMissing() {
			start, end := n.ByteRange()
			sites = append(sites, errorSite{start: start, end: end})
			if !inError {
				errorNodes += int(n.DescendantCount())
			}
			inError = true
		} else if !n.HasError() {
			return
		}
		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i), inError)
		}
	}
	walk(tree.RootNode(), false)
	return sites, errorNodes
}

// isMacroLikeName 报告名字是否是宏形名字：首字符为大写字母，只含大写字母、数字和下划线，至少 2 个字符。
func isMacroLikeName(name string) bool {
	if len(name) < 2 || name[0] < 'A' || name[0] > 'Z' {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '_' {
			return false
		}
	}
	return true
}

// placeholderFinder 在一棵含错误的语法树对应的视图上查找与错误相关的宏调用，并写入占位符。
type placeholderFinder struct {
	// r 复用宏中和扫描器的词法扫描、本文件宏表、查找和写回能力。
	r *macroRewriter
	// tree 是当前视图的语法树，用于判断作用域。
	tree *ts.Tree
	// sites 是当前语法树中的错误节点范围。
	sites []errorSite
	// blocked 是已被宏中和或占位符改写的范围，候选不得与之重叠。
	blocked []SourceRange
	// found 是本轮放置的占位符。
	found []Placeholder
}

// buildPlaceholderView 在视图 view 上为与错误相关的宏调用写入占位符，返回本轮占位符和新视图；没有候选时占位符为空。
func buildPlaceholderView(view []byte, tree *ts.Tree, sites []errorSite, filePath string, lang Language, index *MacroIndex, blocked []SourceRange) ([]Placeholder, []byte) {
	f := &placeholderFinder{r: newMacroRewriter(view, filePath, lang, index), tree: tree, sites: sites, blocked: blocked}
	f.r.scan(f.identifier)
	return f.found, f.r.out
}

// identifier 判断从 start 到 end 的标识符是否是与错误相关的宏调用，是则写入占位符并返回调用结束偏移，否则返回 end。
func (f *placeholderFinder) identifier(start, end, prev int) int {
	r := f.r
	name := string(r.in[start:end])
	if reservedWords[name] {
		return end
	}
	known := f.known(name)
	if !isMacroLikeName(name) && !known {
		return end
	}
	prevChar, prevText, prevEnd := byte(0), "", 0
	if prev >= 0 {
		prevEnd = tokenEnd(r.in, prev, r.lang == LanguageCPP)
		prevChar, prevText = r.in[prev], string(r.in[prev:prevEnd])
	}
	if closing, ok := r.invocationEnd(end); ok {
		next := r.skipSpace(closing)
		if f.overlapsBlocked(start, closing) || !f.related(start, next, prevEnd) {
			return end
		}
		form, text := PlaceholderStatement, name+"();"
		nextChar := byte(0)
		if next < len(r.in) {
			nextChar = r.in[next]
		}
		scope := f.scopeAt(start)
		switch {
		case prevText == "struct" || prevText == "union" || prevText == "enum" || prevText == "class" || scope == scopeEnumerator:
			form, text = PlaceholderName, name
		case !known && scope == scopeDeclaration && afterDeclarator(prevText) && strings.IndexByte("{;=,", nextChar) >= 0:
			form, text = PlaceholderOmitted, ""
		case !known && scope == scopeDeclaration && (prev < 0 || strings.IndexByte(";{}", prevChar) >= 0) && isIdentStart(nextChar):
			form, text = PlaceholderOmitted, ""
		case scope == scopeList && startsOperand(nextChar):
			form, text = PlaceholderExpression, name+"(),"
		case scope == scopeList || (prev >= 0 && strings.IndexByte(expressionPrefix, prevChar) >= 0) || prevText == "return" || prevText == "case" || nextChar == ';':
			form, text = PlaceholderExpression, name+"()"
		case nextChar == '{' && scope == scopeDeclaration:
			form, text = PlaceholderDefinition, "void "+name+"()"
		}
		if len(text) > r.firstLineLength(start, closing) || text == string(r.in[start:closing]) {
			return end
		}
		f.place(start, closing, name, form, text, known)
		return closing
	}
	if known || !isMacroLikeName(name) {
		return end
	}
	next := r.skipSpace(end)
	if next >= len(r.in) || !(r.in[next] == '{' || isIdentStart(r.in[next])) {
		return end
	}
	if prev >= 0 && strings.IndexByte(annotationBlockers, prevChar) >= 0 {
		return end
	}
	if f.overlapsBlocked(start, end) || !f.related(start, next, prevEnd) {
		return end
	}
	f.place(start, end, name, PlaceholderOmitted, "", false)
	return end
}

// known 报告宏名是否在本文件（含已 #undef）或索引中出现过定义。
func (f *placeholderFinder) known(name string) bool {
	if _, ok := f.r.local[name]; ok {
		return true
	}
	return f.r.index != nil && len(f.r.index.definitions[name]) > 0
}

// related 报告宏调用是否与错误相关：错误起点落在 [start,next]，或错误终点落在 [prevEnd,start]。
func (f *placeholderFinder) related(start, next, prevEnd int) bool {
	for _, site := range f.sites {
		if int(site.start) >= start && int(site.start) <= next {
			return true
		}
		if prevEnd > 0 && int(site.end) >= prevEnd && int(site.end) <= start {
			return true
		}
	}
	return false
}

// overlapsBlocked 报告 [start,end) 是否与已改写范围重叠。
func (f *placeholderFinder) overlapsBlocked(start, end int) bool {
	i := sort.Search(len(f.blocked), func(i int) bool { return int(f.blocked[i].EndByte) > start })
	return i < len(f.blocked) && int(f.blocked[i].StartByte) < end
}

// placeholderScope 表示候选位置所在的作用域。
type placeholderScope uint8

const (
	// scopeDeclaration 表示文件、命名空间、extern "C" 或类体等声明作用域。
	scopeDeclaration placeholderScope = iota + 1
	// scopeStatement 表示复合语句（函数体等）作用域。
	scopeStatement
	// scopeList 表示初始化列表或实参列表作用域。
	scopeList
	// scopeEnumerator 表示枚举项列表作用域。
	scopeEnumerator
)

// declaratorQualifiers 是可以出现在函数声明符之后、注解宏之前的关键字。
var declaratorQualifiers = wordSet("const volatile override final noexcept")

// afterDeclarator 报告前一个记号是否表明当前位置在声明符之后：右括号、声明符限定关键字或普通标识符。
func afterDeclarator(prevText string) bool {
	if prevText == ")" || declaratorQualifiers[prevText] {
		return true
	}
	return prevText != "" && isIdentStart(prevText[0]) && !reservedWords[prevText]
}

// scopeAt 从偏移 start 处的最小节点向上查找，返回先遇到的作用域；ERROR 节点不影响查找。
func (f *placeholderFinder) scopeAt(start int) placeholderScope {
	node := f.tree.RootNode().DescendantForByteRange(uint(start), uint(start))
	for ; node != nil; node = node.Parent() {
		switch node.Kind() {
		case "initializer_list", "argument_list":
			return scopeList
		case "enumerator_list":
			return scopeEnumerator
		case "compound_statement":
			return scopeStatement
		case "translation_unit", "declaration_list", "field_declaration_list":
			return scopeDeclaration
		}
	}
	return scopeDeclaration
}

// startsOperand 报告字节是否可以作为下一个列表元素的开头：标识符、数字、字符串、字符或花括号。
func startsOperand(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '"' || c == '\'' || c == '{'
}

// place 写入一个占位符并记录。
func (f *placeholderFinder) place(start, end int, name string, form PlaceholderForm, text string, defined bool) {
	f.r.write(start, end, text)
	f.found = append(f.found, Placeholder{
		Range:   f.r.sourceRange(start, end),
		Name:    name,
		Form:    form,
		Text:    text,
		Defined: defined,
	})
}

// placeholderRounds 对仍有错误的语法树按轮放置宏占位符：每轮完整重新解析，保证最终语法树等于对最终视图的完整解析；
// 错误子树节点数严格减少且错误节点个数不增加时采纳，否则撤销并停止。
// 返回最终语法树；采纳的占位符和视图写回 prepared。
func placeholderRounds(ctx context.Context, tree *ts.Tree, prepared *preparedView, filePath string, lang Language, options ParseOptions) (*ts.Tree, error) {
	if options.SkipPlaceholders || prepared.macros == nil {
		return tree, nil
	}
	sites, errorNodes := collectErrorSites(tree)
	if errorNodes == 0 {
		return tree, nil
	}
	var started time.Time
	if options.CollectMetrics {
		started = time.Now()
		defer func() { prepared.metrics.Placeholder = time.Since(started) }()
	}
	for round := 0; round < placeholderMaxRounds && errorNodes > 0; round++ {
		found, view := buildPlaceholderView(prepared.content, tree, sites, filePath, lang, options.Macros, prepared.rewritten())
		if len(found) == 0 {
			break
		}
		next, _, _, err := rawParse(ctx, view, lang)
		if err != nil {
			if isContextErr(err) {
				tree.Close()
				return nil, err
			}
			break
		}
		nextSites, nextErrorNodes := collectErrorSites(next)
		if nextErrorNodes >= errorNodes || len(nextSites) > len(sites) {
			next.Close()
			break
		}
		tree.Close()
		tree, sites, errorNodes = next, nextSites, nextErrorNodes
		prepared.content = view
		prepared.placeholders = mergePlaceholders(prepared.placeholders, found)
	}
	return tree, nil
}

// mergePlaceholders 合并两组按源码顺序排列的占位符，结果仍按源码顺序排列。
func mergePlaceholders(a, b []Placeholder) []Placeholder {
	out := append(append([]Placeholder(nil), a...), b...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Range.StartByte < out[j].Range.StartByte })
	return out
}
