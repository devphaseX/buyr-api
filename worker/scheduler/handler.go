package scheduler

import (
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/worker"
	"github.com/hibiken/asynq"
)

type AsyncTaskProcessor struct {
	store  *store.Storage
	logger asynq.Logger
}

func NewAsyncTaskProcessor(
	redisOpt asynq.RedisClientOpt,
	store *store.Storage,
) *AsyncTaskProcessor {

	logger := worker.NewLogger()

	return &AsyncTaskProcessor{
		store:  store,
		logger: logger,
	}
}

func (p *AsyncTaskProcessor) MountTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(CronReclaimAbandonedPromos, p.HandleReclaimAbandonedPromos)
}
