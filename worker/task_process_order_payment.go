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

func (rt *RedisTaskProcessor) ProcessTaskConfirmOrderPayment(ctx context.Context, task *asynq.Task) error {
	var payload ProcessPaymentPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Update the order status to "paid".
	payment := &store.Payment{
		OrderID:       payload.OrderID,
		TransactionID: payload.TransactionID,
		Status:        payload.Status,
		PaymentMethod: "stripe",
		Amount:        payload.Amount,
	}
	err := rt.store.Payments.Create(ctx, payment)

	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Log the payment processing.
	rt.logger.Info("payment processed", "successfully", "order_id", payload.OrderID)

	if payment.Status == store.FailedPaymentStatus {
		order, err := rt.store.Orders.GetOrderByID(ctx, payload.OrderID)

		if err != nil && order.PromoCode != "" {
			err = rt.store.Promos.ReleaseUsage(ctx, order.PromoCode)
			if err != nil {
				rt.logger.Error("failed to release promo code usage", "error", err)
			}
		}
	}

	_ = rt.taskDistributor.DistributeTaskOrderConfirmationEmail(ctx, &SendOrderConfirmationEmailPayload{
		OrderID: payload.OrderID,
	})

	return nil
}
