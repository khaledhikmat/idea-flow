package workflows

import (
	"time"

	"github.com/khaledhikmat/idea-flow/application/activities"
	"go.temporal.io/sdk/workflow"
)

// Signals
const (
	IdeaApproveSignalName = "idea-approve-signal"
	IdeaRejectSignalName  = "idea-reject-signal"
)

// Queries
const (
	IdeaStateQueryName = "idea-state-query"
)

type IdeaRequest struct {
	ContestID     string
	Idea          string
	Submitter     string
	ReviewTimeout int
}

type IdeaResult struct {
	ID          string
	ContestID   string
	RunningTime time.Duration
	State       string
	Reviewer    string
	Comment     string
}

func Idea(ctx workflow.Context, request IdeaRequest) (IdeaResult, error) {
	// State
	// These fields ARE the workflow's durable state. They aren't persisted directly —
	// Temporal reconstructs them by replaying history events (activity completions, signals,
	// timers) through this code. That's why mutations must only happen via deterministic
	// paths (activity results, signal handlers, Workflow.* APIs).
	var idea string = request.Idea
	var submitter string = request.Submitter
	var contestID string = request.ContestID
	var timeout int = request.ReviewTimeout
	var reviewTimeout time.Duration = time.Duration(request.ReviewTimeout) * time.Second

	var state = "submitted"
	var ideaID string = ""
	var startedAt time.Time = workflow.Now(ctx)
	var reviewer string = "ai-classifier"
	var comment string = ""

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 10,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)
	startedAt = workflow.Now(ctx)
	logger.Info("Starting idea workflow", "contestID", request.ContestID, "startedAt", startedAt)

	// Setup query handlers for the workflow state
	err := workflow.SetQueryHandler(ctx, IdeaStateQueryName, func() (IdeaResult, error) {
		return IdeaResult{
			ID:          ideaID,
			ContestID:   contestID,
			RunningTime: workflow.Now(ctx).Sub(startedAt),
			State:       state,
			Reviewer:    reviewer,
			Comment:     comment,
		}, nil
	})
	if err != nil {
		return IdeaResult{}, err
	}

	// Store the idea in the database or any persistent storage
	var storageResult activities.StoreResult
	err = workflow.ExecuteActivity(ctx, activities.Store, activities.StoreRequest{
		ContestID: contestID,
		Idea:      idea,
	}).Get(ctx, &storageResult)
	if err != nil {
		return IdeaResult{}, err
	}
	ideaID = storageResult.ID

	// Classify the idea using classification API Endpoints
	var classifyResult activities.ClassifyResult
	err = workflow.ExecuteActivity(ctx, activities.Classify, activities.ClassifyRequest{
		Context:  request.Idea,
		Outcomes: []string{"auto-approved", "auto-rejected", "human-evaluation-required"},
	}).Get(ctx, &classifyResult)
	if err != nil {
		return IdeaResult{}, err
	}

	if classifyResult.Outcome == "auto-approved" {
		state = "approved"
	} else if classifyResult.Outcome == "auto-rejected" {
		state = "rejected"
		return IdeaResult{
			ID:          ideaID,
			ContestID:   contestID,
			RunningTime: workflow.Now(ctx).Sub(startedAt),
			State:       state,
			Reviewer:    reviewer,
			Comment:     comment,
		}, nil

	} else if classifyResult.Outcome == "human-evaluation-required" {
		state = "under-human-review"
	}

	if state == "under-human-review" {
		// Block until approved, rejected, timed out or Continue-As-New is suggested
		for {
			selector := workflow.NewSelector(ctx)
			selector.AddReceive(workflow.GetSignalChannel(ctx, IdeaApproveSignalName), func(c workflow.ReceiveChannel, more bool) {
				var signal ApprovalInfoSignal
				c.Receive(ctx, &signal)
				logger.Info("Idea approved", "ideaID", ideaID, "reviewer", signal.Reviewer)
				reviewer = signal.Reviewer
				state = "approved"
			})
			selector.AddReceive(workflow.GetSignalChannel(ctx, IdeaRejectSignalName), func(c workflow.ReceiveChannel, more bool) {
				var signal RejectionInfoSignal
				c.Receive(ctx, &signal)
				logger.Info("Idea rejected", "ideaID", ideaID, "reviewer", signal.Reviewer, "comment", signal.Comment)
				reviewer = signal.Reviewer
				comment = signal.Comment
				state = "rejected"
			})
			selector.AddFuture(workflow.NewTimer(ctx, reviewTimeout), func(f workflow.Future) {
				logger.Info("Idea approval timed out", "ideaID", ideaID)
				state = "approval-timedout"
			})
			selector.Select(ctx)

			if state == "approved" {
				break // break out of the for loop
			}

			if state == "approval-timedout" || state == "rejected" {
				return IdeaResult{
					ID:          ideaID,
					ContestID:   contestID,
					RunningTime: workflow.Now(ctx).Sub(startedAt),
					State:       state,
					Reviewer:    reviewer,
					Comment:     comment,
				}, nil
			}

			if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
				// Carry current state forward so it is not reset on the new run
				return IdeaResult{
					ID:          "",
					ContestID:   contestID,
					RunningTime: workflow.Now(ctx).Sub(startedAt),
					State:       state,
					Reviewer:    reviewer,
					Comment:     comment,
				}, workflow.NewContinueAsNewError(ctx, Idea, IdeaRequest{
					Idea:          idea,
					ContestID:     contestID,
					ReviewTimeout: timeout,
				})
			}
		}
	}

	logger.Info("Idea was approved", "ideaID", ideaID)

	// Signal the parent contest workflow to register this new idea
	err = workflow.SignalExternalWorkflow(
		ctx,
		contestID, // contest workflow ID
		"",        // run ID; empty = latest run
		ContestIdeaAcceptedSignalName,
		IdeaAcceptedInfoSignal{
			IdeaID:    ideaID,
			Submitter: submitter,
		},
	).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to signal contest", "contestID", contestID, "error", err)
		return IdeaResult{}, err
	}

	state = "accepted"

	return IdeaResult{
		ID:          "",
		ContestID:   contestID,
		RunningTime: workflow.Now(ctx).Sub(startedAt),
		State:       state,
		Reviewer:    reviewer,
		Comment:     comment,
	}, nil
}
