package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dbmove/dbmove/worker/internal/engine"
	"github.com/dbmove/dbmove/worker/internal/reporter"
)

func main() {
	log.SetFlags(log.LstdFlags)

	taskID, err := strconv.ParseUint(os.Getenv("TASK_ID"), 10, 64)
	if err != nil || taskID == 0 {
		log.Fatalf("TASK_ID is required")
	}
	apiBase := strings.TrimRight(os.Getenv("DBMOVE_API"), "/")
	if apiBase == "" {
		log.Fatalf("DBMOVE_API is required")
	}
	token := os.Getenv("DBMOVE_INTERNAL_TOKEN")
	srcPass := os.Getenv("DBMOVE_SOURCE_PASSWORD")
	tgtPass := os.Getenv("DBMOVE_TARGET_PASSWORD")
	dataDir := os.Getenv("DBMOVE_DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}

	rep := reporter.New(apiBase, taskID, token, srcPass, tgtPass)
	defer rep.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		rep.Log("WARN", "worker interrupted, cancelling migration")
		rep.Status("CANCELLED", "worker interrupted")
		rep.Close()
		os.Exit(130)
	}()

	cfg, err := fetchTask(context.Background(), apiBase, taskID, token)
	if err != nil {
		rep.Status("FAILED", "cannot fetch task config: "+err.Error())
		log.Fatalf("fetch task config: %v", err)
	}
	rep.Log("INFO", "task %d (%s) started, engine=%s", cfg.ID, cfg.Name, cfg.Engine)

	eng, err := engine.Get(cfg.Engine)
	if err != nil {
		rep.Status("FAILED", err.Error())
		log.Fatalf("engine: %v", err)
	}
	env := &engine.Env{
		SourcePassword: srcPass,
		TargetPassword: tgtPass,
		DataDir:        dataDir,
	}

	rep.Status("PREPARING", "")
	if err := eng.Preflight(context.Background(), cfg, env, rep); err != nil {
		rep.Log("ERROR", "preflight failed: %s", err)
		rep.Status("FAILED", err.Error())
		os.Exit(1)
	}

	rep.Status("RUNNING", "")
	if err := eng.Migrate(context.Background(), cfg, env, rep); err != nil {
		rep.Log("ERROR", "migration failed: %s", err)
		rep.Status("FAILED", err.Error())
		os.Exit(1)
	}
	rep.Status("SUCCESS", "")
	rep.Log("INFO", "migration completed")
}

func fetchTask(ctx context.Context, apiBase string, taskID uint64, token string) (*engine.TaskConfig, error) {
	url := fmt.Sprintf("%s/api/v1/internal/tasks/%d", apiBase, taskID)
	var lastErr error
	for i := 0; i < 10; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		var envelope struct {
			Success bool              `json:"success"`
			Data    engine.TaskConfig `json:"data"`
			Error   *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			resp.Body.Close()
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		resp.Body.Close()
		if !envelope.Success {
			msg := "unknown error"
			if envelope.Error != nil {
				msg = envelope.Error.Message
			}
			return nil, fmt.Errorf("backend: %s", msg)
		}
		return &envelope.Data, nil
	}
	return nil, fmt.Errorf("cannot reach backend after retries: %w", lastErr)
}
