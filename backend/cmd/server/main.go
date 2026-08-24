package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dbmove/dbmove/backend/internal/api"
	"github.com/dbmove/dbmove/backend/internal/config"
	"github.com/dbmove/dbmove/backend/internal/crypto"
	"github.com/dbmove/dbmove/backend/internal/dispatcher"
	"github.com/dbmove/dbmove/backend/internal/migrate"
	"github.com/dbmove/dbmove/backend/internal/repository"
	"github.com/dbmove/dbmove/backend/internal/runner"
	"github.com/dbmove/dbmove/backend/internal/service"
	"github.com/dbmove/dbmove/backend/internal/sse"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	cfg := config.Load()
	gin.SetMode(gin.ReleaseMode)

	cipher, err := crypto.NewCipher(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("crypto: %v", err)
	}

	db := connectDB(cfg.DatabaseURL)
	if err := migrate.Auto(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	repo := repository.New(db)
	hub := sse.NewHub()
	connSvc := service.NewConnectionService(repo, cipher)
	migSvc := service.NewMigrationService(repo)

	watcher := &runner.Watcher{Repo: repo, Hub: hub}
	var execRunner runner.Runner
	switch cfg.ExecutionMode {
	case "kubernetes", "k8s":
		k8s, err := runner.NewK8sRunner(cfg.Kubeconfig, cfg.K8sNamespace, cfg.WorkerImage, cfg.APIURL,
			runner.K8sResources{
				RequestsCPU:    cfg.K8sJobRequestsCPU,
				RequestsMemory: cfg.K8sJobRequestsMemory,
				LimitsCPU:      cfg.K8sJobLimitsCPU,
				LimitsMemory:   cfg.K8sJobLimitsMemory,
			}, cfg.K8sJobTTLSeconds, watcher)
		if err != nil {
			log.Fatalf("kubernetes runner: %v", err)
		}
		k8s.InternalToken = cfg.InternalToken
		execRunner = k8s
	case "local":
		execRunner = &runner.LocalRunner{WorkerBin: cfg.WorkerBin, APIURL: cfg.APIURL, InternalToken: cfg.InternalToken, Watcher: watcher}
	default:
		execRunner = &runner.DockerRunner{
			WorkerImage:   cfg.WorkerImage,
			APIURL:        cfg.APIURL,
			DataDir:       cfg.DataDir,
			Network:       cfg.DockerNetwork,
			InternalToken: cfg.InternalToken,
			Watcher:       watcher,
		}
	}
	log.Printf("execution mode: %s (worker image: %s)", cfg.ExecutionMode, cfg.WorkerImage)

	handlers := api.NewHandlers(cfg, repo, hub, connSvc, migSvc, func(ctx context.Context, id uint64) error {
		if cleaner, ok := execRunner.(runner.Cleanup); ok {
			return cleaner.Cleanup(ctx, id)
		}
		return execRunner.Cancel(ctx, id)
	})

	disp := dispatcher.New(repo, execRunner, cipher, cfg.MaxConcurrent)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go disp.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handlers.Build(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("DBMove API listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func connectDB(dsn string) *gorm.DB {
	var db *gorm.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err == nil {
			sqlDB, derr := db.DB()
			if derr == nil {
				derr = sqlDB.Ping()
			}
			if derr == nil {
				return db
			}
			err = derr
		}
		log.Printf("waiting for database... (%v)", err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("cannot connect to database: %v", err)
	return nil
}
