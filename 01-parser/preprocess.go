package parser

import (
	"sort"
	"strconv"
	"strings"
)

// ConditionValue 表示条件编译条件的求值结果。
type ConditionValue uint8

const (
	// ConditionUnknown 表示条件依赖外部配置或无法解析，无法确定真假。
	ConditionUnknown ConditionValue = iota + 1
	// ConditionTrue 表示条件确定为真。
	ConditionTrue
	// ConditionFalse 表示条件确定为假。
	ConditionFalse
)

// ConditionalBranch 记录条件编译中一个分支的判断与选择结果。
type ConditionalBranch struct {
	// Directive 是分支指令所在的源码范围，从 # 开始，包含续行，不含行尾换行符。
	Directive SourceRange
	// Keyword 是指令名称：if、ifdef、ifndef、elif 或 else。
	Keyword string
	// Condition 是合并续行、去掉注释并压缩空白后的条件文本，else 分支为空。
	Condition string
	// Value 是条件求值结果，else 分支为真。
	Value ConditionValue
	// Taken 表示该分支是否保留在解析视图中。
	Taken bool
}

// PreprocessReport 记录一次 C/C++ 解析前的条件编译分支选择结果。
type PreprocessReport struct {
	// Branches 是按源码顺序排列的全部条件分支。
	Branches []ConditionalBranch
	// BlankedBytes 是替换为空格、不参与解析的字节数，不含保留的换行符和回车符。
	BlankedBytes int
	// Unterminated 是缺少 #endif 的条件块数量。
	Unterminated int
}

// BuildConditionalView 为 C/C++ 源码生成条件编译分支选择后的解析视图。
// 视图与原文件等长，只把未保留分支和条件编译指令行中的非换行字节替换为空格；lang 决定 __cplusplus 是否定义。
func BuildConditionalView(content []byte, lang Language) ([]byte, PreprocessReport) {
	s := newConditionalScanner(content, lang)
	s.run()
	return s.view, s.report
}

// macroState 记录本文件已保留代码中某个宏的定义状态。
type macroState struct {
	// defined 表示宏当前是否已定义。
	defined bool
	// hasValue 表示宏的替换内容是否为可参与比较的整数字面量。
	hasValue bool
	// value 是宏替换内容对应的整数值。
	value int64
}

// conditionalGroup 记录一组 #if/#elif/#else/#endif 的选择状态。
type conditionalGroup struct {
	// parentActive 表示该组所在区域是否保留在解析视图中。
	parentActive bool
	// anyTaken 表示该组是否已有分支被保留。
	anyTaken bool
	// currentTaken 表示当前分支是否被保留。
	currentTaken bool
	// parentReachable 表示该组所在区域是否可达，即没有被确定为假的条件排除。
	parentReachable bool
	// anyTrue 表示该组是否已有条件确定为真的分支。
	anyTrue bool
	// currentReachable 表示当前分支是否可达：条件不为假，且同组前面没有确定为真的分支。
	currentReachable bool
}

// defineEvent 记录可达区域中的一条 #define 或 #undef 指令，供宏定义索引使用。
type defineEvent struct {
	// keyword 是指令名称：define 或 undef。
	keyword string
	// rest 是指令名称之后的原始文本，包含续行。
	rest string
	// line 是指令所在行号，从 1 开始。
	line int
	// taken 表示指令是否位于分支选择保留的区域。
	taken bool
}

// conditionalScanner 按词法扫描源码，识别条件编译指令并生成解析视图。
type conditionalScanner struct {
	// content 是原始源码。
	content []byte
	// view 是正在生成的解析视图。
	view []byte
	// cpp 表示是否按 C++ 规则扫描和求值。
	cpp bool
	// lineStarts 保存每行首字节偏移，用于把偏移换算为行列位置。
	lineStarts []int
	// macros 保存本文件已保留代码中的宏定义状态。
	macros map[string]macroState
	// groups 是当前嵌套的条件编译组。
	groups []conditionalGroup
	// report 是正在生成的报告。
	report PreprocessReport
	// collectDefines 为真时把可达区域中的 #define 和 #undef 记入 defines。
	collectDefines bool
	// defines 是按源码顺序记录的可达宏定义指令。
	defines []defineEvent
}

