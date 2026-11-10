package parser

import (
	"path"
	"sort"
	"strings"
	"time"
)

// MacroIndex 是项目级宏定义索引，按宏名保存各头文件中的定义，构建后只读，可被并发解析共享。
type MacroIndex struct {
	// definitions 按宏名保存全部收录的定义，同名定义按文件路径和源码顺序排列。
	definitions map[string][]indexedMacro
}

// indexedMacro 是索引中的一条宏定义及其在 C、C++ 两种规则下的条件编译状态。
type indexedMacro struct {
	// def 是解析后的宏定义。
	def *macroDef
	// dir 是定义文件所在目录按 / 拆分后的各级名称，用于计算目录距离。
	dir []string
	// reachable 按语言下标（0 为 C，1 为 C++）记录定义是否可达。
	reachable [2]bool
	// taken 按语言下标记录定义是否位于分支选择保留的区域。
	taken [2]bool
}

// macroDef 是解析后的一条 #define。
type macroDef struct {
	// name 是宏名。
	name string
	// functionLike 表示是否为函数式宏。
	functionLike bool
	// params 是函数式宏的形参名，可变参数记为 __VA_ARGS__ 或其具名形式。
	params []string
	// body 是去掉注释和续行后的定义体记号。
	body []ppToken
	// path 是定义所在文件的项目内相对路径。
	path string
	// line 是定义所在行号，从 1 开始。
	line int
}

// macroCandidate 是某个宏名在当前位置的一条候选定义。
type macroCandidate struct {
	// def 是候选宏定义。
	def *macroDef
	// fallback 表示该定义未被分支选择保留，只是可达；函数式宏不使用此类定义。
	fallback bool
}

// ppToken 是宏定义体中的一个预处理记号。
type ppToken struct {
	// text 是记号文本。
	text string
	// ident 表示记号是否为标识符。
	ident bool
}

// macroKeyword 是宏展开后保留下来的一个关键字。
type macroKeyword struct {
	// text 是关键字文本。
	text string
	// optional 表示关键字放不下时可以省略。
	optional bool
}

// MacroRewrite 记录一次宏调用改写。
type MacroRewrite struct {
	// Range 是被改写的宏调用范围，函数式宏包含参数和右括号。
	Range SourceRange
	// Name 是宏名。
	Name string
	// Replacement 是写在调用起始处的关键字文本，为空表示整个调用替换为空格。
	Replacement string
	// DefinitionPath 是生效定义所在文件的项目内相对路径。
	DefinitionPath string
	// DefinitionLine 是生效定义所在的行号，从 1 开始。
	DefinitionLine int
}

// MacroReport 记录一次宏中和的结果。
type MacroReport struct {
	// Rewrites 是按源码顺序排列的全部改写。
	Rewrites []MacroRewrite
	// Unsupported 是找到定义、但展开含不可改写记号而未改写的宏调用数。
	Unsupported int
	// TooLong 是展开可以改写、但关键字放不下而未改写的宏调用数。
	TooLong int
}

// ParseMetrics 记录一次解析中各处理环节的耗时，.h 文件为两次解析之和。
type ParseMetrics struct {
	// Conditional 是条件编译分支选择耗时。
	Conditional time.Duration
	// Macro 是宏中和耗时。
	Macro time.Duration
	// TreeSitter 是 Tree-sitter 解析耗时。
	TreeSitter time.Duration
}

// maxMacroDepth 是宏递归展开的最大深度。
const maxMacroDepth = 16

// macroDefinitionExts 是宏定义索引收录的头文件扩展名。
var macroDefinitionExts = map[string]bool{
	".h": true, ".hh": true, ".hpp": true, ".hxx": true, ".inc": true, ".inl": true, ".ipp": true, ".tpp": true, ".def": true,
}

