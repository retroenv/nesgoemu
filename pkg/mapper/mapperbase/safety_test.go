package mapperbase

import (
	"encoding/json"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

const (
	safetyTestCHRWindowSize = 0x2000
	snapshotChangedBank     = 99
	stateSwitchIterations   = 1000
	stateSwitchBankCount    = 2
)

func TestStateOwnsBankWindows(t *testing.T) {
	base := newPPUMappingTestBase(t, safetyTestCHRWindowSize, false)
	state := base.State()
	base.SetChrWindow(0, 1)
	base.SetPrgWindow(0, 1)
	assert.Equal(t, 0, state.ChrWindows[0])
	assert.Equal(t, 0, state.PrgWindows[0])

	// A state result must not give the caller access to the live bank tables.
	state.ChrWindows[0] = snapshotChangedBank
	state.PrgWindows[0] = snapshotChangedBank
	assert.Equal(t, 1, base.State().ChrWindows[0])
	assert.Equal(t, 1, base.State().PrgWindows[0])
}

func TestStateDuringBankSwitches(t *testing.T) {
	base := newPPUMappingTestBase(t, safetyTestCHRWindowSize, false)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range stateSwitchIterations {
			base.SetChrWindow(0, i%stateSwitchBankCount)
			base.SetPrgWindow(0, i%stateSwitchBankCount)
		}
	}()
	for range stateSwitchIterations {
		_, err := json.Marshal(base.State())
		assert.NoError(t, err)
	}
	<-done
}
