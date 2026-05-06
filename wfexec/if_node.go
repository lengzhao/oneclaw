package wfexec

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/lengzhao/oneclaw/workflow"
)

func handleWorkflowIf(_ context.Context, _ NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
	if env.Eval == nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: if: missing template Eval (internal)")
	}
	raw := env.Node.Params["when_any"]
	pass, err := evalWhenAny(env.Eval, raw)
	if err != nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: if: %w", err)
	}
	text := "false"
	if pass {
		text = "true"
	}
	return workflow.WorkflowNodeResult{
		Text: text,
		Data: map[string]any{"pass": pass},
	}, nil
}

func evalWhenAny(ev *EvalEnv, raw any) (bool, error) {
	if ev == nil {
		return false, fmt.Errorf("nil eval")
	}
	clauses, ok := raw.([]any)
	if !ok || len(clauses) == 0 {
		return false, fmt.Errorf("params.when_any must be a non-empty array")
	}
	for _, c := range clauses {
		ok, err := evalClause(ev, c)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func evalClause(ev *EvalEnv, c any) (bool, error) {
	switch x := c.(type) {
	case string:
		s, err := ev.Render(strings.TrimSpace(x))
		if err != nil {
			return false, err
		}
		return isTruthyString(s), nil
	case map[string]any:
		if len(x) == 1 {
			if v, ok := x["truthy"]; ok {
				s, err := operandScalarString(ev, v)
				if err != nil {
					return false, err
				}
				return isTruthyString(s), nil
			}
		}
		if v, ok := x["equals"]; ok {
			return evalEquals(ev, v)
		}
		if v, ok := x["gt"]; ok {
			return evalGt(ev, v)
		}
		return false, fmt.Errorf("unsupported if clause shape %v", x)
	default:
		return false, fmt.Errorf("unsupported if clause type %T", c)
	}
}

func asPair(raw any) ([2]any, error) {
	xs, ok := raw.([]any)
	if !ok || len(xs) != 2 {
		return [2]any{}, fmt.Errorf("want [left, right] pair")
	}
	return [2]any{xs[0], xs[1]}, nil
}

func operandScalarString(ev *EvalEnv, v any) (string, error) {
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		if strings.HasPrefix(s, "$") {
			return ev.Render(s)
		}
		return s, nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case bool:
		return strconv.FormatBool(x), nil
	case nil:
		return "", nil
	default:
		return ev.Render(fmt.Sprint(x))
	}
}

func evalEquals(ev *EvalEnv, raw any) (bool, error) {
	pair, err := asPair(raw)
	if err != nil {
		return false, err
	}
	a, err := operandScalarString(ev, pair[0])
	if err != nil {
		return false, err
	}
	b, err := operandScalarString(ev, pair[1])
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(a) == strings.TrimSpace(b), nil
}

func evalGt(ev *EvalEnv, raw any) (bool, error) {
	pair, err := asPair(raw)
	if err != nil {
		return false, err
	}
	a, err := operandScalarString(ev, pair[0])
	if err != nil {
		return false, err
	}
	b, err := operandScalarString(ev, pair[1])
	if err != nil {
		return false, err
	}
	fa, okA := parseFloatish(a)
	fb, okB := parseFloatish(b)
	if !okA || !okB {
		return false, fmt.Errorf("gt: non-numeric operands %q %q", a, b)
	}
	return fa > fb, nil
}
