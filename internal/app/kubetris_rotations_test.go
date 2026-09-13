package app

import "testing"

// wantTetrominoRotations pins the pre-refactor literal rotation grids so a
// derivation change cannot alter observed game behavior.
var wantTetrominoRotations = [7][4][4][4]bool{
	// I piece
	{
		{
			{false, false, false, false},
			{true, true, true, true},
			{false, false, false, false},
			{false, false, false, false},
		},
		{
			{false, false, true, false},
			{false, false, true, false},
			{false, false, true, false},
			{false, false, true, false},
		},
		{
			{false, false, false, false},
			{false, false, false, false},
			{true, true, true, true},
			{false, false, false, false},
		},
		{
			{false, true, false, false},
			{false, true, false, false},
			{false, true, false, false},
			{false, true, false, false},
		},
	},
	// O piece
	{
		{
			{false, true, true, false},
			{false, true, true, false},
			{false, false, false, false},
			{false, false, false, false},
		},
		{
			{false, true, true, false},
			{false, true, true, false},
			{false, false, false, false},
			{false, false, false, false},
		},
		{
			{false, true, true, false},
			{false, true, true, false},
			{false, false, false, false},
			{false, false, false, false},
		},
		{
			{false, true, true, false},
			{false, true, true, false},
			{false, false, false, false},
			{false, false, false, false},
		},
	},
	// T piece
	{
		{
			{false, true, false, false},
			{true, true, true, false},
			{false, false, false, false},
			{false, false, false, false},
		},
		{
			{false, true, false, false},
			{false, true, true, false},
			{false, true, false, false},
			{false, false, false, false},
		},
		{
			{false, false, false, false},
			{true, true, true, false},
			{false, true, false, false},
			{false, false, false, false},
		},
		{
			{false, true, false, false},
			{true, true, false, false},
			{false, true, false, false},
			{false, false, false, false},
		},
	},
	// S piece
	{
		{
			{false, true, true, false},
			{true, true, false, false},
			{false, false, false, false},
			{false, false, false, false},
		},
		{
			{false, true, false, false},
			{false, true, true, false},
			{false, false, true, false},
			{false, false, false, false},
		},
		{
			{false, false, false, false},
			{false, true, true, false},
			{true, true, false, false},
			{false, false, false, false},
		},
		{
			{true, false, false, false},
			{true, true, false, false},
			{false, true, false, false},
			{false, false, false, false},
		},
	},
	// Z piece
	{
		{
			{true, true, false, false},
			{false, true, true, false},
			{false, false, false, false},
			{false, false, false, false},
		},
		{
			{false, false, true, false},
			{false, true, true, false},
			{false, true, false, false},
			{false, false, false, false},
		},
		{
			{false, false, false, false},
			{true, true, false, false},
			{false, true, true, false},
			{false, false, false, false},
		},
		{
			{false, true, false, false},
			{true, true, false, false},
			{true, false, false, false},
			{false, false, false, false},
		},
	},
	// J piece
	{
		{
			{true, false, false, false},
			{true, true, true, false},
			{false, false, false, false},
			{false, false, false, false},
		},
		{
			{false, true, true, false},
			{false, true, false, false},
			{false, true, false, false},
			{false, false, false, false},
		},
		{
			{false, false, false, false},
			{true, true, true, false},
			{false, false, true, false},
			{false, false, false, false},
		},
		{
			{false, true, false, false},
			{false, true, false, false},
			{true, true, false, false},
			{false, false, false, false},
		},
	},
	// L piece
	{
		{
			{false, false, true, false},
			{true, true, true, false},
			{false, false, false, false},
			{false, false, false, false},
		},
		{
			{false, true, false, false},
			{false, true, false, false},
			{false, true, true, false},
			{false, false, false, false},
		},
		{
			{false, false, false, false},
			{true, true, true, false},
			{true, false, false, false},
			{false, false, false, false},
		},
		{
			{true, true, false, false},
			{false, true, false, false},
			{false, true, false, false},
			{false, false, false, false},
		},
	},
}

func TestKubetrisRotationsMatchLiteral(t *testing.T) {
	for pieceIdx, want := range wantTetrominoRotations {
		got := tetrominoes[pieceIdx].rotations
		if got != want {
			t.Errorf("piece %d: rotations = %v, want %v", pieceIdx, got, want)
		}
	}
}
