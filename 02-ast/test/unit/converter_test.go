package unit

import (
	"context"
	"strings"
	"testing"

	parser "creek/01-parser"
	commonast "creek/02-ast"
)

// TestConvertPositive 验证五种语言都能转换为无系统失败的 Common AST。
func TestConvertPositive(t *testing.T) {
	cases := []struct {
		// name 是子测试名称和 golden 文件名。
		name string
		// fixture 是待转换的独立源码 fixture。
		fixture string
		// language 是期望的源码语言。
		language parser.Language
	}{
		{name: "c", fixture: "c/positive/basic.c", language: parser.LanguageC},
		{name: "cpp", fixture: "cpp/positive/basic.cpp", language: parser.LanguageCPP},
		{name: "java", fixture: "java/positive/Basic.java", language: parser.LanguageJava},
		{name: "go", fixture: "go/positive/basic.go", language: parser.LanguageGo},
		{name: "python", fixture: "python/positive/basic.py", language: parser.LanguagePython},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tree, diagnostics := buildAST(t, testCase.fixture)
			if tree.Language() != testCase.language {
				t.Fatalf("AST 语言 = %v，期望 %v", tree.Language(), testCase.language)
			}
			if len(diagnostics) != 0 {
				t.Fatalf("合法正例产生转换诊断: %+v", diagnostics)
			}
			if tree.Root().Kind() != commonast.KindFile {
				t.Fatalf("根节点 = %s，期望 File", tree.Root().Kind())
			}
			assertGolden(t, testCase.name, snapshotAST(tree))
		})
	}
}

// TestCommonASTKeepsFileIdentity 验证 Common AST 原样保留语法树的文件编号与项目内相对路径。
func TestCommonASTKeepsFileIdentity(t *testing.T) {
	fixtures := []string{
		"c/positive/basic.c",
		"cpp/positive/basic.cpp",
		"java/positive/Basic.java",
		"go/positive/basic.go",
		"python/positive/basic.py",
	}
	for index, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			file := loadSourceFile(t, fixture)
			file.ID = parser.FileID(100 + index)
			parsed, err := parser.New().Parse(context.Background(), file, parser.ParseOptions{})
			if err != nil {
				t.Fatalf("解析 fixture 失败 %s: %v", fixture, err)
			}
			defer parsed.Tree.Close()
			tree, _, err := commonast.New().Convert(context.Background(), parsed.Tree)
			if err != nil {
				t.Fatalf("转换 fixture 失败 %s: %v", fixture, err)
			}
			if tree.FileID() != file.ID {
				t.Errorf("FileID = %d，期望 %d", tree.FileID(), file.ID)
			}
			if tree.Path() != fixture {
				t.Errorf("Path = %q，期望 %q", tree.Path(), fixture)
			}
		})
	}
}

// TestLoopSyntaxPreserved 验证 Common AST 不会把不同循环语法提前归一。
func TestLoopSyntaxPreserved(t *testing.T) {
	tree, _ := buildAST(t, "c/positive/basic.c")
	root := tree.Root()
	if countKind(root, commonast.KindForStmt) != 1 {
		t.Fatalf("ForStmt 数量错误: %d", countKind(root, commonast.KindForStmt))
	}
	if countKind(root, commonast.KindWhileStmt) != 1 {
		t.Fatalf("WhileStmt 数量错误: %d", countKind(root, commonast.KindWhileStmt))
	}
	if countKind(root, commonast.KindDoWhileStmt) != 1 {
		t.Fatalf("DoWhileStmt 数量错误: %d", countKind(root, commonast.KindDoWhileStmt))
	}
}

// TestChildByRole 验证条件语句可以通过语义角色访问条件和语句体。
func TestChildByRole(t *testing.T) {
	tree, _ := buildAST(t, "c/positive/basic.c")
	ifNode, ok := findFirst(tree.Root(), commonast.KindIfStmt)
	if !ok {
		t.Fatal("未找到 IfStmt")
	}
	if _, ok := ifNode.ChildByRole(commonast.RoleCondition); !ok {
		t.Error("IfStmt 缺少 condition 角色")
	}
	if _, ok := ifNode.ChildByRole(commonast.RoleBody); !ok {
		t.Error("IfStmt 缺少 body 角色")
	}
}

// TestOperatorAttributes 验证赋值与成员访问运算符通过明确属性保留。
func TestOperatorAttributes(t *testing.T) {
	cTree, _ := buildAST(t, "c/positive/basic.c")
	assignment, ok := findFirst(cTree.Root(), commonast.KindAssignExpr)
	if !ok {
		t.Fatal("未找到 AssignExpr")
	}
	if operator, ok := assignment.Attribute("operator"); !ok || operator != "+=" {
		t.Fatalf("AssignExpr operator = %q, %v，期望 +=", operator, ok)
	}

	cppTree, _ := buildAST(t, "cpp/positive/basic.cpp")
	member, ok := findFirst(cppTree.Root(), commonast.KindMemberAccess)
	if !ok {
		t.Fatal("未找到 MemberAccess")
	}
	if operator, ok := member.Attribute("operator"); !ok || operator == "" {
		t.Fatalf("MemberAccess 未保留 operator: %q, %v", operator, ok)
	}
}

