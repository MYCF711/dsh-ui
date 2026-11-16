package apiclient

import (
	"encoding/json"
	"testing"
)

func TestSessionProjectionsPlan(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want *PlanProjection
	}{
		{name: "active pending", raw: `{"active":true,"pending":true}`, want: &PlanProjection{Active: true, Pending: true}},
		{name: "real false values", raw: `{"active":false,"pending":false}`, want: &PlanProjection{}},
		{name: "null", raw: `null`},
		{name: "missing active", raw: `{"pending":false}`},
		{name: "missing pending", raw: `{"active":false}`},
		{name: "wrong active type", raw: `{"active":"false","pending":false}`},
		{name: "wrong pending type", raw: `{"active":false,"pending":0}`},
		{name: "wrong container", raw: `[]`},
		{name: "malformed", raw: `{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proj := &SessionProjections{Values: map[string]json.RawMessage{"plan": json.RawMessage(tc.raw)}}
			got := proj.Plan()
			if tc.want == nil {
				if got != nil {
					t.Fatalf("坏形状必须是未知：%+v", got)
				}
				return
			}
			if got == nil || *got != *tc.want {
				t.Fatalf("Plan()=%+v，want %+v", got, tc.want)
			}
		})
	}

	for name, proj := range map[string]*SessionProjections{
		"nil receiver": nil,
		"nil values":   {},
		"missing key":  {Values: map[string]json.RawMessage{}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := proj.Plan(); got != nil {
				t.Fatalf("缺键必须是未知：%+v", got)
			}
		})
	}
}