// reservedWords 是 C/C++ 关键字和编译器内建关键字，同名宏一律不改写。
var reservedWords = wordSet(
	"auto break case char const continue default do double else enum extern float for goto if inline int long "+
		"register restrict return short signed sizeof static struct switch typedef union unsigned void volatile while "+
		"_Alignas _Alignof _Atomic _Bool _Complex _Generic _Imaginary _Noreturn _Static_assert _Thread_local",
	"alignas alignof and and_eq asm bitand bitor bool catch char8_t char16_t char32_t class compl concept const_cast "+
		"consteval constexpr constinit co_await co_return co_yield decltype delete dynamic_cast explicit export false final "+
		"friend mutable namespace new noexcept not not_eq nullptr operator or or_eq override private protected public "+
		"reinterpret_cast requires static_assert static_cast template this thread_local throw true try typeid typename "+
		"using virtual wchar_t xor xor_eq",
	"__attribute__ __attribute __declspec __inline __inline__ __forceinline __restrict __restrict__ __thread "+
		"__extension__ __asm__ __asm __volatile__ __typeof__ __typeof typeof __alignof__ __const __signed__",
)

// retainedKeywords 按语言下标保存宏展开中必须保留的说明符关键字。
var retainedKeywords = [2]map[string]bool{
	wordSet("static extern const volatile register restrict __restrict __restrict__ _Noreturn _Thread_local __thread"),
	wordSet("static extern const volatile register restrict __restrict __restrict__ _Noreturn _Thread_local __thread",
		"constexpr consteval constinit virtual explicit final override noexcept thread_local mutable"),
}

// optionalKeywords 是宏展开中放不下时可以省略的关键字。
var optionalKeywords = wordSet("inline __inline __inline__ __forceinline")

// wordSet 把若干以空格分隔的单词列表合并为集合。
func wordSet(lists ...string) map[string]bool {
	set := map[string]bool{}
	for _, list := range lists {
		for _, word := range strings.Fields(list) {
			set[word] = true
		}
	}
	return set
}

// langSlot 返回语言在按语言下标保存的数组中的位置：C 为 0，其余为 1。
func langSlot(lang Language) int {
	if lang == LanguageC {
		return 0
	}
	return 1
}

// IsMacroDefinitionFile 报告路径是否属于宏定义索引收录的头文件类型。
func IsMacroDefinitionFile(filePath string) bool {
	return macroDefinitionExts[strings.ToLower(path.Ext(filePath))]
}

// BuildMacroIndex 扫描项目头文件中的 #define，建立按目录距离查找的宏定义索引；非宏定义文件被忽略。
// 每个头文件分别按 C 与 C++ 规则做条件编译扫描，记录定义是否可达、是否保留；定义后又在可达区域被 #undef 的宏不收录。
func BuildMacroIndex(files []SourceFile) *MacroIndex {
	index := &MacroIndex{definitions: map[string][]indexedMacro{}}
	for _, file := range files {
		if !IsMacroDefinitionFile(file.Path) {
			continue
		}
		for _, entry := range indexFile(file) {
			index.definitions[entry.def.name] = append(index.definitions[entry.def.name], entry)
		}
	}
	for name, entries := range index.definitions {
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].def.path != entries[j].def.path {
				return entries[i].def.path < entries[j].def.path
			}
			return entries[i].def.line < entries[j].def.line
		})
		index.definitions[name] = entries
	}
	return index
}

// indexFile 返回单个头文件中可收录的宏定义，同一条定义合并 C 与 C++ 两种扫描结果。
func indexFile(file SourceFile) []indexedMacro {
	byLine := map[int]*indexedMacro{}
	var order []int
	dir := dirParts(file.Path)
	for _, lang := range []Language{LanguageC, LanguageCPP} {
		slot := langSlot(lang)
		events := scanDefines(file.Content, lang)
		lastUndef := lastUndefs(events)
		for i, event := range events {
			if event.keyword != "define" {
				continue
			}
			def := parseDefine(event.rest, file.Path, event.line)
			if def == nil {
				continue
			}
			if last, ok := lastUndef[def.name]; ok && last > i {
				continue
			}
			entry, ok := byLine[event.line]
			if !ok {
				entry = &indexedMacro{def: def, dir: dir}
				byLine[event.line] = entry
				order = append(order, event.line)
			}
			entry.reachable[slot] = true
			entry.taken[slot] = event.taken
		}
	}
	sort.Ints(order)
	entries := make([]indexedMacro, 0, len(order))
	for _, line := range order {
		entries = append(entries, *byLine[line])
	}
	return entries
}

