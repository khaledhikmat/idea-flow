package workflows

import (
	"sort"
	"strings"
	"time"

	"github.com/khaledhikmat/idea-flow/application/activities"
	"go.temporal.io/sdk/workflow"
)

// Signals
const (
	ContestApproveSignalName      = "contest-approve-signal"
	ContestRejectSignalName       = "contest-reject-signal"
	ContestPledgeSignalName       = "contest-pledge-signal"
	ContestIdeaAcceptedSignalName = "contest-idea-accepted-signal"
	ContestIdeaScoreSignalName    = "contest-idea-score-signal"
)

// Queries
const (
	ContestStateQueryName = "contest-state-query"
)

type ContestRequest struct {
	ID                   string
	Goal                 int
	Rules                []string
	Awards               []string
	ReviewTimeout        int
	FundingTimeout       int
	IdeasAcceptedTimeout int
}

type ContestResult struct {
	ID            string
	RunningTime   time.Duration
	State         string
	Reviewer      string
	Comment       string
	AmountFunded  int
	IdeasAccepted int
	Leaderboard   []IdeaScoreInfoSignal
}

type ApprovalInfoSignal struct {
	Reviewer string
}

type RejectionInfoSignal struct {
	Reviewer string
	Comment  string
}

type PledgeInfoSignal struct {
	Amount int
}

type IdeaAcceptedInfoSignal struct {
	IdeaID    string
	Submitter string
}

type IdeaScoreInfoSignal struct {
	IdeaID    string
	Submitter string
	Score     int
}

func ideaScoreKey(ideaID, submitter string) string {
	return ideaID + ":" + submitter
}

