package coordinator

import (
	"time"

	"github.com/miladrahimi/p-manager/pkg/util"
)

const DefaultXraySharedPassword = "1N92QegUGpI4rX9Q7Tyc6E8UsKX+0C4yjq84jyBc+e4="

// State represents coordinator state.
type State struct {
	xrayUpdatedAt      time.Time
	xraySharedPassword string
}

func newState() *State {
	xraySharedPassword, err := util.Key32()
	if err != nil {
		xraySharedPassword = DefaultXraySharedPassword
	}

	return &State{
		xrayUpdatedAt:      time.Now(),
		xraySharedPassword: xraySharedPassword,
	}
}

func (s State) XraySharedPassword() string {
	return s.xraySharedPassword
}

func (s State) XrayUpdatedAt() time.Time {
	return s.xrayUpdatedAt
}
