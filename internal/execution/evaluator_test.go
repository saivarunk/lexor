package execution

import (
	"context"
	"encoding/json"
	p "github.com/varun/lexor/pkg/pluginapi"
	"math"
	"testing"
)

func TestEvaluator(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b any
		o    p.Comparison
		want bool
	}{
		{"nested", map[string]any{"x": []any{1, nil, true}}, map[string]any{"x": []any{1.0, nil, true}}, p.Comparison{Mode: "exact"}, true},
		{"null", nil, nil, p.Comparison{}, true}, {"null versus zero", nil, 0, p.Comparison{}, false},
		{"wrong order", []int{1, 2}, []int{2, 1}, p.Comparison{}, false},
		{"unordered", []int{1, 2, 1}, []int{2, 1, 1}, p.Comparison{Mode: "unordered"}, true},
		{"multiplicity", []int{1, 2, 1}, []int{2, 2, 1}, p.Comparison{Mode: "unordered"}, false},
		{"float nested", []float64{.30000000000000004}, []float64{.3}, p.Comparison{Mode: "float", AbsoluteTolerance: 1e-8}, true},
		{"float outside", .31, .3, p.Comparison{Mode: "float", AbsoluteTolerance: 1e-8}, false},
		{"large integers differ", json.Number("9007199254740993"), json.Number("9007199254740992"), p.Comparison{}, false},
		{"unordered numeric equality", []any{json.Number("1.0"), json.Number("2")}, []any{json.Number("2.00"), json.Number("1")}, p.Comparison{Mode: "unordered"}, true},
		{"extreme exponent rejected", json.Number("1e-1000000000"), json.Number("1e-1000000000"), p.Comparison{}, false},
		{"non-finite", math.NaN(), nil, p.Comparison{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := (JSONEvaluator{}).Evaluate(context.Background(), tc.a, tc.b, tc.o); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
