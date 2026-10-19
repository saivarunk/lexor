package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	p "github.com/varun/lexor/pkg/pluginapi"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

// JSONEvaluator compares runtime JSON values; type declarations never affect decoding.
type JSONEvaluator struct{}

func (JSONEvaluator) Evaluate(ctx context.Context, actual, expected any, options p.Comparison) bool {
	if ctx.Err() != nil {
		return false
	}
	a, ok := normalize(actual)
	if !ok {
		return false
	}
	e, ok := normalize(expected)
	if !ok {
		return false
	}
	return compare(a, e, options)
}
func normalize(v any) (any, bool) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return nil, false
	}
	if !boundedNumbers(value) {
		return nil, false
	}
	return value, true
}
func compare(a, b any, o p.Comparison) bool {
	if o.Mode == "float" {
		if av, ok := a.(json.Number); ok {
			bv, ok := b.(json.Number)
			if !ok {
				return false
			}
			x, xe := av.Float64()
			y, ye := bv.Float64()
			if xe != nil || ye != nil || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
				return false
			}
			abs, rel := o.AbsoluteTolerance, o.RelativeTolerance
			if abs == 0 && rel == 0 {
				abs = 1e-9
				rel = 1e-9
			}
			return math.Abs(x-y) <= math.Max(abs, rel*math.Max(math.Abs(x), math.Abs(y)))
		}
	}
	if aa, ok := a.([]any); ok {
		bb, ok := b.([]any)
		if !ok || len(aa) != len(bb) {
			return false
		}
		if o.Mode == "unordered" {
			counts := map[string]int{}
			for _, x := range aa {
				key, err := json.Marshal(canonical(x))
				if err != nil {
					return false
				}
				counts[string(key)]++
			}
			for _, x := range bb {
				key, err := json.Marshal(canonical(x))
				if err != nil || counts[string(key)] == 0 {
					return false
				}
				counts[string(key)]--
			}
			return true
		}
		for i := range aa {
			if !compare(aa[i], bb[i], o) {
				return false
			}
		}
		return true
	}
	if aa, ok := a.(map[string]any); ok {
		bb, ok := b.(map[string]any)
		if !ok || len(aa) != len(bb) {
			return false
		}
		for k, x := range aa {
			y, ok := bb[k]
			if !ok || !compare(x, y, o) {
				return false
			}
		}
		return true
	}
	if x, ok := a.(json.Number); ok {
		y, ok := b.(json.Number)
		if !ok {
			return false
		}
		rx, xok := new(big.Rat).SetString(string(x))
		ry, yok := new(big.Rat).SetString(string(y))
		return xok && yok && rx.Cmp(ry) == 0
	}
	return reflect.DeepEqual(a, b)
}

// Typed canonical trees preserve numeric equality (1 == 1.0) without conflating
// user strings or object shapes with internal number encodings.
func canonical(v any) any {
	switch x := v.(type) {
	case json.Number:
		r, ok := new(big.Rat).SetString(string(x))
		if !ok {
			return []any{"invalid-number", string(x)}
		}
		return []any{"number", r.RatString()}
	case []any:
		a := make([]any, len(x))
		for i, v := range x {
			a[i] = canonical(v)
		}
		return []any{"array", a}
	case map[string]any:
		m := map[string]any{}
		for k, v := range x {
			m[k] = canonical(v)
		}
		return []any{"object", m}
	default:
		return []any{fmt.Sprintf("%T", v), v}
	}
}

func boundedNumbers(v any) bool {
	switch x := v.(type) {
	case json.Number:
		if len(x) > 4096 {
			return false
		}
		if at := strings.IndexAny(string(x), "eE"); at >= 0 {
			exponent, e := strconv.Atoi(string(x)[at+1:])
			if e != nil || exponent < -4096 || exponent > 4096 {
				return false
			}
		}
		n, err := x.Float64()
		return err == nil && !math.IsInf(n, 0) && !math.IsNaN(n)
	case []any:
		for _, v := range x {
			if !boundedNumbers(v) {
				return false
			}
		}
	case map[string]any:
		for _, v := range x {
			if !boundedNumbers(v) {
				return false
			}
		}
	}
	return true
}
