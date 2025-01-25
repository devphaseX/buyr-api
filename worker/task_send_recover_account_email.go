package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/mailer"
	"github.com/hibiken/asynq"
)

const TaskSendRecoverAccountEmail = "task:send_recover_account_email"

type PayloadSendRecoverAccountEmail struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Token     string `json:"token"`
	Email     string `json:"email"`
	ClientURL string `json:"client_url"`
}

func (rt *RedisTaskDistributor) DistributeTaskSendRecoverAccountEmail(ctx context.Context, payload *PayloadSendRecoverAccountEmail, opts ...asynq.Option) error {
	jsonPayload, err := json.Marshal(payload)

	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	recoverAccountEmailTask := asynq.NewTask(TaskSendRecoverAccountEmail, jsonPayload, opts...)

	taskInfo, err := rt.client.EnqueueContext(ctx,
		recoverAccountEmailTask,
		asynq.Unique(time.Second*5),
		asynq.TaskID(payload.Token),
	)

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

func (processor *RedisTaskProcessor) ProcessTaskSendRecoverAccountEmail(ctx context.Context, task *asynq.Task) error {
	var payload PayloadSendRecoverAccountEmail

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
		ActivationURL: fmt.Sprintf("%s/confirm/%s?user_id=%s", payload.ClientURL, payload.Token, payload.UserID),
	})

	if err != nil {
		return err
	}

	return nil
}
