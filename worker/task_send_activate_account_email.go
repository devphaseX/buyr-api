package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devphaseX/buyr-api.git/internal/mailer"
	"github.com/hibiken/asynq"
)

const TaskSendActivateAccountEmail = "task:send_activate_account_email"

type PayloadSendActivateAcctEmail struct {
	Username  string `json:"username"`
	Token     string `json:"token"`
	Email     string `json:"email"`
	ClientURL string `json:"client_url"`
}

func (rt *RedisTaskDistributor) DistributeTaskSendActivateAccountEmail(ctx context.Context, payload *PayloadSendActivateAcctEmail, opts ...asynq.Option) error {
	jsonPayload, err := json.Marshal(payload)

	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	activateAccountEmailTask := asynq.NewTask(TaskSendActivateAccountEmail, jsonPayload, opts...)

	taskInfo, err := rt.client.EnqueueContext(ctx, activateAccountEmailTask)

	if err != nil {
		return err
	}

	rt.logger.Info(
		"message", "enqueued task",
		"type", taskInfo.Type,
		"queue", taskInfo.Queue,
		"max_retry", taskInfo.MaxRetry,
	)

	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskSendActivateAcctEmail(ctx context.Context, task *asynq.Task) error {
	var payload PayloadSendActivateAcctEmail

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", asynq.SkipRetry)
	}

	err := processor.mailClient.Send(&mailer.MailOption{
		To:           []string{payload.Email},
		TemplateFile: mailer.ActivateAccountEmailTemplate,
	}, struct {
		Username      string
		ActivationURL string
	}{
		Username:      payload.Username,
		ActivationURL: fmt.Sprintf("%s/confirm/%s", payload.ClientURL, payload.Token),
	})

	return err
}