// newConditionalScanner 创建扫描器并初始化 __cplusplus 的定义状态。
func newConditionalScanner(content []byte, lang Language) *conditionalScanner {
	view := make([]byte, len(content))
	copy(view, content)
	s := &conditionalScanner{
		content:    content,
		view:       view,
		cpp:        lang == LanguageCPP,
		lineStarts: []int{0},
		macros:     map[string]macroState{},
	}
	for i, c := range content {
		if c == '\n' {
			s.lineStarts = append(s.lineStarts, i+1)
		}
	}
	for name, value := range targetDefined {
		s.macros[name] = macroState{defined: true, hasValue: true, value: value}
	}
	for _, name := range targetUndefined {
		s.macros[name] = macroState{defined: false}
	}
	if s.cpp {
		s.macros["__cplusplus"] = macroState{defined: true, hasValue: true, value: 201703}
		s.macros["__GNUG__"] = macroState{defined: true, hasValue: true, value: 13}
		s.macros["__STDC_VERSION__"] = macroState{defined: false}
	} else {
		s.macros["__cplusplus"] = macroState{defined: false}
		s.macros["__GNUG__"] = macroState{defined: false}
		s.macros["__STDC_VERSION__"] = macroState{defined: true, hasValue: true, value: 201710}
	}
	return s
}

// targetDefined 是默认目标配置（Linux x86_64、GCC 兼容编译器、标准头文件已包含）下已定义的宏及其值。
var targetDefined = map[string]int64{
	"__GNUC__": 13, "__GNUC_MINOR__": 0, "__GNUC_PATCHLEVEL__": 0, "__STDC__": 1, "__STDC_HOSTED__": 1,
	"__linux__": 1, "__linux": 1, "__gnu_linux__": 1, "__unix__": 1, "__unix": 1, "__ELF__": 1,
	"__x86_64__": 1, "__x86_64": 1, "__amd64__": 1, "__amd64": 1, "__LP64__": 1, "_LP64": 1,
	"__CHAR_BIT__": 8, "__SIZEOF_INT__": 4, "__SIZEOF_LONG__": 8, "__SIZEOF_LONG_LONG__": 8, "__SIZEOF_POINTER__": 8,
	"__SIZEOF_SIZE_T__": 8, "__ORDER_LITTLE_ENDIAN__": 1234, "__ORDER_BIG_ENDIAN__": 4321, "__BYTE_ORDER__": 1234,
	"CHAR_BIT": 8, "INT_MAX": 2147483647, "UINT_MAX": 4294967295, "LONG_MAX": 9223372036854775807,
	"LLONG_MAX": 9223372036854775807, "INT32_MAX": 2147483647, "UINT32_MAX": 4294967295,
	"INT64_MAX": 9223372036854775807, "INTPTR_MAX": 9223372036854775807, "PTRDIFF_MAX": 9223372036854775807,
	"SIZE_MAX": 9223372036854775807, "UINTPTR_MAX": 9223372036854775807, "UINT64_MAX": 9223372036854775807,
	"ULONG_MAX": 9223372036854775807, "ULLONG_MAX": 9223372036854775807,
}

// targetUndefined 是默认目标配置下确定未定义的平台和编译器宏。
var targetUndefined = []string{
	"_WIN32", "_WIN64", "WIN32", "_MSC_VER", "__MINGW32__", "__MINGW64__", "__CYGWIN__", "__MSYS__",
	"__APPLE__", "__MACH__", "__FreeBSD__", "__OpenBSD__", "__NetBSD__", "__DragonFly__", "__sun", "__SVR4",
	"_AIX", "__HAIKU__", "__ANDROID__", "__EMSCRIPTEN__", "__wasm__", "__clang__", "__INTEL_COMPILER",
	"__aarch64__", "__arm__", "__i386__", "__powerpc__", "__ppc64__", "__s390x__", "__riscv",
}

// scanDefines 按指定语言扫描源码，返回可达区域中按源码顺序排列的 #define 和 #undef 指令。
func scanDefines(content []byte, lang Language) []defineEvent {
	s := newConditionalScanner(content, lang)
	s.collectDefines = true
	s.run()
	return s.defines
}

