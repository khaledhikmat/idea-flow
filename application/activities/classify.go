package activities

type ClassifyRequest struct {
	Context  string
	Outcomes []string
}

type ClassifyResult struct {
	Outcome string
}

func Classify(_ ClassifyRequest) (ClassifyResult, error) {
	return ClassifyResult{Outcome: "auto-approved"}, nil
}
