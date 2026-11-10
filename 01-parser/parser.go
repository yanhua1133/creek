package parser

import (
	"context"
	"time"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// treeParser 是 Parser 接口的默认实现，每次解析使用独立的底层 Parser 实例以保证并发安全。
type treeParser struct{}

// New 返回一个 Parser 实现。返回的实例可被多个 goroutine 并发调用 Parse。
func New() Parser {
	return &treeParser{}
}

// Parse 解析指定源码文件：根据扩展名选择单 grammar 解析或 .h 双 grammar 择优解析。
func (p *treeParser) Parse(ctx context.Context, file SourceFile, options ParseOptions) (ParseResult, error) {
	langs, err := candidateLanguages(file.Path)
	if err != nil {
		return ParseResult{}, err
	}
	if len(langs) == 1 {
		return p.parseSingle(ctx, file, langs[0], options)
	}
	return p.parseHeader(ctx, file, options)
}

// rawParse 用指定语言解析源码内容，返回底层树及其错误节点统计。
// 语法错误不作为 error，error 仅表示 grammar 不兼容、上下文取消或无法生成树等系统性失败。
func rawParse(ctx context.Context, content []byte, lang Language) (tree *ts.Tree, count int, coveredBytes uint, err error) {
	// 上下文已取消时立即返回，不进入解析。
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, 0, 0, ctxErr
	}
	g := grammarFor(lang)
	if g == nil {
		return nil, 0, 0, ErrIncompatibleGrammar
	}
	tsp := ts.NewParser()
	if setErr := tsp.SetLanguage(g); setErr != nil {
		tsp.Close()
		return nil, 0, 0, ErrIncompatibleGrammar
	}
	// 设置非空取消标志，供解析过程中响应上下文取消；同时规避 ParseCtx 在标志为 nil 时的解引用问题。
	tsp.SetCancellationFlag(new(uintptr))
	tree = tsp.ParseCtx(ctx, content, nil)
	tsp.Close()
	if tree == nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, 0, 0, ctxErr
		}
		return nil, 0, 0, ErrEmptyTree
	}
	count, coveredBytes = errorStats(tree)
	return tree, count, coveredBytes, nil
}

// preparedView 保存一次解析实际交给 Tree-sitter 的源码及其预处理报告和耗时。
type preparedView struct {
	// content 是交给 Tree-sitter 的源码，与原文件等长。
	content []byte
	// preprocess 是条件编译分支选择报告，未做分支选择时为空。
	preprocess *PreprocessReport
	// macros 是宏中和报告，未做宏中和时为空。
	macros *MacroReport
	// metrics 记录预处理各环节耗时，只在 CollectMetrics 为真时填写。
	metrics ParseMetrics
}

// parseView 返回指定语言实际交给 Tree-sitter 的源码：C/C++ 默认依次做条件编译分支选择和宏中和，其余为原始源码。
func parseView(file SourceFile, lang Language, options ParseOptions) preparedView {
	if options.RawSource || (lang != LanguageC && lang != LanguageCPP) {
		return preparedView{content: file.Content}
	}
	var started time.Time
	if options.CollectMetrics {
		started = time.Now()
	}
	view, report := BuildConditionalView(file.Content, lang)
	prepared := preparedView{content: view, preprocess: &report}
	if options.CollectMetrics {
		prepared.metrics.Conditional = time.Since(started)
		started = time.Now()
	}
	if options.ConditionalOnly {
		return prepared
	}
	macroView, macroReport := BuildMacroView(view, file.Path, lang, options.Macros)
	prepared.content = macroView
	prepared.macros = &macroReport
	if options.CollectMetrics {
		prepared.metrics.Macro = time.Since(started)
	}
	return prepared
}

// timedParse 解析预处理后的源码，CollectMetrics 为真时把 Tree-sitter 耗时记入 metrics。
func timedParse(ctx context.Context, prepared *preparedView, lang Language, options ParseOptions) (*ts.Tree, int, uint, error) {
	var started time.Time
	if options.CollectMetrics {
		started = time.Now()
	}
	tree, count, coveredBytes, err := rawParse(ctx, prepared.content, lang)
	if options.CollectMetrics {
		prepared.metrics.TreeSitter = time.Since(started)
	}
	return tree, count, coveredBytes, err
}

// parseSingle 用单一 grammar 解析并包装结果。
func (p *treeParser) parseSingle(ctx context.Context, file SourceFile, lang Language, options ParseOptions) (ParseResult, error) {
	prepared := parseView(file, lang, options)
	tree, _, _, err := timedParse(ctx, &prepared, lang, options)
	if err != nil {
		return ParseResult{}, err
	}
	return p.buildResult(file, lang, tree, options, nil, prepared, prepared.metrics), nil
}

