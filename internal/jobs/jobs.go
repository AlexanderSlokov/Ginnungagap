// Package jobs runs background jobs on Valkey with asynq (ADR-0004).
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"github.com/AlexanderSlokov/Ginnungagap/internal/gc"
)

const (
	TypeGCMark  = "gc:mark"
	TypeGCSweep = "gc:sweep"
)

// Run starts the worker and the hourly GC schedule. It stops when ctx is done.
func Run(ctx context.Context, addr string, g *gc.GC) error {
	opt := asynq.RedisClientOpt{Addr: addr}
	srv := asynq.NewServer(opt, asynq.Config{Concurrency: 2})
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeGCMark, func(ctx context.Context, _ *asynq.Task) error {
		_, err := g.Mark(ctx)
		return err
	})
	mux.HandleFunc(TypeGCSweep, func(ctx context.Context, _ *asynq.Task) error {
		_, err := g.Sweep(ctx)
		return err
	})
	if err := srv.Start(mux); err != nil {
		return err
	}
	defer srv.Shutdown()

	sched := asynq.NewScheduler(opt, nil)
	for _, t := range []string{TypeGCMark, TypeGCSweep} {
		if _, err := sched.Register("@every 1h", asynq.NewTask(t, nil), asynq.Unique(50*time.Minute)); err != nil {
			return err
		}
	}
	if err := sched.Start(); err != nil {
		return err
	}
	defer sched.Shutdown()
	slog.Info("job worker started")
	<-ctx.Done()
	return nil
}

type Client struct{ c *asynq.Client }

func NewClient(addr string) *Client {
	return &Client{asynq.NewClient(asynq.RedisClientOpt{Addr: addr})}
}

func (c *Client) Close() error { return c.c.Close() }

// RunGC puts a mark job and a sweep job in the queue now.
func (c *Client) RunGC(ctx context.Context) error {
	for _, t := range []string{TypeGCMark, TypeGCSweep} {
		if _, err := c.c.EnqueueContext(ctx, asynq.NewTask(t, nil)); err != nil {
			return err
		}
	}
	return nil
}
