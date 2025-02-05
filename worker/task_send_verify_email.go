package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/mailer"
	"github.com/hibiken/asynq"
)

const TaskSendVerifyEmail = "task:send_verify_email"

type PayloadSendVerifyEmail struct {
	Username  string `json:"username"`
	Token     string `json:"token"`
	Email     string `json:"email"`
	ClientURL string `json:"client_url"`
}

func (rt *RedisTaskDistributor) DistributeTaskSendVerifyEmail(ctx context.Context, payload *PayloadSendVerifyEmail, opts ...asynq.Option) error {
	jsonPayload, err := json.Marshal(payload)

	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	activateAccountEmailTask := asynq.NewTask(TaskSendVerifyEmail, jsonPayload, opts...)

	taskInfo, err := rt.client.EnqueueContext(ctx,
		activateAccountEmailTask,
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

func (processor *RedisTaskProcessor) ProcessTaskSendVerifyEmail(ctx context.Context, task *asynq.Task) error {
	var payload PayloadSendVerifyEmail

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", asynq.SkipRetry)
	}

	err := processor.mailClient.Send(&mailer.MailOption{
		To:           []string{payload.Email},
		TemplateFile: mailer.VerifyEmailTemplate,
	}, struct {
		Username        string
		VerificationURL string
		CurrentYear     int
	}{
		Username:        payload.Username,
		VerificationURL: fmt.Sprintf("%s/verify-email/%s", payload.ClientURL, payload.Token),
		CurrentYear:     time.Now().Year(),
	})

	if err != nil {
		return err
	}

	return nil
}
