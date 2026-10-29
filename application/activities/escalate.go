package activities

type EscalateRequest struct {
	ID      string
	Message string
}

type EscalateResult struct {
	Success bool
}

func Escalate(_ EscalateRequest) (EscalateResult, error) {
	return EscalateResult{Success: true}, nil
}
