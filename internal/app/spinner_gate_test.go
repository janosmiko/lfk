package app

import (
	"testing"
)

func TestSpinnerNeeded(t *testing.T) {
	if (Model{}).spinnerNeeded() {
		t.Fatal("idle model should not need the spinner")
	}
	if !(Model{loading: true}).spinnerNeeded() {
		t.Fatal("loading model should need the spinner")
	}
	if !(Model{commandBarNameLoading: "pods"}).spinnerNeeded() {
		t.Fatal("command-bar fetch should need the spinner")
	}
}

func TestArmSpinnerStartsLoopOnceWhenNeeded(t *testing.T) {
	m := Model{loading: true, spinnerTicking: false}
	out, cmd := armSpinner(m, nil)
	if !out.(Model).spinnerTicking {
		t.Fatal("armSpinner should mark the loop running")
	}
	if cmd == nil {
		t.Fatal("armSpinner should start the tick when needed and not already ticking")
	}
	// Already ticking -> no second loop.
	_, cmd2 := armSpinner(out.(Model), nil)
	if cmd2 != nil {
		t.Fatal("armSpinner must not stack a second tick loop")
	}
}

func TestArmSpinnerNoLoopWhenIdle(t *testing.T) {
	_, cmd := armSpinner(Model{}, nil)
	if cmd != nil {
		t.Fatal("armSpinner must not start the loop when nothing is loading")
	}
}