// reachable 报告当前位置是否可达，即没有被确定为假的条件排除。
func (s *conditionalScanner) reachable() bool {
	if len(s.groups) == 0 {
		return true
	}
	top := s.groups[len(s.groups)-1]
	return top.parentReachable && top.currentReachable
}

// active 报告当前位置是否保留在解析视图中。
func (s *conditionalScanner) active() bool {
	if len(s.groups) == 0 {
		return true
	}
	top := s.groups[len(s.groups)-1]
	return top.parentActive && top.currentTaken
}

// blank 把 [start,end) 中除换行符和回车符外的字节替换为空格，并计入报告。
func (s *conditionalScanner) blank(start, end int) {
	for i := start; i < end; i++ {
		if s.content[i] == '\n' || s.content[i] == '\r' {
			continue
		}
		s.view[i] = ' '
		s.report.BlankedBytes++
	}
}

// point 把字节偏移换算为零基行列位置。
func (s *conditionalScanner) point(offset int) SourcePoint {
	row := sort.Search(len(s.lineStarts), func(i int) bool { return s.lineStarts[i] > offset }) - 1
	return SourcePoint{Row: uint(row), Column: uint(offset - s.lineStarts[row])}
}

// sourceRange 构造 [start,end) 对应的源码范围。
func (s *conditionalScanner) sourceRange(start, end int) SourceRange {
	return SourceRange{StartByte: uint(start), EndByte: uint(end), StartPoint: s.point(start), EndPoint: s.point(end)}
}

// run 从头到尾扫描源码：识别行首指令，跳过注释、字符串和字符字面量，并按当前分支状态生成视图。
func (s *conditionalScanner) run() {
	n := len(s.content)
	lineStart := true
	for i := 0; i < n; {
		c := s.content[i]
		if c == '\n' {
			lineStart = true
			i++
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v' {
			s.blankIfInactive(i, i+1)
			i++
			continue
		}
		if c == '#' && lineStart {
			i = s.directive(i)
			continue
		}
		lineStart = false
		end := s.token(i)
		s.blankIfInactive(i, end)
		if end > i && s.content[end-1] == '\n' {
			lineStart = true
		}
		i = end
	}
	s.report.Unterminated = len(s.groups)
}

// blankIfInactive 在当前区域未保留时替换 [start,end)。
func (s *conditionalScanner) blankIfInactive(start, end int) {
	if !s.active() {
		s.blank(start, end)
	}
}

// token 返回从 i 开始的一个词法单元的结束偏移，正确跳过注释、字符串、字符字面量、原始字符串和数字。
func (s *conditionalScanner) token(i int) int {
	return tokenEnd(s.content, i, s.cpp)
}

// tokenEnd 返回从 i 开始的一个词法单元的结束偏移；cpp 为真时识别 C++ 原始字符串。
func tokenEnd(content []byte, i int, cpp bool) int {
	n := len(content)
	c := content[i]
	switch {
	case c == '/' && i+1 < n && content[i+1] == '/':
		return lineCommentEnd(content, i)
	case c == '/' && i+1 < n && content[i+1] == '*':
		return blockCommentEnd(content, i)
	case c == '"' || c == '\'':
		return quotedEnd(content, i)
	case isDigit(c) || (c == '.' && i+1 < n && isDigit(content[i+1])):
		return numberEnd(content, i)
	case isIdentStart(c):
		end := i
		for end < n && isIdentPart(content[end]) {
			end++
		}
		if cpp && end < n && content[end] == '"' {
			switch string(content[i:end]) {
			case "R", "u8R", "uR", "UR", "LR":
				if rawEnd, ok := rawStringEnd(content, end); ok {
					return rawEnd
				}
			}
		}
		return end
	default:
		return i + 1
	}
}