func Contest(ctx workflow.Context, request ContestRequest) (ContestResult, error) {
	// State
	// These fields ARE the workflow's durable state. They aren't persisted directly —
	// Temporal reconstructs them by replaying history events (activity completions, signals,
	// timers) through this code. That's why mutations must only happen via deterministic
	// paths (activity results, signal handlers, Workflow.* APIs).
	var ID string = request.ID
	var goal int = request.Goal
	var rules []string = request.Rules
	var awards []string = request.Awards
	var reviewTimeout time.Duration = time.Duration(request.ReviewTimeout) * time.Second
	var fundingTimeout time.Duration = time.Duration(request.FundingTimeout) * time.Second
	var ideasAcceptedTimeout time.Duration = time.Duration(request.IdeasAcceptedTimeout) * time.Second

	var state = "pending"
	var startedAt time.Time = workflow.Now(ctx)
	var reviewer string = ""
	var comment string = ""
	var amountFunded int = 0
	var ideasAccepted int = 0
	var ideaScores = make(map[string]int)
	var leaderboard []IdeaScoreInfoSignal

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 10,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)
	startedAt = workflow.Now(ctx)
	logger.Info("Starting contest workflow", "contestID", request.ID, "startedAt", startedAt)

	// Setup query handlers for the workflow state
	err := workflow.SetQueryHandler(ctx, ContestStateQueryName, func() (ContestResult, error) {
		return ContestResult{
			ID:            request.ID,
			RunningTime:   workflow.Now(ctx).Sub(startedAt),
			State:         state,
			Reviewer:      reviewer,
			Comment:       comment,
			AmountFunded:  amountFunded,
			IdeasAccepted: ideasAccepted,
			Leaderboard:   leaderboard,
		}, nil
	})
	if err != nil {
		return ContestResult{}, err
	}

	// Notify admins that the idea requires human evaluation
	var notifyResult activities.NotifyResult
	err = workflow.ExecuteActivity(ctx, activities.Notify, activities.NotifyRequest{
		ID:      request.ID,
		Message: "Idea requires human evaluation",
	}).Get(ctx, &notifyResult)
	if err != nil {
		return ContestResult{}, err
	}

	// Block until approved, rejected, timed out or Continue-As-New is suggested
	for {
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(workflow.GetSignalChannel(ctx, ContestApproveSignalName), func(c workflow.ReceiveChannel, more bool) {
			var signal ApprovalInfoSignal
			c.Receive(ctx, &signal)
			logger.Info("Contest approved", "contestID", request.ID, "reviewer", signal.Reviewer)
			reviewer = signal.Reviewer
			state = "approved"
		})
		selector.AddReceive(workflow.GetSignalChannel(ctx, ContestRejectSignalName), func(c workflow.ReceiveChannel, more bool) {
			var signal RejectionInfoSignal
			c.Receive(ctx, &signal)
			logger.Info("Contest rejected", "contestID", request.ID, "reviewer", signal.Reviewer, "comment", signal.Comment)
			reviewer = signal.Reviewer
			comment = signal.Comment
			state = "rejected"
		})
		selector.AddFuture(workflow.NewTimer(ctx, reviewTimeout), func(f workflow.Future) {
			logger.Info("Contest approval timed out", "contestID", request.ID)
			state = "approval-timedout"
		})
		selector.Select(ctx)

		if state == "approved" {
			break // break out of the for loop
		}

		if state == "approval-timedout" || state == "rejected" {
			return ContestResult{
				ID:            request.ID,
				RunningTime:   workflow.Now(ctx).Sub(startedAt),
				State:         state,
				Reviewer:      reviewer,
				Comment:       comment,
				AmountFunded:  amountFunded,
				IdeasAccepted: ideasAccepted,
				Leaderboard:   leaderboard,
			}, nil
		}

		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			// Carry current state forward so it is not reset on the new run
			return ContestResult{
				ID:            request.ID,
				RunningTime:   workflow.Now(ctx).Sub(startedAt),
				State:         state,
				Reviewer:      reviewer,
				Comment:       comment,
				AmountFunded:  amountFunded,
				IdeasAccepted: ideasAccepted,
				Leaderboard:   leaderboard,
			}, workflow.NewContinueAsNewError(ctx, Contest, ContestRequest{
				ID:                   ID,
				Goal:                 goal,
				Rules:                rules,
				Awards:               awards,
				IdeasAcceptedTimeout: request.IdeasAcceptedTimeout,
				FundingTimeout:       request.FundingTimeout,
				ReviewTimeout:        request.ReviewTimeout,
			})
		}
	}

	logger.Info("Contest was approved", "contestID", request.ID)

	// Block until funded, timed out or Continue-As-New is suggested
	for {
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(workflow.GetSignalChannel(ctx, ContestPledgeSignalName), func(c workflow.ReceiveChannel, more bool) {
			logger.Info("Contest pledge received", "contestID", request.ID)
			var signal PledgeInfoSignal
			c.Receive(ctx, &signal)
			amountFunded += signal.Amount
			if amountFunded >= goal {
				state = "funded"
			}
		})
		selector.AddFuture(workflow.NewTimer(ctx, fundingTimeout), func(f workflow.Future) {
			logger.Info("Contest funding timed out", "contestID", request.ID)
			state = "funding-timedout"
		})
		selector.Select(ctx)

		if state == "funded" {
			break // break out of the for loop
		}

		if state == "funding-timedout" {
			return ContestResult{
				ID:            request.ID,
				RunningTime:   workflow.Now(ctx).Sub(startedAt),
				State:         state,
				Reviewer:      reviewer,
				Comment:       comment,
				AmountFunded:  amountFunded,
				IdeasAccepted: ideasAccepted,
				Leaderboard:   leaderboard,
			}, nil
		}

		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			// Carry current state forward so it is not reset on the new run
			return ContestResult{
				ID:            request.ID,
				RunningTime:   workflow.Now(ctx).Sub(startedAt),
				State:         state,
				Reviewer:      reviewer,
				Comment:       comment,
				AmountFunded:  amountFunded,
				IdeasAccepted: ideasAccepted,
				Leaderboard:   leaderboard,
			}, workflow.NewContinueAsNewError(ctx, Contest, ContestRequest{
				ID:                   request.ID,
				Goal:                 request.Goal,
				Rules:                request.Rules,
				Awards:               request.Awards,
				IdeasAcceptedTimeout: request.IdeasAcceptedTimeout,
				FundingTimeout:       request.FundingTimeout,
				ReviewTimeout:        request.ReviewTimeout,
			})

		}
	}

	logger.Info("Contest was funded", "contestID", request.ID)

	// Block until timed out or Continue-As-New is suggested
	state = "accepting-ideas-n-scores"
	for {
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(workflow.GetSignalChannel(ctx, ContestIdeaAcceptedSignalName), func(c workflow.ReceiveChannel, more bool) {
			logger.Info("Contest idea received", "contestID", request.ID)
			var signal IdeaAcceptedInfoSignal
			c.Receive(ctx, &signal)
			ideasAccepted += 1
			ideaScores[ideaScoreKey(signal.IdeaID, signal.Submitter)] = 0
			leaderboard = buildLeaderboard(ideaScores)
		})
		selector.AddReceive(workflow.GetSignalChannel(ctx, ContestIdeaScoreSignalName), func(c workflow.ReceiveChannel, more bool) {
			logger.Info("Contest idea score received", "contestID", request.ID)
			var signal IdeaScoreInfoSignal
			c.Receive(ctx, &signal)
			ideaScores[ideaScoreKey(signal.IdeaID, signal.Submitter)] = signal.Score
			leaderboard = buildLeaderboard(ideaScores)
		})
		selector.AddFuture(workflow.NewTimer(ctx, ideasAcceptedTimeout), func(f workflow.Future) {
			logger.Info("Ideas acceptance timed out", "contestID", request.ID)
			state = "ideas-accepted-timedout"
		})
		selector.Select(ctx)

		if state == "ideas-accepted-timedout" {
			break // break out of the for loop
		}

		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			// Carry current state forward so it is not reset on the new run
			return ContestResult{
				ID:            request.ID,
				RunningTime:   workflow.Now(ctx).Sub(startedAt),
				State:         state,
				Reviewer:      reviewer,
				Comment:       comment,
				AmountFunded:  amountFunded,
				IdeasAccepted: ideasAccepted,
				Leaderboard:   leaderboard,
			}, workflow.NewContinueAsNewError(ctx, Contest, ContestRequest{
				ID:                   request.ID,
				Goal:                 request.Goal,
				Rules:                request.Rules,
				Awards:               request.Awards,
				IdeasAcceptedTimeout: request.IdeasAcceptedTimeout,
				FundingTimeout:       request.FundingTimeout,
				ReviewTimeout:        request.ReviewTimeout,
				// TODO: Add leaderboard
			})

		}
	}

	logger.Info("Contest ideas and scores timed out", "contestID", request.ID)

	// The leaderboard is aleady built from idea scores
	logger.Info("Contest ideas leaderboard", "contestID", request.ID, "leaderboard", leaderboard)

	// TODO: Send awards

	return ContestResult{
		ID:            request.ID,
		RunningTime:   workflow.Now(ctx).Sub(startedAt),
		State:         state,
		Reviewer:      reviewer,
		Comment:       comment,
		AmountFunded:  amountFunded,
		IdeasAccepted: ideasAccepted,
		Leaderboard:   leaderboard,
	}, nil
}

func buildLeaderboard(ideaScores map[string]int) []IdeaScoreInfoSignal {
	board := make([]IdeaScoreInfoSignal, 0, len(ideaScores))
	for key, score := range ideaScores {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		board = append(board, IdeaScoreInfoSignal{IdeaID: parts[0], Submitter: parts[1], Score: score})
	}
	sort.Slice(board, func(i, j int) bool {
		if board[i].Score != board[j].Score {
			return board[i].Score > board[j].Score
		}
		if board[i].IdeaID != board[j].IdeaID { // deterministic tie-break
			return board[i].IdeaID < board[j].IdeaID
		}
		return board[i].Submitter < board[j].Submitter
	})
	return board
}
