package activities

type SubmitRequest struct {
	Idea string
}

type SubmitResult struct {
	ID string
}

func Submit(_ SubmitRequest) (SubmitResult, error) {
	return SubmitResult{ID: "some-id"}, nil
}
