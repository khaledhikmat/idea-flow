package activities

import (
	"fmt"
	"time"
)

type StoreRequest struct {
	ContestID string
	Idea      string
}

type StoreResult struct {
	ID string
}

func Store(_ StoreRequest) (StoreResult, error) {
	// Implement the logic to store the idea in the database or any persistent storage
	// For now, we will just return a dummy ID
	return StoreResult{
		ID: "dummy-id" + fmt.Sprint(time.Now().Unix()),
	}, nil
}
