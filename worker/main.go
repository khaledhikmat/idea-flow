package main

import (
	"log"

	"github.com/khaledhikmat/idea-flow/application/activities"
	"github.com/khaledhikmat/idea-flow/application/workflows"
	"github.com/khaledhikmat/idea-flow/helpers"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	w := worker.New(c, helpers.WorkerQueueName, worker.Options{})

	w.RegisterWorkflow(workflows.Contest)
	w.RegisterWorkflow(workflows.Idea)
	w.RegisterActivity(activities.Submit)
	w.RegisterActivity(activities.Evaluate)
	w.RegisterActivity(activities.Notify)
	w.RegisterActivity(activities.Escalate)
	w.RegisterActivity(activities.TransitionState)
	w.RegisterActivity(activities.Activate)
	w.RegisterActivity(activities.Classify)
	w.RegisterActivity(activities.Store)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
