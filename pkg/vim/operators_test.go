package vim

import "testing"

func makeCtx(text string, cursor int) *OperatorContext {
	ctx := &OperatorContext{
		Text:   text,
		Cursor: cursor,
	}
	ctx.SetText = func(s string) { ctx.Text = s }
	ctx.SetOffset = func(off int) { ctx.Cursor = off }
	ctx.EnterInsert = func(off int) { ctx.Cursor = off }
	reg := ""
	regLW := false
	ctx.GetRegister = func() string { return reg }
	ctx.SetRegister = func(content string, linewise bool) { reg = content; regLW = linewise; _ = regLW }
	return ctx
}

func TestExecuteOperatorMotion_dw(t *testing.T) {
	ctx := makeCtx("hello world", 0)
	ExecuteOperatorMotion(OpDelete, "w", 1, ctx)
	if ctx.Text != "world" {
		t.Fatalf("dw from 0 in 'hello world': want 'world', got %q", ctx.Text)
	}
}

func TestExecuteOperatorMotion_de(t *testing.T) {
	ctx := makeCtx("hello world", 0)
	ExecuteOperatorMotion(OpDelete, "e", 1, ctx)
	if ctx.Text != " world" {
		t.Fatalf("de from 0: want ' world', got %q", ctx.Text)
	}
}

func TestExecuteOperatorMotion_d_dollar(t *testing.T) {
	ctx := makeCtx("hello world", 5)
	ExecuteOperatorMotion(OpDelete, "$", 1, ctx)
	if ctx.Text != "hello" {
		t.Fatalf("d$ from 5: want 'hello', got %q", ctx.Text)
	}
}

func TestExecuteOperatorMotion_yank(t *testing.T) {
	reg := ""
	ctx := makeCtx("hello world", 0)
	ctx.SetRegister = func(content string, _ bool) { reg = content }
	ExecuteOperatorMotion(OpYank, "w", 1, ctx)
	if reg != "hello " {
		t.Fatalf("yw: want 'hello ', got %q", reg)
	}
	// Text unchanged
	if ctx.Text != "hello world" {
		t.Fatalf("yw should not change text")
	}
}

func TestExecuteOperatorTextObj_diw(t *testing.T) {
	ctx := makeCtx("hello world", 2)
	ExecuteOperatorTextObj(OpDelete, ScopeInner, 'w', ctx)
	if ctx.Text != " world" {
		t.Fatalf("diw at 2: want ' world', got %q", ctx.Text)
	}
}

func TestExecuteOperatorTextObj_ci_paren(t *testing.T) {
	inserted := false
	ctx := makeCtx("foo(bar)", 5)
	ctx.EnterInsert = func(off int) { ctx.Cursor = off; inserted = true }
	ExecuteOperatorTextObj(OpChange, ScopeInner, '(', ctx)
	if ctx.Text != "foo()" {
		t.Fatalf("ci(: want 'foo()', got %q", ctx.Text)
	}
	if !inserted {
		t.Fatal("ci( should enter insert mode")
	}
}

func TestExecuteX(t *testing.T) {
	ctx := makeCtx("hello", 0)
	ExecuteX(1, ctx)
	if ctx.Text != "ello" {
		t.Fatalf("x at 0: want 'ello', got %q", ctx.Text)
	}
}

func TestExecuteX_count(t *testing.T) {
	ctx := makeCtx("hello", 0)
	ExecuteX(3, ctx)
	if ctx.Text != "lo" {
		t.Fatalf("3x at 0: want 'lo', got %q", ctx.Text)
	}
}

func TestExecuteReplace(t *testing.T) {
	ctx := makeCtx("hello", 0)
	ExecuteReplace('X', 1, ctx)
	if ctx.Text != "Xello" {
		t.Fatalf("r X at 0: want 'Xello', got %q", ctx.Text)
	}
}

func TestExecuteToggleCase(t *testing.T) {
	ctx := makeCtx("Hello", 0)
	ExecuteToggleCase(3, ctx)
	if ctx.Text != "hELlo" {
		t.Fatalf("3~ at 0: want 'hELlo', got %q", ctx.Text)
	}
}

