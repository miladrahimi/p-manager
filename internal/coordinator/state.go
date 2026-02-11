package coordinator

import (
	"time"
)

// State represents coordinator state.
type State struct {
	xrayUpdatedAt time.Time
}

func newState() *State {
	return &State{
		xrayUpdatedAt: time.Now(),
	}
}

func (s State) XrayUpdatedAt() time.Time {
	return s.xrayUpdatedAt
}
