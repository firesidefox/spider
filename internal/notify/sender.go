// Package notify provides notification sending for task run results.
package notify

import (
	"context"
	"fmt"

	"github.com/spiderai/spider/internal/models"
)

// Sender sends a notification message to a channel.
type Sender interface {
	Send(ctx context.Context, msg string) error
}

// NewSender returns a Sender for the given channel type and config.
// Returns an error if the channel type is unsupported or config is invalid.
func NewSender(ch *models.NotifyChannel) (Sender, error) {
	switch ch.Type {
	case models.NotifyChannelDingTalk:
		webhookURL, ok := ch.Config["webhook_url"]
		if !ok || webhookURL == "" {
			return nil, fmt.Errorf("dingtalk channel %d missing webhook_url", ch.ID)
		}
		secret := ch.Config["secret"] // optional
		return &dingTalkSender{webhookURL: webhookURL, secret: secret}, nil
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
}

// FormatMessage builds a human-readable notification body for a task run.
func FormatMessage(task *models.Task, run *models.TaskRun) string {
	status := string(run.Status)
	finished := ""
	if run.FinishedAt != nil {
		finished = run.FinishedAt.Format("2006-01-02 15:04:05")
	}
	msg := fmt.Sprintf("[Spider] Task: %s\nStatus: %s\nFinished: %s", task.Name, status, finished)
	if run.Summary != "" {
		msg += "\nSummary: " + run.Summary
	}
	return msg
}