// lastUndefs 返回每个宏名最后一次被 #undef 的指令下标。
func lastUndefs(events []defineEvent) map[string]int {
	last := map[string]int{}
	for i, event := range events {
		if event.keyword != "undef" {
			continue
		}
		if name := leadingIdentifier(strings.TrimSpace(normalizeCondition(event.rest))); name != "" {
			last[name] = i
		}
	}
	return last
}

// dirParts 返回文件所在目录按 / 拆分后的各级名称，项目根目录为空切片。
func dirParts(filePath string) []string {
	dir := path.Dir(filePath)
	if dir == "." || dir == "/" || dir == "" {
		return nil
	}
	return strings.Split(dir, "/")
}

// dirDistance 返回两个目录之间需要上溯和下行的目录层数之和。
func dirDistance(a, b []string) int {
	common := 0
	for common < len(a) && common < len(b) && a[common] == b[common] {
		common++
	}
	return len(a) + len(b) - 2*common
}

// lookup 返回文件 filePath 中宏 name 的候选定义：在其他头文件中选目录距离最小、距离相同时路径字典序最小的文件，
// 先返回该文件中保留的定义，再返回其余可达定义（标记为回退定义），各自按源码顺序排列。
func (index *MacroIndex) lookup(name, filePath string, dir []string, slot int) []macroCandidate {
	if index == nil {
		return nil
	}
	entries := index.definitions[name]
	bestPath := ""
	bestDistance := -1
	for i := range entries {
		entry := &entries[i]
		if entry.def.path == filePath || !entry.reachable[slot] {
			continue
		}
		distance := dirDistance(dir, entry.dir)
		if bestDistance < 0 || distance < bestDistance || (distance == bestDistance && entry.def.path < bestPath) {
			bestDistance = distance
			bestPath = entry.def.path
		}
	}
	if bestDistance < 0 {
		return nil
	}
	var taken, others []macroCandidate
	for i := range entries {
		entry := &entries[i]
		if entry.def.path != bestPath || !entry.reachable[slot] {
			continue
		}
		if entry.taken[slot] {
			taken = append(taken, macroCandidate{def: entry.def})
		} else {
			others = append(others, macroCandidate{def: entry.def, fallback: true})
		}
	}
	return append(taken, others...)
}

// parseDefine 解析 #define 之后的文本，返回宏定义；无法识别宏名或形参时返回 nil。
func parseDefine(rest, filePath string, line int) *macroDef {
	text := strings.ReplaceAll(strings.ReplaceAll(rest, "\\\r\n", " "), "\\\n", " ")
	text = strings.TrimLeft(text, " \t")
	name := leadingIdentifier(text)
	if name == "" {
		return nil
	}
	def := &macroDef{name: name, path: filePath, line: line}
	after := text[len(name):]
	if strings.HasPrefix(after, "(") {
		closing := strings.IndexByte(after, ')')
		if closing < 0 {
			return nil
		}
		def.functionLike = true
		for _, param := range strings.Split(after[1:closing], ",") {
			param = strings.TrimSpace(param)
			switch {
			case param == "" && strings.TrimSpace(after[1:closing]) == "":
				continue
			case param == "":
				return nil
			case param == "...":
				param = "__VA_ARGS__"
			case strings.HasSuffix(param, "..."):
				param = strings.TrimSpace(strings.TrimSuffix(param, "..."))
			}
			if leadingIdentifier(param) != param {
				return nil
			}
			def.params = append(def.params, param)
		}
		after = after[closing+1:]
	}
	def.body = ppTokenize(after)
	return def
}

// ppTokenize 把宏定义体切分为预处理记号，去掉空白和注释，字符串和字符字面量作为整体。
func ppTokenize(text string) []ppToken {
	var tokens []ppToken
	content := []byte(text)
	for i := 0; i < len(content); {
		c := content[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v':
			i++
		case c == '/' && i+1 < len(content) && (content[i+1] == '/' || content[i+1] == '*'):
			i = tokenEnd(content, i, false)
		default:
			end := tokenEnd(content, i, false)
			tokens = append(tokens, ppToken{text: text[i:end], ident: isIdentStart(c) && !strings.ContainsAny(text[i:end], "\"'")})
			i = end
		}
	}
	return tokens
}

