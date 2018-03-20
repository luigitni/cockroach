// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package constraint

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

func TestConstraintUnion(t *testing.T) {
	test := func(t *testing.T, evalCtx *tree.EvalContext, left, right *Constraint, expected string) {
		t.Helper()
		clone := *left
		clone.UnionWith(evalCtx, right)

		if actual := clone.String(); actual != expected {
			format := "left: %s, right: %s, expected: %v, actual: %v"
			t.Errorf(format, left.String(), right.String(), expected, actual)
		}
	}

	st := cluster.MakeTestingClusterSettings()
	evalCtx := tree.MakeTestingEvalContext(st)
	data := newConstraintTestData(&evalCtx)

	// Union constraint with itself.
	test(t, &evalCtx, &data.c1to10, &data.c1to10, "/1: [/1 - /10]")

	// Merge first spans in each constraint.
	test(t, &evalCtx, &data.c1to10, &data.c5to25, "/1: [/1 - /25)")
	test(t, &evalCtx, &data.c5to25, &data.c1to10, "/1: [/1 - /25)")

	// Disjoint spans in each constraint.
	test(t, &evalCtx, &data.c1to10, &data.c40to50, "/1: [/1 - /10] [/40 - /50]")
	test(t, &evalCtx, &data.c40to50, &data.c1to10, "/1: [/1 - /10] [/40 - /50]")

	// Adjacent disjoint spans in each constraint.
	test(t, &evalCtx, &data.c20to30, &data.c30to40, "/1: [/20 - /40]")
	test(t, &evalCtx, &data.c30to40, &data.c20to30, "/1: [/20 - /40]")

	// Merge multiple spans down to single span.
	var left, right Constraint
	left = data.c1to10
	left.UnionWith(&evalCtx, &data.c20to30)
	left.UnionWith(&evalCtx, &data.c40to50)

	right = data.c5to25
	right.UnionWith(&evalCtx, &data.c30to40)

	test(t, &evalCtx, &left, &right, "/1: [/1 - /50]")
	test(t, &evalCtx, &right, &left, "/1: [/1 - /50]")

	// Multiple disjoint spans on each side.
	left = data.c1to10
	left.UnionWith(&evalCtx, &data.c20to30)

	right = data.c40to50
	right.UnionWith(&evalCtx, &data.c60to70)

	test(t, &evalCtx, &left, &right, "/1: [/1 - /10] [/20 - /30) [/40 - /50] (/60 - /70)")
	test(t, &evalCtx, &right, &left, "/1: [/1 - /10] [/20 - /30) [/40 - /50] (/60 - /70)")

	// Multiple spans that yield the unconstrained span.
	left = data.cLt10
	right = data.c5to25
	right.UnionWith(&evalCtx, &data.cGt20)

	test(t, &evalCtx, &left, &right, "/1: unconstrained")
	test(t, &evalCtx, &right, &left, "/1: unconstrained")

	if left.String() != "/1: [ - /10)" {
		t.Errorf("tryUnionWith failed, but still modified one of the spans: %v", left.String())
	}
	if right.String() != "/1: (/5 - ]" {
		t.Errorf("tryUnionWith failed, but still modified one of the spans: %v", right.String())
	}

	// Multiple columns.
	expected := "/1/2: [/'cherry'/true - /'strawberry']"
	test(t, &evalCtx, &data.cherryRaspberry, &data.mangoStrawberry, expected)
	test(t, &evalCtx, &data.mangoStrawberry, &data.cherryRaspberry, expected)
}

func TestConstraintIntersect(t *testing.T) {
	test := func(t *testing.T, evalCtx *tree.EvalContext, left, right *Constraint, expected string) {
		t.Helper()
		clone := *left
		clone.IntersectWith(evalCtx, right)
		if actual := clone.String(); actual != expected {
			format := "left: %s, right: %s, expected: %v, actual: %v"
			t.Errorf(format, left.String(), right.String(), expected, actual)
		}
	}

	st := cluster.MakeTestingClusterSettings()
	evalCtx := tree.MakeTestingEvalContext(st)
	data := newConstraintTestData(&evalCtx)

	// Intersect constraint with itself.
	test(t, &evalCtx, &data.c1to10, &data.c1to10, "/1: [/1 - /10]")

	// Intersect first spans in each constraint.
	test(t, &evalCtx, &data.c1to10, &data.c5to25, "/1: (/5 - /10]")
	test(t, &evalCtx, &data.c5to25, &data.c1to10, "/1: (/5 - /10]")

	// Disjoint spans in each constraint.
	test(t, &evalCtx, &data.c1to10, &data.c40to50, "/1: contradiction")
	test(t, &evalCtx, &data.c40to50, &data.c1to10, "/1: contradiction")

	// Intersect multiple spans.
	var left, right Constraint
	left = data.c1to10
	left.UnionWith(&evalCtx, &data.c20to30)
	left.UnionWith(&evalCtx, &data.c40to50)

	right = data.c5to25
	right.UnionWith(&evalCtx, &data.c30to40)

	test(t, &evalCtx, &right, &left, "/1: (/5 - /10] [/20 - /25) [/40 - /40]")
	test(t, &evalCtx, &left, &right, "/1: (/5 - /10] [/20 - /25) [/40 - /40]")

	// Intersect multiple disjoint spans.
	left = data.c1to10
	left.UnionWith(&evalCtx, &data.c20to30)

	right = data.c40to50
	right.UnionWith(&evalCtx, &data.c60to70)

	test(t, &evalCtx, &left, &right, "/1: contradiction")
	test(t, &evalCtx, &right, &left, "/1: contradiction")

	if left.String() != "/1: [/1 - /10] [/20 - /30)" {
		t.Errorf("tryIntersectWith failed, but still modified one of the spans: %v", left.String())
	}
	if right.String() != "/1: [/40 - /50] (/60 - /70)" {
		t.Errorf("tryIntersectWith failed, but still modified one of the spans: %v", right.String())
	}

	// Multiple columns.
	expected := "/1/2: [/'mango'/false - /'raspberry'/false)"
	test(t, &evalCtx, &data.cherryRaspberry, &data.mangoStrawberry, expected)
	test(t, &evalCtx, &data.mangoStrawberry, &data.cherryRaspberry, expected)
}

