package expr

import (
	"encoding/json"
	"math"
	"testing"
)

func mustExpr(t *testing.T, s string) *Expr {
	t.Helper()
	var e Expr
	if err := json.Unmarshal([]byte(s), &e); err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return &e
}

func baseCtx() *Ctx {
	return &Ctx{
		Params:     map[string]float64{"intensity": 1.2, "head_scale": 1.5},
		Index:      3,
		FrameCount: 8,
		Phase:      math.Pi / 2,
		Signals:    map[string]func(*Ctx) float64{"sig": func(c *Ctx) float64 { return 2.5 }},
	}
}

func TestConstAndNumber(t *testing.T) {
	ctx := baseCtx()
	if got := mustExpr(t, "1.5").Eval(ctx); got != 1.5 {
		t.Errorf("const: got %v", got)
	}
	if got := mustExpr(t, `{"const": 0.25}`).Eval(ctx); got != 0.25 {
		t.Errorf("const op: got %v", got)
	}
}

func TestParam(t *testing.T) {
	ctx := baseCtx()
	if got := mustExpr(t, `{"param":"intensity"}`).Eval(ctx); got != 1.2 {
		t.Errorf("param: got %v", got)
	}
}

func TestPhaseIndexFrame(t *testing.T) {
	ctx := baseCtx()
	if got := mustExpr(t, `{"phase":0}`).Eval(ctx); got != math.Pi/2 {
		t.Errorf("phase: got %v", got)
	}
	if got := mustExpr(t, `{"index":0}`).Eval(ctx); got != 3 {
		t.Errorf("index: got %v", got)
	}
	if got := mustExpr(t, `{"frame_count":0}`).Eval(ctx); got != 8 {
		t.Errorf("frame_count: got %v", got)
	}
}

func TestSignal(t *testing.T) {
	ctx := baseCtx()
	if got := mustExpr(t, `{"signal":"sig"}`).Eval(ctx); got != 2.5 {
		t.Errorf("signal op: got %v", got)
	}
	if got := mustExpr(t, `"sig"`).Eval(ctx); got != 2.5 {
		t.Errorf("signal string: got %v", got)
	}
}

func TestTrig(t *testing.T) {
	ctx := baseCtx()
	// sin(π/2)=1
	if got := mustExpr(t, `{"sin":{"phase":0}}`).Eval(ctx); math.Abs(got-1) > 1e-9 {
		t.Errorf("sin: got %v", got)
	}
	// cos(0)=1 via const
	if got := mustExpr(t, `{"cos":{"const":0}}`).Eval(ctx); math.Abs(got-1) > 1e-9 {
		t.Errorf("cos: got %v", got)
	}
}

func TestNegRectAbs(t *testing.T) {
	ctx := baseCtx()
	if got := mustExpr(t, `{"neg":{"const":3}}`).Eval(ctx); got != -3 {
		t.Errorf("neg: got %v", got)
	}
	if got := mustExpr(t, `{"rect":{"const":-2}}`).Eval(ctx); got != 0 {
		t.Errorf("rect: got %v", got)
	}
	if got := mustExpr(t, `{"rect":{"const":4}}`).Eval(ctx); got != 4 {
		t.Errorf("rect+: got %v", got)
	}
	if got := mustExpr(t, `{"abs":{"const":-5}}`).Eval(ctx); got != 5 {
		t.Errorf("abs: got %v", got)
	}
}

func TestArith(t *testing.T) {
	ctx := baseCtx()
	if got := mustExpr(t, `{"add":[{"const":1},{"const":2},{"const":3}]}`).Eval(ctx); got != 6 {
		t.Errorf("add: got %v", got)
	}
	if got := mustExpr(t, `{"sub":[{"const":10},{"const":4}]}`).Eval(ctx); got != 6 {
		t.Errorf("sub: got %v", got)
	}
	if got := mustExpr(t, `{"mul":[{"param":"intensity"},{"const":2}]}`).Eval(ctx); got != 2.4 {
		t.Errorf("mul: got %v", got)
	}
}

func TestTable(t *testing.T) {
	ctx := baseCtx() // Index=3
	// table index 3 → 30
	if got := mustExpr(t, `{"table":[10,20,30,40]}`).Eval(ctx); got != 40 {
		t.Errorf("table idx3: got %v", got)
	}
}

func TestParamValue(t *testing.T) {
	var num ParamValue
	if err := json.Unmarshal([]byte("2.5"), &num); err != nil || *num.Num != 2.5 {
		t.Fatalf("num param value: %+v %v", num, err)
	}
	var e ParamValue
	if err := json.Unmarshal([]byte(`{"param":"head_scale"}`), &e); err != nil {
		t.Fatalf("expr param value: %v", err)
	}
	ctx := baseCtx()
	if got := e.Eval(ctx); got != 1.5 {
		t.Errorf("expr param value eval: got %v", got)
	}
}

func TestResolveParams(t *testing.T) {
	defaults := map[string]float64{"intensity": 1.0}
	refs := map[string]float64{"head_scale": 1.5}
	// override as expression: intensity × head_scale
	over := map[string]*ParamValue{
		"intensity": mustParamValue(t, `{"mul":[{"param":"intensity"},{"param":"head_scale"}]}`),
	}
	out, err := ResolveParams(defaults, over, refs)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := out["intensity"]; math.Abs(got-1.5) > 1e-9 {
		t.Errorf("intensity expr: got %v want 1.5", got)
	}
	// unknown param → error
	if _, err := ResolveParams(defaults, map[string]*ParamValue{"nope": mustParamValue(t, "1")}, nil); err == nil {
		t.Error("expected error for unknown param")
	}
}

func mustParamValue(t *testing.T, s string) *ParamValue {
	t.Helper()
	var p ParamValue
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return &p
}
