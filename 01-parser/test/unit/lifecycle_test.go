package unit

import (
	"context"
	"errors"
	"testing"

	parser "creek/01-parser"
)

// TestLifecycleCloseIdempotent 验证语法树正常关闭、重复关闭都安全返回。
func TestLifecycleCloseIdempotent(t *testing.T) {
	p := parser.New()
	file := loadFixture(t, "fixture/go/positive/basic.go")
	res, err := p.Parse(context.Background(), file, parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if err := res.Tree.Close(); err != nil {
		t.Fatalf("首次关闭应成功: %v", err)
	}
	if err := res.Tree.Close(); err != nil {
		t.Fatalf("重复关闭应安全返回: %v", err)
	}
}

// TestLifecycleAccessAfterClose 验证关闭语法树后访问节点返回 ErrTreeClosed，而不是访问已释放内存。
func TestLifecycleAccessAfterClose(t *testing.T) {
	p := parser.New()
	file := loadFixture(t, "fixture/go/positive/basic.go")
	res, err := p.Parse(context.Background(), file, parser.ParseOptions{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	root := res.Tree.Root()
	if err := res.Tree.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}

	if _, err := root.Kind(); !errors.Is(err, parser.ErrTreeClosed) {
		t.Errorf("关闭后 Kind 应返回 ErrTreeClosed，实得 %v", err)
	}
	if _, err := res.Tree.Root().Kind(); !errors.Is(err, parser.ErrTreeClosed) {
		t.Errorf("关闭后经 Root 访问应返回 ErrTreeClosed，实得 %v", err)
	}
	if _, err := root.Range(); !errors.Is(err, parser.ErrTreeClosed) {
		t.Errorf("关闭后 Range 应返回 ErrTreeClosed，实得 %v", err)
	}
	if _, _, err := root.Child(0); !errors.Is(err, parser.ErrTreeClosed) {
		t.Errorf("关闭后 Child 应返回 ErrTreeClosed，实得 %v", err)
	}
}

// TestConcurrentParse 验证同一 Parser 可被多个 goroutine 并发调用且结果可用。
func TestConcurrentParse(t *testing.T) {
	p := parser.New()
	file := loadFixture(t, "fixture/python/positive/basic.py")
	const n = 8
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			res, err := p.Parse(context.Background(), file, parser.ParseOptions{CollectDiagnostics: true})
			if err != nil {
				errCh <- err
				return
			}
			_, kerr := res.Tree.Root().Kind()
			res.Tree.Close()
			errCh <- kerr
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("并发解析出错: %v", err)
		}
	}
}
