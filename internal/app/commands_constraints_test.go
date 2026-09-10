package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
)

func TestLoadConstraints_SchedulesOneCall(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.scheduler.StopWorkers()
	m.constraints.target = k8s.ConstraintTarget{Namespace: "default"}

	assertSchedulesOne(t, m, m.loadConstraints("pod-1"))
}