type constraintTestData struct {
	cLt10           Constraint // [ - /10)
	cGt20           Constraint // (/20 - ]
	c1to10          Constraint // [/1 - /10]
	c5to25          Constraint // (/5 - /25)
	c20to30         Constraint // [/20 - /30)
	c30to40         Constraint // [/30 - /40]
	c40to50         Constraint // [/40 - /50]
	c60to70         Constraint // (/60 - /70)
	cherryRaspberry Constraint // [/'cherry'/true - /'raspberry'/false)
	mangoStrawberry Constraint // [/'mango'/false - /'strawberry']
}

func newConstraintTestData(evalCtx *tree.EvalContext) *constraintTestData {
	data := &constraintTestData{}

	key1 := MakeKey(tree.NewDInt(1))
	key5 := MakeKey(tree.NewDInt(5))
	key10 := MakeKey(tree.NewDInt(10))
	key20 := MakeKey(tree.NewDInt(20))
	key25 := MakeKey(tree.NewDInt(25))
	key30 := MakeKey(tree.NewDInt(30))
	key40 := MakeKey(tree.NewDInt(40))
	key50 := MakeKey(tree.NewDInt(50))
	key60 := MakeKey(tree.NewDInt(60))
	key70 := MakeKey(tree.NewDInt(70))

	kc12 := testKeyContext(1, 2)
	kc1 := testKeyContext(1)

	cherry := MakeCompositeKey(tree.NewDString("cherry"), tree.DBoolTrue)
	mango := MakeCompositeKey(tree.NewDString("mango"), tree.DBoolFalse)
	raspberry := MakeCompositeKey(tree.NewDString("raspberry"), tree.DBoolFalse)
	strawberry := MakeKey(tree.NewDString("strawberry"))

	var span Span

	// [ - /10)
	span.Set(kc1, EmptyKey, IncludeBoundary, key10, ExcludeBoundary)
	data.cLt10.Init(kc1, SingleSpan(&span))

	// (/20 - ]
	span.Set(kc1, key20, ExcludeBoundary, EmptyKey, IncludeBoundary)
	data.cGt20.Init(kc1, SingleSpan(&span))

	// [/1 - /10]
	span.Set(kc1, key1, IncludeBoundary, key10, IncludeBoundary)
	data.c1to10.Init(kc1, SingleSpan(&span))

	// (/5 - /25)
	span.Set(kc1, key5, ExcludeBoundary, key25, ExcludeBoundary)
	data.c5to25.Init(kc1, SingleSpan(&span))

	// [/20 - /30)
	span.Set(kc1, key20, IncludeBoundary, key30, ExcludeBoundary)
	data.c20to30.Init(kc1, SingleSpan(&span))

	// [/30 - /40]
	span.Set(kc1, key30, IncludeBoundary, key40, IncludeBoundary)
	data.c30to40.Init(kc1, SingleSpan(&span))

	// [/40 - /50]
	span.Set(kc1, key40, IncludeBoundary, key50, IncludeBoundary)
	data.c40to50.Init(kc1, SingleSpan(&span))

	// (/60 - /70)
	span.Set(kc1, key60, ExcludeBoundary, key70, ExcludeBoundary)
	data.c60to70.Init(kc1, SingleSpan(&span))

	// [/'cherry'/true - /'raspberry'/false)
	span.Set(kc12, cherry, IncludeBoundary, raspberry, ExcludeBoundary)
	data.cherryRaspberry.Init(kc12, SingleSpan(&span))

	// [/'mango'/false - /'strawberry']
	span.Set(kc12, mango, IncludeBoundary, strawberry, IncludeBoundary)
	data.mangoStrawberry.Init(kc12, SingleSpan(&span))

	return data
}
