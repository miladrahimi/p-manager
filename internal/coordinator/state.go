package coordinator

import "time"

type State struct {
	xrayUpdatedAt time.Time
}

func (s *State) XrayUpdatedAt() time.Time {
	return s.xrayUpdatedAt
}

func NewState() *State {
	return &State{
		xrayUpdatedAt: time.Now(),
	}
}
