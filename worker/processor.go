package worker

import (
	"context"

	"github.com/devphaseX/buyr-api.git/internal/mailer"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/internal/store/cache"
	"github.com/hibiken/asynq"
)

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
)

type CronTaskRunner interface {
	MountTasks(*asynq.ServeMux)
}

type TaskProcessor interface {
	Start() error
	Close()
	ProcessTaskSendActivateAcctEmail(ctx context.Context, task *asynq.Task) error
	ProcessTaskConfirmOrderPayment(ctx context.Context, task *asynq.Task) error
	ProcessSendOrderConfirmationEmailTask(ctx context.Context, task *asynq.Task) error
	ProcessTaskSendRecoverAccountEmail(ctx context.Context, task *asynq.Task) error
	ProcessTaskSendVendorActivationEmail(ctx context.Context, task *asynq.Task) error
	ProcessTaskSendAdminOnboardEmail(ctx context.Context, task *asynq.Task) error
}

type RedisTaskProcessor struct {
	server         *asynq.Server
	store          *store.Storage
	cachestore     *cache.Storage
	logger         asynq.Logger
	mailClient     mailer.Client
	cronTaskRunner CronTaskRunner
}

func NewRedisTaskProcessor(redisOpt asynq.RedisClientOpt, cronTaskRunner CronTaskRunner, store *store.Storage, cacheStore *cache.Storage, mailClient mailer.Client) TaskProcessor {
	logger := NewLogger()
	server := asynq.NewServer(redisOpt, asynq.Config{
		Queues: map[string]int{
			QueueCritical: 10,
			QueueDefault:  5,
		},

		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			logger.Error(
				"message", "failed to process task", "type",
				task.Type(), "payload", task.Payload(),
				"err", err,
			)

		}),
		Concurrency: 10,
		Logger:      logger,
	})

	return &RedisTaskProcessor{
		server:         server,
		store:          store,
		cachestore:     cacheStore,
		cronTaskRunner: cronTaskRunner,
		mailClient:     mailClient,
		logger:         NewLogger(),
	}
}

func (processor *RedisTaskProcessor) Start() error {
	mux := asynq.NewServeMux()

	mux.HandleFunc(TaskSendActivateAccountEmail, processor.ProcessTaskSendActivateAcctEmail)
	mux.HandleFunc(TaskSendRecoverAccountEmail, processor.ProcessTaskSendRecoverAccountEmail)
	mux.HandleFunc(TaskSendVendorActivationEmail, processor.ProcessTaskSendVendorActivationEmail)
	mux.HandleFunc(TaskSendAdminOnboardEmail, processor.ProcessTaskSendAdminOnboardEmail)
	mux.HandleFunc(TaskProcessOrderPayment, processor.ProcessTaskConfirmOrderPayment)
	mux.HandleFunc(TaskSendOrderConfirmationEmail, processor.ProcessSendOrderConfirmationEmailTask)

	if processor.cronTaskRunner != nil {
		processor.cronTaskRunner.MountTasks(mux)
	}

	return processor.server.Start(mux)
}

func (processor *RedisTaskProcessor) Close() {
	processor.server.Shutdown()
}
