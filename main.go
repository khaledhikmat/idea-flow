package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/khaledhikmat/idea-flow/application/workflows"
	"github.com/khaledhikmat/idea-flow/helpers"
)

func main() {
	if len(os.Args) < 2 {
		printUsageAndExit()
	}

	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	ctx := context.Background()

	workflowID := "contest-workflow-" + fmt.Sprint(time.Now().Unix())
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: helpers.WorkerQueueName,
	}

	we, startErr := c.ExecuteWorkflow(ctx, options, workflows.Contest, workflows.ContestRequest{
		ID:                   workflowID,
		Goal:                 100,
		Rules:                []string{"rule1", "rule2"},
		Awards:               []string{"award1", "award2", "award3"},
		ReviewTimeout:        60,
		FundingTimeout:       60,
		IdeasAcceptedTimeout: 60,
	})
	if startErr != nil {
		log.Fatalln("Unable to execute workflow", startErr)
	}
	log.Println("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())

	switch os.Args[1] {
	case "happy":
		signalErr := c.SignalWorkflow(ctx, workflowID, "", workflows.ContestApproveSignalName, workflows.ApprovalInfoSignal{
			Reviewer: "John",
		})
		if signalErr != nil {
			log.Fatalln("Unable to signal workflow", signalErr)
		}
		log.Println("Sent signal", "workflowID", workflowID, "signal", workflows.ContestApproveSignalName)

		time.Sleep(2 * time.Second)
		log.Println("Happy path signal sent and waited for 2 seconds")

		signalErr = c.SignalWorkflow(ctx, workflowID, "", workflows.ContestPledgeSignalName, workflows.PledgeInfoSignal{
			Amount: 100,
		})
		if signalErr != nil {
			log.Fatalln("Unable to signal workflow", signalErr)
		}
		log.Println("Sent signal", "workflowID", workflowID, "signal", workflows.ContestPledgeSignalName)

		time.Sleep(2 * time.Second)
		log.Println("Funding signal sent and waited for 2 seconds")

		// Start idea workflows in new threads
		go launchIdea(ctx, c, 0, workflowID)
		go launchIdea(ctx, c, 1, workflowID)
		go launchIdea(ctx, c, 2, workflowID)
		go launchIdea(ctx, c, 3, workflowID)
		go launchIdea(ctx, c, 4, workflowID)

		log.Println("All signals dispatched", "workflowID", workflowID)
		time.Sleep(31 * time.Second)
		log.Println("Waited for 31 seconds")

		qry, queryErr := c.QueryWorkflow(ctx, workflowID, "", workflows.ContestStateQueryName)
		if queryErr != nil {
			log.Fatalln("Unable to query contest workflow", queryErr)
		}
		var result workflows.ContestResult
		if err = qry.Get(&result); err != nil {
			log.Fatalln("Unable to decode query response", err)
		}

		fmt.Println("contest workflow query result:", result)

		run := c.GetWorkflow(ctx, workflowID, "")
		if err = run.Get(ctx, &result); err != nil {
			log.Fatalln("Unable get contest workflow result", err)
		}

		fmt.Println("contest workflow result:", result)

	default:
		printUsageAndExit()
	}
}

func printUsageAndExit() {
	fmt.Println("Usage:")
	fmt.Println("go run main.go happy")
	os.Exit(1)
}

var submitters = []string{"Alice", "Bob", "Charlie", "David", "Eve"}
var ideas = []string{"New innovative idea", "Another great idea", "Yet another idea", "A fourth idea", "A fifth idea"}

func launchIdea(ctx context.Context, cl client.Client, ideaIndex int, parentWorkflowID string) {
	ideaWorkflowID := "contest-idea-" + fmt.Sprintf("%d_%d", time.Now().Unix(), ideaIndex)
	options := client.StartWorkflowOptions{
		ID:        ideaWorkflowID,
		TaskQueue: helpers.WorkerQueueName,
	}

	we, startErr := cl.ExecuteWorkflow(ctx, options, workflows.Idea, workflows.IdeaRequest{
		ContestID:     parentWorkflowID,
		Idea:          ideas[ideaIndex],
		Submitter:     submitters[ideaIndex],
		ReviewTimeout: 60,
	})
	if startErr != nil {
		log.Fatalln("Unable to execute workflow", startErr)
	}
	log.Println("Started idea workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())

	var result workflows.IdeaResult
	for attempt := 0; attempt < 3; attempt++ {
		qry, queryErr := cl.QueryWorkflow(ctx, we.GetID(), "", workflows.IdeaStateQueryName)
		if queryErr != nil {
			log.Fatalln("Unable to query workflow", queryErr)
		}
		if err := qry.Get(&result); err != nil {
			log.Fatalln("Unable to decode query response", err)
		}

		fmt.Println("idea workflow query result:", attempt, result)
		time.Sleep(2 * time.Second)
	}

	//TODO: Make sure the idea is 'accepted' before signaling the contest workflow
	signalErr := cl.SignalWorkflow(ctx, parentWorkflowID, "", workflows.ContestIdeaScoreSignalName, workflows.IdeaScoreInfoSignal{
		IdeaID:    result.ID,
		Submitter: submitters[ideaIndex],
		Score:     10,
	})
	if signalErr != nil {
		log.Fatalln("Unable to signal workflow", signalErr)
	}

	run := cl.GetWorkflow(ctx, we.GetID(), "")
	if err := run.Get(ctx, &result); err != nil {
		log.Fatalln("Unable get workflow result", err)
	}

	fmt.Println("idea workflow result:", result)
}