// TestIndexes 验证 NodeID 索引与 CST 范围反向索引都能找到同一节点。
func TestIndexes(t *testing.T) {
	tree, _ := buildAST(t, "java/positive/Basic.java")
	function, ok := findFirst(tree.Root(), commonast.KindFunctionDecl)
	if !ok {
		t.Fatal("未找到 FunctionDecl")
	}
	byID, ok := tree.NodeByID(function.ID())
	if !ok || byID.Kind() != commonast.KindFunctionDecl {
		t.Fatalf("NodeByID 未返回目标节点: %v, %v", byID, ok)
	}
	byRange := tree.NodesByCSTRange(function.CSTRange())
	found := false
	for _, node := range byRange {
		if node.ID() == function.ID() {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("NodesByCSTRange 未返回目标节点")
	}
}

// TestLanguageExtension 验证语言特有语法通过受控扩展保留且不产生未映射诊断。
func TestLanguageExtension(t *testing.T) {
	tree, diagnostics := buildAST(t, "c/positive/extension.c")
	if len(diagnostics) != 0 {
		t.Fatalf("已知语言扩展产生诊断: %+v", diagnostics)
	}
	extensionNode, ok := findFirst(tree.Root(), commonast.KindUnknown)
	if !ok {
		t.Fatal("未找到承载宏定义的扩展节点")
	}
	extension, ok := extensionNode.Extension()
	if !ok {
		t.Fatal("宏定义节点缺少语言扩展")
	}
	if extension.Kind() != "宏定义" || extension.Language() != parser.LanguageC {
		t.Fatalf("宏定义扩展错误: kind=%s language=%v", extension.Kind(), extension.Language())
	}
}

// TestDeterminism 验证同一 CST 重复转换得到相同结构和 NodeID。
func TestDeterminism(t *testing.T) {
	first, _ := buildAST(t, "cpp/positive/basic.cpp")
	second, _ := buildAST(t, "cpp/positive/basic.cpp")
	if snapshotAST(first) != snapshotAST(second) {
		t.Fatal("重复转换得到的 AST 结构不一致")
	}
	firstFunction, ok := findFirst(first.Root(), commonast.KindFunctionDecl)
	if !ok {
		t.Fatal("首次转换未找到 FunctionDecl")
	}
	secondFunction, ok := findFirst(second.Root(), commonast.KindFunctionDecl)
	if !ok {
		t.Fatal("二次转换未找到 FunctionDecl")
	}
	if firstFunction.ID() != secondFunction.ID() {
		t.Fatalf("FunctionDecl NodeID 不稳定: %d != %d", firstFunction.ID(), secondFunction.ID())
	}
}

// TestCrossLanguageFunctionShape 验证五种语言的等价函数具有一致的核心控制结构。
func TestCrossLanguageFunctionShape(t *testing.T) {
	fixtures := []string{
		"cross/function/c/main.c",
		"cross/function/cpp/main.cpp",
		"cross/function/java/Main.java",
		"cross/function/go/main.go",
		"cross/function/python/main.py",
	}
	var expected string
	for _, fixture := range fixtures {
		tree, _ := buildAST(t, fixture)
		function, ok := findFirst(tree.Root(), commonast.KindFunctionDecl)
		if !ok {
			t.Fatalf("%s 未找到 FunctionDecl", fixture)
		}
		var shape strings.Builder
		structuralShape(function, &shape)
		if expected == "" {
			expected = shape.String()
			continue
		}
		if shape.String() != expected {
			t.Fatalf("跨语言函数结构不一致 %s\n实际: %s\n期望: %s", fixture, shape.String(), expected)
		}
	}
}

// TestEmptyFiles 验证五种语言的空文件都能产生空的 File 根节点。
func TestEmptyFiles(t *testing.T) {
	fixtures := []string{
		"c/boundary/empty.c",
		"cpp/boundary/empty.cpp",
		"java/boundary/Empty.java",
		"go/boundary/empty.go",
		"python/boundary/empty.py",
	}
	for _, fixture := range fixtures {
		tree, diagnostics := buildAST(t, fixture)
		if tree.Root().Kind() != commonast.KindFile {
			t.Fatalf("%s 根节点不是 File", fixture)
		}
		if tree.Root().ChildCount() != 0 {
			t.Fatalf("%s 空文件 AST 不为空", fixture)
		}
		if len(diagnostics) != 0 {
			t.Fatalf("%s 空文件产生诊断: %+v", fixture, diagnostics)
		}
	}
}
