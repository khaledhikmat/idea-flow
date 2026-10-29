package activities

type ActivationRequest struct {
	Idea string
}

type ActivationResult struct {
	ID string
}

func Activate(_ ActivationRequest) (ActivationResult, error) {
	return ActivationResult{ID: "some-id"}, nil
}
