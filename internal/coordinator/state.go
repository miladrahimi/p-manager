package coordinator

import (
	"github.com/miladrahimi/p-manager/internal/utils"
	"time"
)

const DefaultXraySharedPassword = "1N92QegUGpI4rX9Q7Tyc6E8UsKX+0C4yjq84jyBc+e4="

type State struct {
	xrayUpdatedAt      time.Time
	xraySharedPassword string
}

func (s *State) XraySharedPassword() string {
	return s.xraySharedPassword
}

func (s *State) XrayUpdatedAt() time.Time {
	return s.xrayUpdatedAt
}

func NewState() *State {
	xraySharedPassword, err := utils.Key32()
	if err != nil {
		xraySharedPassword = DefaultXraySharedPassword
	}

	return &State{
		xrayUpdatedAt:      time.Now(),
		xraySharedPassword: xraySharedPassword,
	}
}
