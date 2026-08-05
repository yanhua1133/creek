package parser

import (
	"context"

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

// parseSingle 用单一 grammar 解析并包装结果。
func (p *treeParser) parseSingle(ctx context.Context, file SourceFile, lang Language, options ParseOptions) (ParseResult, error) {
	tree, _, _, err := rawParse(ctx, file.Content, lang)
	if err != nil {
		return ParseResult{}, err
	}
	return p.buildResult(file, lang, tree, options, nil), nil
}

// parseHeader 对 .h 头文件分别用 C 与 C++ grammar 解析，按错误节点数、错误覆盖字节、C++ 优先的固定规则择优。
func (p *treeParser) parseHeader(ctx context.Context, file SourceFile, options ParseOptions) (ParseResult, error) {
	cTree, cCount, cBytes, cErr := rawParse(ctx, file.Content, LanguageC)
	if isContextErr(cErr) {
		if cTree != nil {
			cTree.Close()
		}
		return ParseResult{}, cErr
	}
	cppTree, cppCount, cppBytes, cppErr := rawParse(ctx, file.Content, LanguageCPP)
	if isContextErr(cppErr) {
		if cTree != nil {
			cTree.Close()
		}
		if cppTree != nil {
			cppTree.Close()
		}
		return ParseResult{}, cppErr
	}
	// 两次解析都系统性失败时报头文件解析失败。
	if cErr != nil && cppErr != nil {
		return ParseResult{}, ErrHeaderParseFailed
	}
	// 其中一次系统性失败时直接采用成功的一次。
	if cErr != nil {
		return p.buildResult(file, LanguageCPP, cppTree, options,
			&HeaderResolution{CPPErrorCount: cppCount, CPPErrorBytes: cppBytes, Chosen: LanguageCPP}), nil
	}
	if cppErr != nil {
		return p.buildResult(file, LanguageC, cTree, options,
			&HeaderResolution{CErrorCount: cCount, CErrorBytes: cBytes, Chosen: LanguageC}), nil
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
		return p.buildResult(file, LanguageC, cTree, options, res), nil
	}
	cTree.Close()
	return p.buildResult(file, LanguageCPP, cppTree, options, res), nil
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

// buildResult 把选中的底层树包装为 SyntaxTree 并按需收集诊断，组装成解析结果。
func (p *treeParser) buildResult(file SourceFile, lang Language, tree *ts.Tree, options ParseOptions, header *HeaderResolution) ParseResult {
	st := &syntaxTree{
		lang:    lang,
		fileID:  file.ID,
		content: file.Content,
		tree:    tree,
	}
	var diags []Diagnostic
	if options.CollectDiagnostics {
		diags = collectDiagnostics(tree)
	}
	return ParseResult{
		Language:    lang,
		Tree:        st,
		Diagnostics: diags,
		Header:      header,
	}
}

// isContextErr 报告错误是否由上下文取消或超时引起。
func isContextErr(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}
