package worker

import (
	"context"

	"github.com/hibiken/asynq"
)

type TaskDistributor interface {
	DistributeTaskSendActivateAccountEmail(ctx context.Context, payload *PayloadSendActivateAcctEmail, opts ...asynq.Option) error
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
