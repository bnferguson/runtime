package saga

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Independent action types — each has unique saga keys so there are no
// data dependencies between actions.

type alphaIn struct {
	X int `saga:"alpha_x"`
}
type alphaOut struct {
	Y int `saga:"alpha_y"`
}

func alphaExec(_ context.Context, in alphaIn) (alphaOut, error) {
	return alphaOut{Y: in.X}, nil
}
func alphaUndo(_ context.Context, _ alphaIn, _ alphaOut) error { return nil }

type bravoIn struct {
	X int `saga:"bravo_x"`
}
type bravoOut struct {
	Y int `saga:"bravo_y"`
}

func bravoExec(_ context.Context, in bravoIn) (bravoOut, error) {
	return bravoOut{Y: in.X}, nil
}
func bravoUndo(_ context.Context, _ bravoIn, _ bravoOut) error { return nil }

type charlieIn struct {
	X int `saga:"charlie_x"`
}
type charlieOut struct {
	Y int `saga:"charlie_y"`
}

func charlieExec(_ context.Context, in charlieIn) (charlieOut, error) {
	return charlieOut{Y: in.X}, nil
}
func charlieUndo(_ context.Context, _ charlieIn, _ charlieOut) error { return nil }

// Diamond dependency types: A -> {B, C} -> D
//
//   A produces "mid_a"
//   B reads "mid_a", produces "mid_b"
//   C reads "mid_a", produces "mid_c"
//   D reads "mid_b" and "mid_c"

type diamondAIn struct {
	Seed int `saga:"seed"`
}
type diamondAOut struct {
	MidA int `saga:"mid_a"`
}

func diamondAExec(_ context.Context, in diamondAIn) (diamondAOut, error) {
	return diamondAOut{MidA: in.Seed}, nil
}
func diamondAUndo(_ context.Context, _ diamondAIn, _ diamondAOut) error { return nil }

type diamondBIn struct {
	MidA int `saga:"mid_a"`
}
type diamondBOut struct {
	MidB int `saga:"mid_b"`
}

func diamondBExec(_ context.Context, in diamondBIn) (diamondBOut, error) {
	return diamondBOut{MidB: in.MidA}, nil
}
func diamondBUndo(_ context.Context, _ diamondBIn, _ diamondBOut) error { return nil }

type diamondCIn struct {
	MidA int `saga:"mid_a"`
}
type diamondCOut struct {
	MidC int `saga:"mid_c"`
}

func diamondCExec(_ context.Context, in diamondCIn) (diamondCOut, error) {
	return diamondCOut{MidC: in.MidA}, nil
}
func diamondCUndo(_ context.Context, _ diamondCIn, _ diamondCOut) error { return nil }

type diamondDIn struct {
	MidB int `saga:"mid_b"`
	MidC int `saga:"mid_c"`
}
type diamondDOut struct {
	Result int `saga:"diamond_result"`
}

func diamondDExec(_ context.Context, in diamondDIn) (diamondDOut, error) {
	return diamondDOut{Result: in.MidB + in.MidC}, nil
}
func diamondDUndo(_ context.Context, _ diamondDIn, _ diamondDOut) error { return nil }

// Edge-linked action types — the only dependency is an Edge key.

type edgeProducerIn struct {
	Seed int `saga:"seed"`
}
type edgeProducerOut struct {
	Value int  `saga:"edge_value"`
	Done  Edge `saga:"producer_done"`
}

func edgeProducerExec(_ context.Context, in edgeProducerIn) (edgeProducerOut, error) {
	return edgeProducerOut{Value: in.Seed * 2}, nil
}
func edgeProducerUndo(_ context.Context, _ edgeProducerIn, _ edgeProducerOut) error { return nil }

type edgeConsumerIn struct {
	X    int  `saga:"consumer_x"`
	Done Edge `saga:"producer_done"`
}
type edgeConsumerOut struct {
	Y int `saga:"consumer_y"`
}

func edgeConsumerExec(_ context.Context, in edgeConsumerIn) (edgeConsumerOut, error) {
	return edgeConsumerOut{Y: in.X}, nil
}
func edgeConsumerUndo(_ context.Context, _ edgeConsumerIn, _ edgeConsumerOut) error { return nil }

func TestEdge_CreatesDependencyWithoutData(t *testing.T) {
	// producer outputs an Edge key "producer_done"
	// consumer reads "producer_done" — this should create a dependency edge
	// even though no real data flows through it.
	def, err := Define("edge-test").
		Action("consumer", edgeConsumerExec).Undo(edgeConsumerUndo).
		Action("producer", edgeProducerExec).Undo(edgeProducerUndo).
		Build()
	require.NoError(t, err)

	order := def.ExecutionOrder()
	require.Len(t, order, 2)

	// producer must come before consumer because of the Edge dependency
	assert.Equal(t, "producer", order[0])
	assert.Equal(t, "consumer", order[1])

	// Verify the dependency is recorded
	consumerNode := def.Actions["consumer"]
	assert.Contains(t, consumerNode.Dependencies, "producer")
}

func TestTopologicalSort_Deterministic(t *testing.T) {
	t.Run("independent actions are alphabetically sorted", func(t *testing.T) {
		// Register actions in non-alphabetical order to verify the sort
		// isn't just preserving insertion order.
		def, err := Define("independent").
			Action("charlie", charlieExec).Undo(charlieUndo).
			Action("alpha", alphaExec).Undo(alphaUndo).
			Action("bravo", bravoExec).Undo(bravoUndo).
			Build()
		require.NoError(t, err)

		order := def.ExecutionOrder()
		assert.Equal(t, []string{"alpha", "bravo", "charlie"}, order)

		// Call again to verify stability.
		assert.Equal(t, order, def.ExecutionOrder())
	})

	t.Run("diamond dependencies with alphabetical tiebreaking", func(t *testing.T) {
		// Diamond: A -> {B, C} -> D
		// B and C are independent of each other but both depend on A,
		// so they should be ordered alphabetically between themselves.
		def, err := Define("diamond").
			Action("d-action", diamondDExec).Undo(diamondDUndo).
			Action("c-action", diamondCExec).Undo(diamondCUndo).
			Action("a-action", diamondAExec).Undo(diamondAUndo).
			Action("b-action", diamondBExec).Undo(diamondBUndo).
			Build()
		require.NoError(t, err)

		order := def.ExecutionOrder()
		require.Len(t, order, 4)

		// A must come first (only node with no deps).
		assert.Equal(t, "a-action", order[0])

		// B and C depend only on A — alphabetical tiebreak.
		assert.Equal(t, "b-action", order[1])
		assert.Equal(t, "c-action", order[2])

		// D depends on B and C — must come last.
		assert.Equal(t, "d-action", order[3])

		// Verify stability across repeated calls.
		assert.Equal(t, order, def.ExecutionOrder())
	})
}
