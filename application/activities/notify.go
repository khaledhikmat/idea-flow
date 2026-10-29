package activities

type NotifyRequest struct {
	ID      string
	Message string
}

type NotifyResult struct {
	Success bool
}

func Notify(_ NotifyRequest) (NotifyResult, error) {
	return NotifyResult{Success: true}, nil
}