// directive 处理从 start 开始的一条预处理指令，返回指令结束后的偏移（指向行尾换行符或文件末尾）。
func (s *conditionalScanner) directive(start int) int {
	end := directiveEnd(s.content, start)
	text := string(s.content[start:end])
	keyword, rest := splitDirective(text)
	switch keyword {
	case "if", "ifdef", "ifndef":
		parentActive := s.active()
		condition := normalizeCondition(rest)
		value := s.evaluate(keyword, condition)
		taken := parentActive && value != ConditionFalse
		s.groups = append(s.groups, conditionalGroup{
			parentActive:     parentActive,
			anyTaken:         taken,
			currentTaken:     taken,
			parentReachable:  s.reachable(),
			anyTrue:          value == ConditionTrue,
			currentReachable: value != ConditionFalse,
		})
		s.record(start, end, keyword, condition, value, taken)
		s.blank(start, end)
	case "elif", "else":
		if len(s.groups) == 0 {
			s.blankIfInactive(start, end)
			return end
		}
		group := &s.groups[len(s.groups)-1]
		condition := ""
		value := ConditionTrue
		if keyword == "elif" {
			condition = normalizeCondition(rest)
			value = s.evaluate(keyword, condition)
		}
		taken := group.parentActive && !group.anyTaken && value != ConditionFalse
		group.currentTaken = taken
		group.anyTaken = group.anyTaken || taken
		group.currentReachable = !group.anyTrue && value != ConditionFalse
		group.anyTrue = group.anyTrue || value == ConditionTrue
		s.record(start, end, keyword, condition, value, taken)
		s.blank(start, end)
	case "endif":
		if len(s.groups) == 0 {
			return end
		}
		s.groups = s.groups[:len(s.groups)-1]
		s.blank(start, end)
	default:
		if s.collectDefines && (keyword == "define" || keyword == "undef") && s.reachable() {
			s.defines = append(s.defines, defineEvent{keyword: keyword, rest: rest, line: int(s.point(start).Row) + 1, taken: s.active()})
		}
		if !s.active() {
			s.blank(start, end)
			return end
		}
		switch keyword {
		case "define":
			s.define(rest)
		case "undef":
			name := leadingIdentifier(strings.TrimSpace(normalizeCondition(rest)))
			if name != "" {
				s.macros[name] = macroState{defined: false}
			}
		}
	}
	return end
}

// record 把一个条件分支写入报告。
func (s *conditionalScanner) record(start, end int, keyword, condition string, value ConditionValue, taken bool) {
	s.report.Branches = append(s.report.Branches, ConditionalBranch{
		Directive: s.sourceRange(start, end),
		Keyword:   keyword,
		Condition: condition,
		Value:     value,
		Taken:     taken,
	})
}

// define 记录一条已保留的 #define：函数式宏只记为已定义，对象式宏的替换内容为整数字面量时记录其值。
func (s *conditionalScanner) define(rest string) {
	rest = strings.TrimLeft(strings.ReplaceAll(strings.ReplaceAll(rest, "\\\r\n", " "), "\\\n", " "), " \t")
	name := leadingIdentifier(rest)
	if name == "" {
		return
	}
	after := rest[len(name):]
	if strings.HasPrefix(after, "(") {
		s.macros[name] = macroState{defined: true}
		return
	}
	body := normalizeCondition(after)
	if value, ok := parseInteger(body); ok {
		s.macros[name] = macroState{defined: true, hasValue: true, value: value}
		return
	}
	s.macros[name] = macroState{defined: true}
}

// evaluate 按指令类型求值条件。
func (s *conditionalScanner) evaluate(keyword, condition string) ConditionValue {
	switch keyword {
	case "ifdef", "ifndef":
		name := leadingIdentifier(condition)
		if name == "" || name != condition {
			return ConditionUnknown
		}
		state, known := s.macros[name]
		if !known {
			return ConditionUnknown
		}
		if state.defined == (keyword == "ifdef") {
			return ConditionTrue
		}
		return ConditionFalse
	default:
		e := &conditionEvaluator{tokens: tokenizeCondition(condition), macros: s.macros}
		result, ok := e.parseOr()
		if !ok || e.pos != len(e.tokens) || !result.known {
			return ConditionUnknown
		}
		if result.value != 0 {
			return ConditionTrue
		}
		return ConditionFalse
	}
}

