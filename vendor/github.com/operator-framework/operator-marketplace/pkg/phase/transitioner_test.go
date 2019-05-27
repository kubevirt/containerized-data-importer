package phase_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	marketplace "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/operator-framework/operator-marketplace/pkg/phase"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"
)

var (
	phaseWantValidating = marketplace.Phase{
		Name:    "Validating",
		Message: "Scheduled for validation",
	}
	nextPhaseAfterValidating = &marketplace.Phase{
		Name:    phaseWantValidating.Name,
		Message: phaseWantValidating.Message,
	}
)

// Use Case: Phase and Message specified in both objects are identical.
// Expected Result: The function is expected to return false to indicate that no
// transition has taken place.
func TestTransitionInto_IdenticalPhase_FalseExpected(t *testing.T) {
	clock := clock.NewFakeClock(time.Now())
	transitioner := phase.NewTransitionerWithClock(clock)

	opsrcIn := &marketplace.OperatorSource{
		Status: marketplace.OperatorSourceStatus{
			CurrentPhase: marketplace.ObjectPhase{
				Phase: phaseWantValidating,
			},
		},
	}

	changedGot := transitioner.TransitionInto(&opsrcIn.Status.CurrentPhase, nextPhaseAfterValidating)

	assert.False(t, changedGot)
	assert.Equal(t, phaseWantValidating.Name, opsrcIn.Status.CurrentPhase.Name)
	assert.Equal(t, phaseWantValidating.Message, opsrcIn.Status.CurrentPhase.Message)
}

// Use Case: Both Phase and Message specified in both objects are different.
// Expected Result: The function is expected to return true to indicate that a
// transition has taken place.
func TestTransitionInto_BothPhaseAndMessageAreDifferent_TrueExpected(t *testing.T) {
	now := time.Now()

	clock := clock.NewFakeClock(now)
	transitioner := phase.NewTransitionerWithClock(clock)

	opsrcIn := &marketplace.OperatorSource{
		Status: marketplace.OperatorSourceStatus{
			CurrentPhase: marketplace.ObjectPhase{
				Phase: marketplace.Phase{
					Name:    "Initial",
					Message: "Not validated",
				},
			},
		},
	}

	changedGot := transitioner.TransitionInto(&opsrcIn.Status.CurrentPhase, nextPhaseAfterValidating)

	assert.True(t, changedGot)
	assert.Equal(t, phaseWantValidating.Name, opsrcIn.Status.CurrentPhase.Name)
	assert.Equal(t, phaseWantValidating.Message, opsrcIn.Status.CurrentPhase.Message)
	assert.Equal(t, metav1.NewTime(now), opsrcIn.Status.CurrentPhase.LastTransitionTime)
	assert.Equal(t, metav1.NewTime(now), opsrcIn.Status.CurrentPhase.LastUpdateTime)
}

// Use Case: Phase specified in both objects are same but Message is different.
// Expected Result: The function is expected to return true to indicate that an
// update has taken place. LastTransitionTime is expected not to be changed.
func TestTransitionInto_MessageIsDifferent_TrueExpected(t *testing.T) {
	now := time.Now()
	clock := clock.NewFakeClock(now)
	transitioner := phase.NewTransitionerWithClock(clock)

	phaseWant := marketplace.Phase{
		Name:    "Failed",
		Message: "Second try- reason 2",
	}

	opsrcIn := &marketplace.OperatorSource{
		Status: marketplace.OperatorSourceStatus{
			CurrentPhase: marketplace.ObjectPhase{
				Phase: marketplace.Phase{
					Name:    phaseWant.Name,
					Message: "First try- reason 1",
				},
			},
		},
	}

	nextPhase := &marketplace.Phase{
		Name:    phaseWant.Name,
		Message: phaseWant.Message,
	}

	changedGot := transitioner.TransitionInto(&opsrcIn.Status.CurrentPhase, nextPhase)

	assert.True(t, changedGot)
	assert.Equal(t, phaseWant.Name, opsrcIn.Status.CurrentPhase.Name)
	assert.Equal(t, phaseWant.Message, opsrcIn.Status.CurrentPhase.Message)
	assert.Empty(t, opsrcIn.Status.CurrentPhase.LastTransitionTime)
	assert.Equal(t, metav1.NewTime(now), opsrcIn.Status.CurrentPhase.LastUpdateTime)
}
