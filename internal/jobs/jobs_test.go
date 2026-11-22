package jobs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

// TestValkey runs one job on Valkey, to check that asynq works with Valkey (ADR-0004).
func TestValkey(t *testing.T) {
	addr := os.Getenv("TEST_VALKEY_ADDR")
	if addr == "" {
		t.Skip("TEST_VALKEY_ADDR is not set")
	}
	opt := asynq.RedisClientOpt{Addr: addr}
	srv := asynq.NewServer(opt, asynq.Config{Concurrency: 1, Queues: map[string]int{"test": 1}})
	ran := make(chan string, 1)
	mux := asynq.NewServeMux()
	mux.HandleFunc("test:echo", func(_ context.Context, task *asynq.Task) error {
		ran <- string(task.Payload())
		return nil
	})
	if err := srv.Start(mux); err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	c := asynq.NewClient(opt)
	defer c.Close()
	if _, err := c.Enqueue(asynq.NewTask("test:echo", []byte("hello")), asynq.Queue("test")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-ran:
		if got != "hello" {
			t.Errorf("payload = %q", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the job did not run")
	}
}
