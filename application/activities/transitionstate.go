package activities

type State string

const (
	SubmittedState State = "submitted"
	ApprovedState  State = "approved"
	RejectedState  State = "rejected"
	PendingState   State = "pending"
)

type StateTransitionRequest struct {
	ID   string
	From State
	To   State
}

type StateTransitionResult struct {
	StatusCode string
}

func TransitionState(_ StateTransitionRequest) (StateTransitionResult, error) {
	return StateTransitionResult{StatusCode: "success"}, nil
}
