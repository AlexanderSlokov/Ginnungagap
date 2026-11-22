// Command ginnungagap runs the package proxy.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"

	"github.com/AlexanderSlokov/Ginnungagap/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	s3, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseTLS,
	})
	if err != nil {
		return err
	}

	valkey := redis.NewClient(&redis.Options{Addr: cfg.ValkeyAddr})
	defer valkey.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := check(ctx, db, s3, cfg.S3Bucket, valkey); err != nil {
			// Keep the details in the log. Do not show them to unauthenticated clients.
			slog.Warn("health check failed", "err", err)
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok\n"))
	})

	srv := &http.Server{Addr: cfg.Listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	slog.Info("listening", "addr", cfg.Listen)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// check returns an error if PostgreSQL, the S3 bucket, or Valkey is not reachable.
func check(ctx context.Context, db *pgxpool.Pool, s3 *minio.Client, bucket string, valkey *redis.Client) error {
	if err := db.Ping(ctx); err != nil {
		return errors.Join(errors.New("postgres"), err)
	}
	ok, err := s3.BucketExists(ctx, bucket)
	if err != nil {
		return errors.Join(errors.New("s3"), err)
	}
	if !ok {
		return errors.New("s3: bucket " + bucket + " does not exist")
	}
	if err := valkey.Ping(ctx).Err(); err != nil {
		return errors.Join(errors.New("valkey"), err)
	}
	return nil
}