// directiveEnd 返回从 start 开始的指令结束偏移：遇到不属于续行或块注释的换行符为止。
func directiveEnd(content []byte, start int) int {
	n := len(content)
	for i := start; i < n; {
		c := content[i]
		switch {
		case c == '\n':
			if i > start && content[i-1] == '\\' {
				i++
				continue
			}
			if i > start+1 && content[i-1] == '\r' && content[i-2] == '\\' {
				i++
				continue
			}
			end := i
			if end > start && content[end-1] == '\r' {
				end--
			}
			return end
		case c == '/' && i+1 < n && content[i+1] == '*':
			i = blockCommentEnd(content, i)
		case c == '/' && i+1 < n && content[i+1] == '/':
			end := lineCommentEnd(content, i)
			if end > start && end <= n && end > 0 && content[end-1] == '\n' {
				end--
			}
			if end > start && content[end-1] == '\r' {
				end--
			}
			return end
		case c == '"' || c == '\'':
			i = quotedEnd(content, i)
			if i > 0 && content[i-1] == '\n' {
				return i - 1
			}
		default:
			i++
		}
	}
	return n
}

// splitDirective 把以 # 开头的指令文本拆成指令名和其余部分。
func splitDirective(text string) (string, string) {
	rest := strings.TrimLeft(text[1:], " \t")
	name := leadingIdentifier(rest)
	return name, rest[len(name):]
}

// normalizeCondition 合并续行、去掉注释并把连续空白压缩为一个空格。
func normalizeCondition(text string) string {
	text = strings.ReplaceAll(text, "\\\r\n", " ")
	text = strings.ReplaceAll(text, "\\\n", " ")
	var b strings.Builder
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "/*"):
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				i = len(text)
			} else {
				i += end + 4
			}
			b.WriteByte(' ')
		case strings.HasPrefix(text[i:], "//"):
			i = len(text)
		default:
			b.WriteByte(text[i])
			i++
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// leadingIdentifier 返回文本开头的标识符，没有时返回空串。
func leadingIdentifier(text string) string {
	if text == "" || !isIdentStart(text[0]) {
		return ""
	}
	end := 1
	for end < len(text) && isIdentPart(text[end]) {
		end++
	}
	return text[:end]
}

// lineCommentEnd 返回行注释的结束偏移（包含结束换行符），支持反斜杠续行。
func lineCommentEnd(content []byte, i int) int {
	n := len(content)
	for j := i + 2; j < n; j++ {
		if content[j] == '\n' {
			if content[j-1] == '\\' || (j >= 2 && content[j-1] == '\r' && content[j-2] == '\\') {
				continue
			}
			return j + 1
		}
	}
	return n
}

// blockCommentEnd 返回块注释的结束偏移；缺少结束符时到文件末尾。
func blockCommentEnd(content []byte, i int) int {
	n := len(content)
	for j := i + 2; j+1 < n; j++ {
		if content[j] == '*' && content[j+1] == '/' {
			return j + 2
		}
	}
	return n
}

// quotedEnd 返回字符串或字符字面量的结束偏移；遇到未转义的换行符时在换行符之后结束。
func quotedEnd(content []byte, i int) int {
	n := len(content)
	quote := content[i]
	for j := i + 1; j < n; j++ {
		switch content[j] {
		case '\\':
			j++
		case quote:
			return j + 1
		case '\n':
			return j + 1
		}
	}
	return n
}

// rawStringEnd 返回从引号位置 quote 开始的 C++ 原始字符串结束偏移；格式不合法时返回 false。
func rawStringEnd(content []byte, quote int) (int, bool) {
	n := len(content)
	open := quote + 1
	for open < n && open-quote-1 <= 16 && content[open] != '(' {
		switch content[open] {
		case ' ', ')', '\\', '\t', '\n', '\r', '"':
			return 0, false
		}
		open++
	}
	if open >= n || content[open] != '(' {
		return 0, false
	}
	terminator := ")" + string(content[quote+1:open]) + "\""
	idx := strings.Index(string(content[open+1:]), terminator)
	if idx < 0 {
		return n, true
	}
	return open + 1 + idx + len(terminator), true
}

// numberEnd 返回预处理数字的结束偏移，包含指数符号和 C++14 数字分隔符。
func numberEnd(content []byte, i int) int {
	n := len(content)
	j := i + 1
	for j < n {
		c := content[j]
		switch {
		case isIdentPart(c) || c == '.':
			j++
		case (c == '+' || c == '-') && strings.ContainsRune("eEpP", rune(content[j-1])):
			j++
		case c == '\'' && j+1 < n && isIdentPart(content[j+1]):
			j += 2
		default:
			return j
		}
	}
	return j
}