func TestExecuteLineOp_dd(t *testing.T) {
	ctx := makeCtx("line1\nline2\nline3", 7) // on line2
	ExecuteLineOp(OpDelete, 1, ctx)
	if ctx.Text != "line1\nline3" {
		t.Fatalf("dd on line2: want 'line1\\nline3', got %q", ctx.Text)
	}
}

func TestExecuteLineOp_yy(t *testing.T) {
	reg := ""
	ctx := makeCtx("line1\nline2\nline3", 0)
	ctx.SetRegister = func(c string, _ bool) { reg = c }
	ExecuteLineOp(OpYank, 1, ctx)
	if reg != "line1\n" {
		t.Fatalf("yy on line1: want 'line1\\n', got %q", reg)
	}
	if ctx.Text != "line1\nline2\nline3" {
		t.Fatal("yy should not modify text")
	}
}

func TestExecuteJoin(t *testing.T) {
	ctx := makeCtx("hello\nworld", 0)
	ExecuteJoin(1, ctx)
	if ctx.Text != "hello world" {
		t.Fatalf("J: want 'hello world', got %q", ctx.Text)
	}
}

func TestExecutePaste_characterwise(t *testing.T) {
	ctx := makeCtx("hllo", 0)
	reg := "e"
	ctx.GetRegister = func() string { return reg }
	ctx.SetRegister = func(string, bool) {}
	ExecutePaste(true, 1, ctx) // p after cursor
	if ctx.Text != "hello" {
		t.Fatalf("p 'e' after pos 0 in 'hllo': want 'hello', got %q", ctx.Text)
	}
}

func TestExecutePaste_linewise(t *testing.T) {
	ctx := makeCtx("line1\nline3", 0)
	reg := "line2\n"
	ctx.GetRegister = func() string { return reg }
	ctx.SetRegister = func(string, bool) {}
	ExecutePaste(true, 1, ctx) // p below current line
	if ctx.Text != "line1\nline2\nline3" {
		t.Fatalf("p linewise: want 'line1\\nline2\\nline3', got %q", ctx.Text)
	}
}

func TestExecuteIndent(t *testing.T) {
	ctx := makeCtx("hello", 0)
	ExecuteIndent('>', 1, ctx)
	if ctx.Text != "  hello" {
		t.Fatalf(">>: want '  hello', got %q", ctx.Text)
	}
}

func TestExecuteIndent_dedent(t *testing.T) {
	ctx := makeCtx("  hello", 0)
	ExecuteIndent('<', 1, ctx)
	if ctx.Text != "hello" {
		t.Fatalf("<<: want 'hello', got %q", ctx.Text)
	}
}

func TestExecuteOpenLine_below(t *testing.T) {
	inserted := false
	ctx := makeCtx("line1\nline2", 0)
	ctx.EnterInsert = func(off int) { ctx.Cursor = off; inserted = true }
	ExecuteOpenLine("below", ctx)
	if ctx.Text != "line1\n\nline2" {
		t.Fatalf("o: want 'line1\\n\\nline2', got %q", ctx.Text)
	}
	if !inserted {
		t.Fatal("o should enter insert mode")
	}
}

func TestExecuteOperatorFind_df(t *testing.T) {
	ctx := makeCtx("hello world", 0)
	ExecuteOperatorFind(OpDelete, FindF, 'o', 1, ctx)
	if ctx.Text != " world" {
		t.Fatalf("dfo from 0: want ' world', got %q", ctx.Text)
	}
}

func TestExecuteOperatorG_dG(t *testing.T) {
	ctx := makeCtx("line1\nline2\nline3", 0)
	ExecuteOperatorG(OpDelete, 1, ctx)
	if ctx.Text != "" {
		t.Fatalf("dG from start: want empty, got %q", ctx.Text)
	}
}

func TestExecuteOperatorGg_dgg(t *testing.T) {
	ctx := makeCtx("line1\nline2\nline3", 12) // on line3
	ExecuteOperatorGg(OpDelete, 1, ctx)
	if ctx.Text != "" {
		t.Fatalf("dgg from end: want empty, got %q", ctx.Text)
	}
}
