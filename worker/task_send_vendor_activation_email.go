package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/mailer"
	"github.com/hibiken/asynq"
)

const TaskSendVendorActivationEmail = "task:send_vendor_activation_email"

type PayloadSendVendorActivationEmail struct {
	Username  string `json:"username"`
	Token     string `json:"token"`
	Email     string `json:"email"`
	ClientURL string `json:"client_url"`
}

func (rt *RedisTaskDistributor) DistributeTaskSendVendorActivationEmail(ctx context.Context, payload *PayloadSendVendorActivationEmail, opts ...asynq.Option) error {
	jsonPayload, err := json.Marshal(payload)

	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	vendorActivationTask := asynq.NewTask(TaskSendVendorActivationEmail, jsonPayload, opts...)

	taskInfo, err := rt.client.EnqueueContext(ctx,
		vendorActivationTask,
		asynq.Unique(time.Hour*24), // 24 hour expiration
		asynq.TaskID(payload.Token),
	)

	if err != nil {
		return err
	}

	rt.logger.Info(
		"message", "enqueued vendor activation email task",
		"type", taskInfo.Type,
		"queue", taskInfo.Queue,
		"max_retry", taskInfo.MaxRetry,
	)

	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskSendVendorActivationEmail(ctx context.Context, task *asynq.Task) error {
	var payload PayloadSendVendorActivationEmail

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", asynq.SkipRetry)
	}

	err := processor.mailClient.Send(&mailer.MailOption{
		To:           []string{payload.Email},
		TemplateFile: mailer.VendorActivationTemplate,
	}, struct {
		Username      string
		ActivationURL string
		Platform      string
	}{
		Username:      payload.Username,
		ActivationURL: fmt.Sprintf("%s/activate-account?token=%s", payload.ClientURL, payload.Token),
		Platform:      "Buyr",
	})

	if err != nil {
		return err
	}

	return nil
}
