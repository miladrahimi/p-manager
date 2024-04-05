package coordinator

type State struct {
	licensed bool
}

func newState() *State {
	return &State{licensed: false}
}