// BuildMacroView 在条件编译视图上做宏中和，返回与原文件等长的解析视图和报告；index 可以为空，此时只使用本文件内的定义。
func BuildMacroView(view []byte, filePath string, lang Language, index *MacroIndex) ([]byte, MacroReport) {
	r := &macroRewriter{
		in:         view,
		out:        append([]byte(nil), view...),
		path:       filePath,
		dir:        dirParts(filePath),
		lang:       lang,
		slot:       langSlot(lang),
		index:      index,
		local:      map[string]*macroDef{},
		lineStarts: []int{0},
		global:     map[string][]macroCandidate{},
	}
	for i, c := range view {
		if c == '\n' {
			r.lineStarts = append(r.lineStarts, i+1)
		}
	}
	r.run()
	return r.out, r.report
}

// macroRewriter 按词法扫描条件编译视图，识别可中和的宏调用并原位改写。
type macroRewriter struct {
	// in 是输入的条件编译视图。
	in []byte
	// out 是正在生成的宏中和视图。
	out []byte
	// path 是当前文件的项目内相对路径。
	path string
	// dir 是当前文件所在目录的各级名称。
	dir []string
	// lang 是本次使用的 grammar 对应语言。
	lang Language
	// slot 是语言在按语言下标保存的数组中的位置。
	slot int
	// index 是项目级宏定义索引，可以为空。
	index *MacroIndex
	// local 保存本文件中此前已保留的定义；值为 nil 表示已被 #undef。
	local map[string]*macroDef
	// lineStarts 保存每行首字节偏移，用于把偏移换算为行列位置。
	lineStarts []int
	// global 缓存索引查找结果，键为宏名。
	global map[string][]macroCandidate
	// report 是正在生成的报告。
	report MacroReport
}

// run 从头到尾扫描视图：处理行首指令以维护本文件宏表，对代码中的标识符尝试宏中和。
func (r *macroRewriter) run() {
	content := r.in
	n := len(content)
	cpp := r.lang == LanguageCPP
	lineStart := true
	prev, prevPrev := -1, -1
	for i := 0; i < n; {
		c := content[i]
		switch {
		case c == '\n':
			lineStart = true
			i++
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
			continue
		case c == '#' && lineStart:
			end := directiveEnd(content, i)
			r.directive(string(content[i:end]), i)
			i = end
			continue
		}
		lineStart = false
		end := tokenEnd(content, i, cpp)
		if c == '/' && i+1 < n && (content[i+1] == '/' || content[i+1] == '*') {
			if content[end-1] == '\n' {
				lineStart = true
			}
			i = end
			continue
		}
		if isIdentStart(c) && !strings.ContainsAny(string(content[i:end]), "\"'") && !r.qualified(prev, prevPrev, end) {
			if next := r.identifier(i, end); next > end {
				prev, prevPrev = next-1, -1
				i = next
				continue
			}
		}
		prev, prevPrev = i, prev
		if content[end-1] == '\n' {
			lineStart = true
		}
		i = end
	}
}

// directive 处理一条保留的预处理指令：#define 记入本文件宏表，#undef 把宏标记为未定义。
func (r *macroRewriter) directive(text string, start int) {
	keyword, rest := splitDirective(text)
	switch keyword {
	case "define":
		if def := parseDefine(rest, r.path, int(r.point(start).Row)+1); def != nil {
			r.local[def.name] = def
		}
	case "undef":
		if name := leadingIdentifier(strings.TrimSpace(normalizeCondition(rest))); name != "" {
			r.local[name] = nil
		}
	}
}

// qualified 报告从 prev 结束的标识符是否是成员或限定名：前面是 .、->、::，或后面是 ::。
func (r *macroRewriter) qualified(prev, prevPrev, end int) bool {
	content := r.in
	if prev >= 0 {
		switch content[prev] {
		case '.':
			return true
		case '>':
			if prevPrev >= 0 && prevPrev == prev-1 && content[prevPrev] == '-' {
				return true
			}
		case ':':
			if prevPrev >= 0 && prevPrev == prev-1 && content[prevPrev] == ':' {
				return true
			}
		}
	}
	next := r.skipSpace(end)
	return next+1 < len(content) && content[next] == ':' && content[next+1] == ':'
}

