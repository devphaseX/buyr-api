package worker

import (
	"context"

	"github.com/hibiken/asynq"
)

type TaskDistributor interface {
	DistributeTaskSendRecoverAccountEmail(ctx context.Context, payload *PayloadSendRecoverAccountEmail, opts ...asynq.Option) error
	DistributeTaskSendActivateAccountEmail(ctx context.Context, payload *PayloadSendActivateAcctEmail, opts ...asynq.Option) error
	DistributeTaskSendVendorActivationEmail(ctx context.Context, payload *PayloadSendVendorActivationEmail, opts ...asynq.Option) error
	DistributeTaskSendAdminOnboardEmail(ctx context.Context, payload *PayloadSendAdminOnboardEmail, opts ...asynq.Option) error
}

type RedisTaskDistributor struct {
	logger asynq.Logger
	client *asynq.Client
}

func NewTaskDistributor(redisOpt asynq.RedisClientOpt) TaskDistributor {
	client := asynq.NewClient(redisOpt)

	return &RedisTaskDistributor{
		logger: NewLogger(),
		client: client,
	}
}
