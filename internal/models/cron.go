package models

import (
	"github.com/go-co-op/gocron/v2"
)

// Cron job structure holding the job definition and the function task to run by the bot.
type CronJob struct {
	Definition gocron.JobDefinition
	Task       gocron.Task
}

// Create a new cron job model.
func NewCronJob(definition gocron.JobDefinition, task gocron.Task) CronJob {
	return CronJob{definition, task}
}
