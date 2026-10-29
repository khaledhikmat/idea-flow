package activities

type EvaluationRequest struct {
	Idea string
}

type EvaluationResult string

const (
	Approved        EvaluationResult = "approved"
	Rejected        EvaluationResult = "rejected"
	HumanEvaluation EvaluationResult = "human_evaluation"
)

func Evaluate(_ EvaluationRequest) (EvaluationResult, error) {
	return HumanEvaluation, nil
}
