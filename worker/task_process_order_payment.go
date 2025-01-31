package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/hibiken/asynq"
)

const TaskProcessOrderPayment = "task:process_payment"

type ProcessPaymentPayload struct {
	OrderID       string              `json:"order_id"`
	Amount        float64             `json:"amount"`
	TransactionID string              `json:"transaction_id"`
	Status        store.PaymentStatus `json:"status"`
}

func (rt *RedisTaskDistributor) DistributeTaskProcessOrderPayment(ctx context.Context, payload *ProcessPaymentPayload, opts ...asynq.Option) error {
	jsonPayload, err := json.Marshal(payload)

	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	processPaymentTask := asynq.NewTask(TaskProcessOrderPayment, jsonPayload, opts...)

	taskInfo, err := rt.client.EnqueueContext(ctx,
		processPaymentTask,
		asynq.Unique(time.Minute*5),
		asynq.TaskID(payload.OrderID),
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

func (app *RedisTaskProcessor) ProcessTaskConfirmOrderPayment(ctx context.Context, task *asynq.Task) error {
	var payload ProcessPaymentPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Update the order status to "paid".
	err := app.store.Payments.Create(ctx, &store.Payment{
		OrderID:       payload.OrderID,
		TransactionID: payload.TransactionID,
		Status:        payload.Status,
		PaymentMethod: "stripe",
		Amount:        payload.Amount,
	})

	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Log the payment processing.
	app.logger.Info("payment processed", "successfully", "order_id", payload.OrderID)

	// After updating the order status to "paid".
	// payload, err := json.Marshal(SendOrderConfirmationEmailPayload{
	// 		OrderID: orderID,
	// 		Email:   user.Email,
	// })
	// if err != nil {
	// 		return fmt.Errorf("failed to marshal payload: %w", err)
	// }

	// task := asynq.NewTask("send_order_confirmation_email", payload)
	// if _, err := app.asynqClient.Enqueue(task); err != nil {
	// 		return fmt.Errorf("failed to enqueue email task: %w", err)
	// }

	return nil
}
