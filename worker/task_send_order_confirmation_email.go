package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/mailer"
	"github.com/hibiken/asynq"
)

var TaskSendOrderConfirmationEmail = "send_order_confirmation_email"

type SendOrderConfirmationEmailPayload struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
}

func (rt *RedisTaskDistributor) DistributeTaskOrderConfirmationEmail(ctx context.Context, payload *PayloadSendActivateAcctEmail, opts ...asynq.Option) error {
	jsonPayload, err := json.Marshal(payload)

	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	orderConfirmationEmailTask := asynq.NewTask(TaskSendOrderConfirmationEmail, jsonPayload, opts...)

	taskInfo, err := rt.client.EnqueueContext(ctx,
		orderConfirmationEmailTask,
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

func (app *RedisTaskProcessor) ProcessSendOrderConfirmationEmailTask(ctx context.Context, task *asynq.Task) error {
	var payload SendOrderConfirmationEmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Fetch the order details.
	order, err := app.store.Orders.GetOrderByID(ctx, payload.OrderID)
	if err != nil {
		return fmt.Errorf("failed to fetch order: %w", err)
	}

	// Send the order confirmation email.
	// //payload.Email, ,
	err = app.mailClient.Send(&mailer.MailOption{
		To:           []string{payload.Email},
		TemplateFile: "order_confirmation.html",
	}, map[string]interface{}{
		"OrderID": order.ID,
		"Total":   order.TotalAmount,
	})

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	// Log the email sending.
	app.logger.Info("order confirmation email sent", "order_id", payload.OrderID, "email", payload.Email)

	return nil
}
