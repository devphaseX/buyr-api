package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/mailer"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/hibiken/asynq"
)

const TaskSendAdminOnboardEmail = "task:send_admin_onboard_email"

type PayloadSendAdminOnboardEmail struct {
	Username  string           `json:"username"`
	Token     string           `json:"token"`
	Email     string           `json:"email"`
	ClientURL string           `json:"client_url"`
	Role      store.AdminLevel `json:"role"` // Can be useful to specify admin role/level
}

func (rt *RedisTaskDistributor) DistributeTaskSendAdminOnboardEmail(ctx context.Context, payload *PayloadSendAdminOnboardEmail, opts ...asynq.Option) error {
	jsonPayload, err := json.Marshal(payload)

	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	adminOnboardTask := asynq.NewTask(TaskSendAdminOnboardEmail, jsonPayload, opts...)

	taskInfo, err := rt.client.EnqueueContext(ctx,
		adminOnboardTask,
		asynq.Unique(time.Hour*24), // 24 hour expiration
		asynq.TaskID(payload.Token),
	)

	if err != nil {
		return err
	}

	rt.logger.Info(
		"message", "enqueued admin onboarding email task",
		"type", taskInfo.Type,
		"queue", taskInfo.Queue,
		"max_retry", taskInfo.MaxRetry,
	)

	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskSendAdminOnboardEmail(ctx context.Context, task *asynq.Task) error {
	var payload PayloadSendAdminOnboardEmail

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", asynq.SkipRetry)
	}

	err := processor.mailClient.Send(&mailer.MailOption{
		To:           []string{payload.Email},
		TemplateFile: mailer.AdminOnboardTemplate, // You'll need to create this template
	}, struct {
		Username      string
		ActivationURL string
		Platform      string
		Role          store.AdminLevel
	}{
		Username:      payload.Username,
		ActivationURL: fmt.Sprintf("%s/activate-account?token=%s", payload.ClientURL, payload.Token),
		Platform:      "Your Platform Name", // Replace with your platform name
		Role:          payload.Role,
	})

	if err != nil {
		return err
	}

	return nil
}
