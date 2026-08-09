// Package expr implements the CreatureForge motion expression DSL evaluator,
// a Go mirror of creatureforge/motion.py (motion._eval / _resolve_params /
// _build_signals).
//
// Data-driven: expressions are parsed from external JSON; no values are
// hardcoded. Supported ops: const / param / phase / index / frame_count /
// signal / sin / cos / neg / rect / abs / add / sub / mul / table.
package expr

import (
	"encoding/json"
	"fmt"
	"math"
)

// Ctx is the evaluation context, mirroring motion._eval's ctx dict.
type Ctx struct {
	Params     map[string]float64
	Index      int
	FrameCount int
	Phase      float64
	Signals    map[string]func(*Ctx) float64
}

// Expr is a parsed expression: either a plain number, a signal-name string,
// or a single-op dict like {"mul":[{"param":"x"},{"const":2}]}.
type Expr struct {
	Num     *float64 // plain number
	Raw     string   // bare string = signal name (motion._eval: str → signals[expr])
	Op      string   // one of the DSL ops
	NumArg  *float64
	StrArg  *string
	SubArg  *Expr
	ListArg []Expr
	Table   []float64
}

// UnmarshalJSON parses number | string | single-op dict.
func (e *Expr) UnmarshalJSON(b []byte) error {
	*e = Expr{}
	var num float64
	if err := json.Unmarshal(b, &num); err == nil {
		e.Num = &num
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		e.Raw = s
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("expr: cannot parse %s: %w", b, err)
	}
	if len(m) != 1 {
		return fmt.Errorf("expr: must be single-op dict: %s", b)
	}
	for op, raw := range m {
		e.Op = op
		switch op {
		case "param", "signal":
			var v string
			if err := json.Unmarshal(raw, &v); err != nil {
				return fmt.Errorf("expr %s: %w", op, err)
			}
			e.StrArg = &v
		case "const", "index", "frame_count", "phase":
			var v float64
			if err := json.Unmarshal(raw, &v); err != nil {
				return fmt.Errorf("expr %s: %w", op, err)
			}
			e.NumArg = &v
		case "table":
			var t []float64
			if err := json.Unmarshal(raw, &t); err != nil {
				return fmt.Errorf("expr table: %w", err)
			}
			e.Table = t
		case "sin", "cos", "neg", "rect", "abs":
			var sub Expr
			if err := json.Unmarshal(raw, &sub); err != nil {
				return fmt.Errorf("expr %s: %w", op, err)
			}
			e.SubArg = &sub
		case "add", "sub", "mul":
			var l []Expr
			if err := json.Unmarshal(raw, &l); err != nil {
				return fmt.Errorf("expr %s: %w", op, err)
			}
			e.ListArg = l
		default:
			return fmt.Errorf("expr: unknown op %q", op)
		}
	}
	return nil
}

// Eval evaluates the expression against ctx (mirror of motion._eval).
func (e *Expr) Eval(ctx *Ctx) float64 {
	if e == nil {
		return 0
	}
	if e.Num != nil {
		return *e.Num
	}
	if e.Raw != "" {
		return ctx.Signals[e.Raw](ctx)
	}
	switch e.Op {
	case "param":
		return ctx.Params[*e.StrArg]
	case "phase":
		return ctx.Phase
	case "index":
		return float64(ctx.Index)
	case "frame_count":
		return float64(ctx.FrameCount)
	case "const":
		return *e.NumArg
	case "signal":
		return ctx.Signals[*e.StrArg](ctx)
	case "sin":
		return math.Sin(e.SubArg.Eval(ctx))
	case "cos":
		return math.Cos(e.SubArg.Eval(ctx))
	case "neg":
		return -e.SubArg.Eval(ctx)
	case "rect":
		return math.Max(0, e.SubArg.Eval(ctx))
	case "abs":
		return math.Abs(e.SubArg.Eval(ctx))
	case "add":
		s := 0.0
		for _, a := range e.ListArg {
			s += a.Eval(ctx)
		}
		return s
	case "sub":
		return e.ListArg[0].Eval(ctx) - e.ListArg[1].Eval(ctx)
	case "mul":
		p := 1.0
		for _, a := range e.ListArg {
			p *= a.Eval(ctx)
		}
		return p
	case "table":
		n := len(e.Table)
		if n == 0 {
			return 0
		}
		return e.Table[ctx.Index%n]
	}
	return 0
}

// ParamValue is a value that is either a plain number or an expression dict
// (used for coordinate components / motion param overrides).
type ParamValue struct {
	Num  *float64
	Expr *Expr
}

// UnmarshalJSON accepts number | expression dict.
func (p *ParamValue) UnmarshalJSON(b []byte) error {
	*p = ParamValue{}
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		p.Num = &n
		return nil
	}
	var e Expr
	if err := json.Unmarshal(b, &e); err != nil {
		return fmt.Errorf("param value: cannot parse %s: %w", b, err)
	}
	p.Expr = &e
	return nil
}

// Eval resolves the value (number directly, expression via ctx).
func (p *ParamValue) Eval(ctx *Ctx) float64 {
	if p == nil {
		return 0
	}
	if p.Num != nil {
		return *p.Num
	}
	return p.Expr.Eval(ctx)
}

// BuildSignals compiles {name: expr} into callable closures (mirror of
// motion._build_signals). Closures reference ctx at call time, so signals may
// reference one another.
func BuildSignals(signals map[string]*Expr) map[string]func(*Ctx) float64 {
	out := make(map[string]func(*Ctx) float64, len(signals))
	for name, e := range signals {
		name, e := name, e
		out[name] = func(c *Ctx) float64 { return e.Eval(c) }
	}
	return out
}

// ResolveParams resolves motion params from defaults + overrides (mirror of
// motion._resolve_params). override values may be expressions referencing
// refs (body/coord params) and other action params.
func ResolveParams(defaults map[string]float64, overrides map[string]*ParamValue, refs map[string]float64) (map[string]float64, error) {
	merged := make(map[string]float64, len(defaults))
	for k, v := range defaults {
		merged[k] = v
	}
	for key, val := range overrides {
		if _, ok := merged[key]; !ok {
			return nil, fmt.Errorf("unknown motion param: %s", key)
		}
		if val.Expr != nil {
			refsAll := make(map[string]float64, len(merged)+len(refs))
			for k, v := range merged {
				refsAll[k] = v
			}
			for k, v := range refs {
				refsAll[k] = v
			}
			ctx := &Ctx{Params: refsAll, Index: 0, FrameCount: 1, Phase: 0}
			merged[key] = val.Expr.Eval(ctx)
		} else if val.Num != nil {
			merged[key] = *val.Num
		}
	}
	return merged, nil
}