// isDigit 报告字节是否是十进制数字。
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// isIdentStart 报告字节是否可以作为标识符首字符；非 ASCII 字节按标识符处理。
func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

// isIdentPart 报告字节是否可以出现在标识符中。
func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}

// parseInteger 解析带可选 u/U/l/L 后缀和数字分隔符的十进制、十六进制、八进制或二进制整数字面量。
func parseInteger(text string) (int64, bool) {
	text = strings.ReplaceAll(text, "'", "")
	text = strings.TrimRight(text, "uUlL")
	if text == "" {
		return 0, false
	}
	base := 10
	digits := text
	switch {
	case strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X"):
		base, digits = 16, text[2:]
	case strings.HasPrefix(text, "0b") || strings.HasPrefix(text, "0B"):
		base, digits = 2, text[2:]
	case len(text) > 1 && text[0] == '0':
		base, digits = 8, text[1:]
	}
	value, err := strconv.ParseUint(digits, base, 63)
	if err != nil {
		return 0, false
	}
	return int64(value), true
}

// conditionToken 是条件表达式中的一个词法单元。
type conditionToken struct {
	// text 是词法单元文本。
	text string
	// kind 是词法单元种类：ident、number 或 op。
	kind string
}

// tokenizeCondition 把条件文本切分为标识符、数字和运算符；无法识别的字符作为单独的运算符，由求值器判定为未知。
func tokenizeCondition(text string) []conditionToken {
	var tokens []conditionToken
	for i := 0; i < len(text); {
		c := text[i]
		switch {
		case c == ' ':
			i++
		case isIdentStart(c):
			j := i + 1
			for j < len(text) && isIdentPart(text[j]) {
				j++
			}
			tokens = append(tokens, conditionToken{text: text[i:j], kind: "ident"})
			i = j
		case isDigit(c):
			j := numberEnd([]byte(text), i)
			tokens = append(tokens, conditionToken{text: text[i:j], kind: "number"})
			i = j
		default:
			if i+1 < len(text) {
				two := text[i : i+2]
				switch two {
				case "&&", "||", "==", "!=", "<=", ">=":
					tokens = append(tokens, conditionToken{text: two, kind: "op"})
					i += 2
					continue
				}
			}
			tokens = append(tokens, conditionToken{text: text[i : i+1], kind: "op"})
			i++
		}
	}
	return tokens
}

// conditionResult 是条件子表达式的求值结果。
type conditionResult struct {
	// known 表示结果是否确定。
	known bool
	// value 是确定结果的整数值。
	value int64
}

// conditionEvaluator 用递归下降法对条件表达式做三值求值。
type conditionEvaluator struct {
	// tokens 是待求值的词法单元。
	tokens []conditionToken
	// pos 是当前读取位置。
	pos int
	// macros 是本文件已知的宏定义状态。
	macros map[string]macroState
}

// peek 返回当前词法单元文本，读完时返回空串。
func (e *conditionEvaluator) peek() string {
	if e.pos >= len(e.tokens) {
		return ""
	}
	return e.tokens[e.pos].text
}

// parseOr 求值 || 表达式：任一确定为真则为真，全部确定为假则为假，否则未知。
func (e *conditionEvaluator) parseOr() (conditionResult, bool) {
	left, ok := e.parseAnd()
	if !ok {
		return conditionResult{}, false
	}
	for e.peek() == "||" {
		e.pos++
		right, ok := e.parseAnd()
		if !ok {
			return conditionResult{}, false
		}
		switch {
		case (left.known && left.value != 0) || (right.known && right.value != 0):
			left = conditionResult{known: true, value: 1}
		case left.known && right.known:
			left = conditionResult{known: true, value: 0}
		default:
			left = conditionResult{}
		}
	}
	return left, true
}

// parseAnd 求值 && 表达式：任一确定为假则为假，全部确定为真则为真，否则未知。
func (e *conditionEvaluator) parseAnd() (conditionResult, bool) {
	left, ok := e.parseEquality()
	if !ok {
		return conditionResult{}, false
	}
	for e.peek() == "&&" {
		e.pos++
		right, ok := e.parseEquality()
		if !ok {
			return conditionResult{}, false
		}
		switch {
		case (left.known && left.value == 0) || (right.known && right.value == 0):
			left = conditionResult{known: true, value: 0}
		case left.known && right.known:
			left = conditionResult{known: true, value: 1}
		default:
			left = conditionResult{}
		}
	}
	return left, true
}

