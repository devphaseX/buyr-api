package scheduler

import (
	"github.com/devphaseX/buyr-api.git/worker"
	"github.com/hibiken/asynq"
)

type AsyncTaskScheduler struct {
	scheduler *asynq.Scheduler
	logger    asynq.Logger
}

func NewAsyncTaskScheduler(redisOpt asynq.RedisClientOpt, options *asynq.SchedulerOpts) *AsyncTaskScheduler {
	if options == nil {
		options = &asynq.SchedulerOpts{}
	}

	if options.Logger == nil {
		options.Logger = worker.NewLogger()
	}
	scheduler := asynq.NewScheduler(
		redisOpt,
		options,
	)

	logger := worker.NewLogger()
	return &AsyncTaskScheduler{
		scheduler: scheduler,
		logger:    logger,
	}
}

func (c *AsyncTaskScheduler) Run() {
	if err := c.scheduler.Run(); err != nil {
		c.logger.Fatal("failed to start scheduler: ", err)
	}
}

func (s *AsyncTaskScheduler) RegisterTasks() {
	s.reclaimAbandonedPromos()
}

func (c *AsyncTaskScheduler) Close() {
	c.scheduler.Shutdown()
}