// parseHeader 对 .h 头文件分别用 C 与 C++ grammar 解析，按错误节点数、错误覆盖字节、C++ 优先的固定规则择优。
func (p *treeParser) parseHeader(ctx context.Context, file SourceFile, options ParseOptions) (ParseResult, error) {
	cPrepared := parseView(file, LanguageC, options)
	cTree, cCount, cBytes, cErr := timedParse(ctx, &cPrepared, LanguageC, options)
	if isContextErr(cErr) {
		if cTree != nil {
			cTree.Close()
		}
		return ParseResult{}, cErr
	}
	cppPrepared := parseView(file, LanguageCPP, options)
	cppTree, cppCount, cppBytes, cppErr := timedParse(ctx, &cppPrepared, LanguageCPP, options)
	if isContextErr(cppErr) {
		if cTree != nil {
			cTree.Close()
		}
		if cppTree != nil {
			cppTree.Close()
		}
		return ParseResult{}, cppErr
	}
	metrics := ParseMetrics{
		Conditional: cPrepared.metrics.Conditional + cppPrepared.metrics.Conditional,
		Macro:       cPrepared.metrics.Macro + cppPrepared.metrics.Macro,
		TreeSitter:  cPrepared.metrics.TreeSitter + cppPrepared.metrics.TreeSitter,
	}
	// 两次解析都系统性失败时报头文件解析失败。
	if cErr != nil && cppErr != nil {
		return ParseResult{}, ErrHeaderParseFailed
	}
	// 其中一次系统性失败时直接采用成功的一次。
	if cErr != nil {
		return p.buildResult(file, LanguageCPP, cppTree, options,
			&HeaderResolution{CPPErrorCount: cppCount, CPPErrorBytes: cppBytes, Chosen: LanguageCPP}, cppPrepared, metrics), nil
	}
	if cppErr != nil {
		return p.buildResult(file, LanguageC, cTree, options,
			&HeaderResolution{CErrorCount: cCount, CErrorBytes: cBytes, Chosen: LanguageC}, cPrepared, metrics), nil
	}
	res := &HeaderResolution{
		CErrorCount:   cCount,
		CPPErrorCount: cppCount,
		CErrorBytes:   cBytes,
		CPPErrorBytes: cppBytes,
	}
	chosen := chooseHeaderLanguage(cCount, cppCount, cBytes, cppBytes)
	res.Chosen = chosen
	if chosen == LanguageC {
		cppTree.Close()
		return p.buildResult(file, LanguageC, cTree, options, res, cPrepared, metrics), nil
	}
	cTree.Close()
	return p.buildResult(file, LanguageCPP, cppTree, options, res, cppPrepared, metrics), nil
}

// chooseHeaderLanguage 按固定规则在 C 与 C++ 之间择优：错误节点数少者胜；相等则错误覆盖字节少者胜；仍相等选 C++。
func chooseHeaderLanguage(cCount, cppCount int, cBytes, cppBytes uint) Language {
	if cCount != cppCount {
		if cCount < cppCount {
			return LanguageC
		}
		return LanguageCPP
	}
	if cBytes != cppBytes {
		if cBytes < cppBytes {
			return LanguageC
		}
		return LanguageCPP
	}
	return LanguageCPP
}

// buildResult 把选中的底层树包装为 SyntaxTree 并按需收集诊断，组装成解析结果；语法树保留原始源码供节点取文本，有宏改写时另存解析视图。
func (p *treeParser) buildResult(file SourceFile, lang Language, tree *ts.Tree, options ParseOptions, header *HeaderResolution, prepared preparedView, metrics ParseMetrics) ParseResult {
	st := &syntaxTree{
		lang:    lang,
		fileID:  file.ID,
		path:    file.Path,
		content: file.Content,
		tree:    tree,
	}
	if prepared.macros != nil && len(prepared.macros.Rewrites) > 0 {
		st.view = prepared.content
		st.rewrites = prepared.macros.Rewrites
	}
	var diags []Diagnostic
	if options.CollectDiagnostics {
		diags = collectDiagnostics(tree)
	}
	result := ParseResult{
		Language:    lang,
		Tree:        st,
		Diagnostics: diags,
		Header:      header,
		Preprocess:  prepared.preprocess,
		Macros:      prepared.macros,
	}
	if options.CollectMetrics {
		result.Metrics = &metrics
	}
	return result
}

// isContextErr 报告错误是否由上下文取消或超时引起。
func isContextErr(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}