// parseEquality 求值 == 和 != 表达式，两侧都确定时结果才确定。
func (e *conditionEvaluator) parseEquality() (conditionResult, bool) {
	left, ok := e.parseRelational()
	if !ok {
		return conditionResult{}, false
	}
	for e.peek() == "==" || e.peek() == "!=" {
		op := e.peek()
		e.pos++
		right, ok := e.parseRelational()
		if !ok {
			return conditionResult{}, false
		}
		left = compare(left, right, op)
	}
	return left, true
}

// parseRelational 求值 <、>、<=、>= 表达式，两侧都确定时结果才确定。
func (e *conditionEvaluator) parseRelational() (conditionResult, bool) {
	left, ok := e.parseUnary()
	if !ok {
		return conditionResult{}, false
	}
	for {
		op := e.peek()
		if op != "<" && op != ">" && op != "<=" && op != ">=" {
			return left, true
		}
		e.pos++
		right, ok := e.parseUnary()
		if !ok {
			return conditionResult{}, false
		}
		left = compare(left, right, op)
	}
}

// compare 对两个结果做比较运算，任一侧未知时结果未知。
func compare(left, right conditionResult, op string) conditionResult {
	if !left.known || !right.known {
		return conditionResult{}
	}
	var result bool
	switch op {
	case "==":
		result = left.value == right.value
	case "!=":
		result = left.value != right.value
	case "<":
		result = left.value < right.value
	case ">":
		result = left.value > right.value
	case "<=":
		result = left.value <= right.value
	case ">=":
		result = left.value >= right.value
	}
	if result {
		return conditionResult{known: true, value: 1}
	}
	return conditionResult{known: true, value: 0}
}

// parseUnary 求值逻辑非和基本表达式。
func (e *conditionEvaluator) parseUnary() (conditionResult, bool) {
	if e.peek() == "!" {
		e.pos++
		inner, ok := e.parseUnary()
		if !ok {
			return conditionResult{}, false
		}
		if !inner.known {
			return conditionResult{}, true
		}
		if inner.value == 0 {
			return conditionResult{known: true, value: 1}, true
		}
		return conditionResult{known: true, value: 0}, true
	}
	return e.parsePrimary()
}

// parsePrimary 求值数字、defined、标识符和括号表达式；其他写法视为无法解析。
func (e *conditionEvaluator) parsePrimary() (conditionResult, bool) {
	if e.pos >= len(e.tokens) {
		return conditionResult{}, false
	}
	token := e.tokens[e.pos]
	e.pos++
	switch {
	case token.kind == "number":
		value, ok := parseInteger(token.text)
		if !ok {
			return conditionResult{}, false
		}
		return conditionResult{known: true, value: value}, true
	case token.text == "defined":
		parenthesized := e.peek() == "("
		if parenthesized {
			e.pos++
		}
		if e.pos >= len(e.tokens) || e.tokens[e.pos].kind != "ident" {
			return conditionResult{}, false
		}
		name := e.tokens[e.pos].text
		e.pos++
		if parenthesized {
			if e.peek() != ")" {
				return conditionResult{}, false
			}
			e.pos++
		}
		state, known := e.macros[name]
		if !known {
			return conditionResult{}, true
		}
		if state.defined {
			return conditionResult{known: true, value: 1}, true
		}
		return conditionResult{known: true, value: 0}, true
	case token.kind == "ident":
		if e.peek() == "(" {
			return conditionResult{}, false
		}
		state, known := e.macros[token.text]
		switch {
		case known && !state.defined:
			return conditionResult{known: true, value: 0}, true
		case known && state.hasValue:
			return conditionResult{known: true, value: state.value}, true
		default:
			return conditionResult{}, true
		}
	case token.text == "(":
		inner, ok := e.parseOr()
		if !ok || e.peek() != ")" {
			return conditionResult{}, false
		}
		e.pos++
		return inner, true
	default:
		return conditionResult{}, false
	}
}