// skipSpace 返回从 i 开始跳过空白和注释后的第一个字节偏移。
func (r *macroRewriter) skipSpace(i int) int {
	content := r.in
	for i < len(content) {
		c := content[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v':
			i++
		case c == '/' && i+1 < len(content) && (content[i+1] == '/' || content[i+1] == '*'):
			i = tokenEnd(content, i, false)
		default:
			return i
		}
	}
	return i
}

// candidates 返回宏名在当前位置可用的候选定义：本文件定义优先，已被 #undef 时返回空，否则查索引；
// 函数式宏只使用分支选择保留的定义。
func (r *macroRewriter) candidates(name string) []*macroDef {
	if reservedWords[name] {
		return nil
	}
	if def, ok := r.local[name]; ok {
		if def == nil {
			return nil
		}
		return []*macroDef{def}
	}
	if r.index == nil {
		return nil
	}
	found, ok := r.global[name]
	if !ok {
		found = r.index.lookup(name, r.path, r.dir, r.slot)
		r.global[name] = found
	}
	var defs []*macroDef
	for _, candidate := range found {
		if candidate.fallback && candidate.def.functionLike {
			continue
		}
		defs = append(defs, candidate.def)
	}
	return defs
}

// identifier 尝试中和从 start 到 end 的标识符对应的宏调用，返回改写范围的结束偏移；未改写时返回 end。
func (r *macroRewriter) identifier(start, end int) int {
	name := string(r.in[start:end])
	defs := r.candidates(name)
	if len(defs) == 0 {
		return end
	}
	applicable, tooLong := false, false
	for _, def := range defs {
		spanEnd := end
		if def.functionLike {
			closing, ok := r.invocationEnd(end)
			if !ok {
				continue
			}
			spanEnd = closing
		}
		applicable = true
		keywords, ok := r.expand(def, 0, map[string]bool{name: true})
		if !ok {
			continue
		}
		text, fits := fitKeywords(keywords, r.firstLineLength(start, spanEnd))
		if !fits {
			tooLong = true
			continue
		}
		r.write(start, spanEnd, text)
		r.report.Rewrites = append(r.report.Rewrites, MacroRewrite{
			Range:          r.sourceRange(start, spanEnd),
			Name:           name,
			Replacement:    text,
			DefinitionPath: def.path,
			DefinitionLine: def.line,
		})
		return spanEnd
	}
	switch {
	case tooLong:
		r.report.TooLong++
	case applicable:
		r.report.Unsupported++
	}
	return end
}

// invocationEnd 返回从 end 开始的函数式宏实参列表的结束偏移（右括号之后）；没有括号、括号不配对或跨越指令行时返回 false。
func (r *macroRewriter) invocationEnd(end int) (int, bool) {
	content := r.in
	i := r.skipSpace(end)
	if i >= len(content) || content[i] != '(' {
		return 0, false
	}
	depth := 0
	cpp := r.lang == LanguageCPP
	for i < len(content) {
		c := content[i]
		switch {
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		case c == '#' && r.atLineStart(i):
			return 0, false
		}
		i = tokenEnd(content, i, cpp)
	}
	return 0, false
}

// atLineStart 报告偏移 i 之前同一行是否只有空白。
func (r *macroRewriter) atLineStart(i int) bool {
	for j := i - 1; j >= 0; j-- {
		switch r.in[j] {
		case '\n':
			return true
		case ' ', '\t', '\r', '\f', '\v':
		default:
			return false
		}
	}
	return true
}

