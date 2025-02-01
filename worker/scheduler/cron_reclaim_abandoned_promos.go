package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
)

var (
	CronReclaimAbandonedPromos = "reclaim_abandoned_promos"
)

const (
	abandonedOrderTimeout = time.Minute * 30
)

func (c *AsyncTaskScheduler) reclaimAbandonedPromos() {
	_, err := c.scheduler.Register("@every 1m", asynq.NewTask(CronReclaimAbandonedPromos, nil))
	if err != nil {
		log.Fatalf("failed to schedule ReclaimAbandonedPromos task: %v", err)
	}
}

func (p *AsyncTaskProcessor) HandleReclaimAbandonedPromos(ctx context.Context, t *asynq.Task) error {
	p.logger.Info("running reclaim abadoned promos")
	// Calculate the cutoff time for abandoned orders
	cutoffTime := time.Now().Add(-abandonedOrderTimeout)

	// Fetch abandoned orders
	abandonedOrders, err := p.store.Orders.GetAbandonedOrders(ctx, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to fetch abandoned orders: %w", err)
	}

	// Release promo codes for abandoned orders
	for _, order := range abandonedOrders {
		if order.PromoCode != "" {
			err := p.store.Promos.ReleaseUsage(ctx, order.PromoCode)
			if err != nil {
				// Log the error and continue processing other orders
				log.Printf("failed to release promo code for order %s: %v", order.ID, err)
				continue
			}
		}

		// Mark the order as expired
		err := p.store.Orders.UpdateStatus(ctx, order.ID, "expired")
		if err != nil {
			log.Printf("failed to update status for order %s: %v", order.ID, err)
		}
	}

	return nil
}
