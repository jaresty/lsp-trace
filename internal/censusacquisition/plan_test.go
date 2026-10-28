package censusacquisition

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type planningOnlyObserver struct {
	got  []BatchRequest
	stop error
}

func (o *planningOnlyObserver) AcquireV5(_ context.Context, b BatchRequest) (AcquiredV5, error) {
	o.got = append(o.got, b)
	return AcquiredV5{}, o.stop
}

func TestPlanDiscoveryMatchesCoreWithoutAdmission(t *testing.T) {
	d := discovery(25)
	c := Core{Planning: &PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 63}}
	planned, e := c.PlanDiscovery(d.Session, d)
	if e != nil {
		t.Fatal(e)
	}
	observer := &planningOnlyObserver{stop: errors.New("planning-only observation; no acquisition")}
	c.Discoverer = discoveryFunc(func(context.Context, SessionIdentity) (Discovery, error) { return d, nil })
	c.Acquirer = observer
	_, e = c.Run(context.Background(), d.Session)
	if !errors.Is(e, observer.stop) || !reflect.DeepEqual(planned, observer.got) {
		t.Fatal("ASSERT_PLAN_IDENTICAL_TO_CORE", e)
	}
	d.Complete = false
	if _, e = c.PlanDiscovery(d.Session, d); e == nil {
		t.Fatal("ASSERT_PLAN_INCOMPLETE_REFUSED")
	}
}