// expand 递归展开宏定义体：丢弃属性，收集说明符关键字；遇到其他记号或超过深度上限时返回 false。
func (r *macroRewriter) expand(def *macroDef, depth int, visiting map[string]bool) ([]macroKeyword, bool) {
	if depth > maxMacroDepth {
		return nil, false
	}
	params := map[string]bool{}
	for _, param := range def.params {
		params[param] = true
	}
	var keywords []macroKeyword
	tokens := def.body
	for i := 0; i < len(tokens); {
		token := tokens[i]
		switch {
		case (token.text == "__attribute__" || token.text == "__attribute" || token.text == "__declspec") && i+1 < len(tokens) && tokens[i+1].text == "(":
			i = skipGroup(tokens, i+1, "(", ")")
		case token.text == "[" && i+1 < len(tokens) && tokens[i+1].text == "[":
			i = skipGroup(tokens, i, "[", "]")
		case !token.ident || params[token.text]:
			return nil, false
		case retainedKeywords[r.slot][token.text]:
			keywords = append(keywords, macroKeyword{text: token.text})
			i++
		case optionalKeywords[token.text]:
			keywords = append(keywords, macroKeyword{text: token.text, optional: true})
			i++
		default:
			nested, next, ok := r.expandNested(tokens, i, depth, visiting)
			if !ok {
				return nil, false
			}
			keywords = append(keywords, nested...)
			i = next
		}
	}
	return keywords, true
}

// expandNested 展开定义体中位于 i 的宏名：对象式宏直接展开，函数式宏要求后跟括号并跳过实参；返回关键字和下一个记号位置。
func (r *macroRewriter) expandNested(tokens []ppToken, i, depth int, visiting map[string]bool) ([]macroKeyword, int, bool) {
	name := tokens[i].text
	if visiting[name] {
		return nil, 0, false
	}
	visiting[name] = true
	defer delete(visiting, name)
	for _, def := range r.candidates(name) {
		next := i + 1
		if def.functionLike {
			if next >= len(tokens) || tokens[next].text != "(" {
				continue
			}
			next = skipGroup(tokens, next, "(", ")")
		}
		if keywords, ok := r.expand(def, depth+1, visiting); ok {
			return keywords, next, true
		}
	}
	return nil, 0, false
}

// skipGroup 从 tokens[i]（必须是 open）开始跳过配对的括号组，返回组后第一个记号位置；不配对时返回记号总数。
func skipGroup(tokens []ppToken, i int, open, close string) int {
	depth := 0
	for ; i < len(tokens); i++ {
		switch tokens[i].text {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(tokens)
}

// fitKeywords 把关键字用空格连接后放入 avail 字节：放不下时先去掉可省略关键字，仍放不下返回 false。
func fitKeywords(keywords []macroKeyword, avail int) (string, bool) {
	var all, required []string
	for _, keyword := range keywords {
		all = append(all, keyword.text)
		if !keyword.optional {
			required = append(required, keyword.text)
		}
	}
	if text := strings.Join(all, " "); len(text) <= avail {
		return text, true
	}
	if text := strings.Join(required, " "); len(text) <= avail {
		return text, true
	}
	return "", false
}

// firstLineLength 返回 [start,end) 中第一个换行符之前的字节数。
func (r *macroRewriter) firstLineLength(start, end int) int {
	for i := start; i < end; i++ {
		if r.in[i] == '\n' || r.in[i] == '\r' {
			return i - start
		}
	}
	return end - start
}

// write 把 text 写在 start 处，并把 [start,end) 中其余非换行字节替换为空格。
func (r *macroRewriter) write(start, end int, text string) {
	for i := start; i < end; i++ {
		if r.in[i] == '\n' || r.in[i] == '\r' {
			continue
		}
		r.out[i] = ' '
	}
	copy(r.out[start:], text)
}

// point 把字节偏移换算为零基行列位置。
func (r *macroRewriter) point(offset int) SourcePoint {
	row := sort.Search(len(r.lineStarts), func(i int) bool { return r.lineStarts[i] > offset }) - 1
	return SourcePoint{Row: uint(row), Column: uint(offset - r.lineStarts[row])}
}

// sourceRange 构造 [start,end) 对应的源码范围。
func (r *macroRewriter) sourceRange(start, end int) SourceRange {
	return SourceRange{StartByte: uint(start), EndByte: uint(end), StartPoint: r.point(start), EndPoint: r.point(end)}
}
