package config

import (
	"os"
	"strconv"
)

// Config holds all runtime configuration for the DBMove backend.
type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	EncryptionKey        string
	ExecutionMode        string // kubernetes | docker | local
	Kubeconfig           string
	K8sNamespace         string
	WorkerImage          string
	WorkerBin            string // used by local execution mode
	APIURL               string // address workers use to reach the backend
	DockerNetwork        string
	MaxConcurrent        int
	DataDir              string
	CORSOrigins          string
	InternalToken        string
	K8sJobRequestsCPU    string
	K8sJobRequestsMemory string
	K8sJobLimitsCPU      string
	K8sJobLimitsMemory   string
	K8sJobTTLSeconds     int
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// Load reads configuration from environment variables.
func Load() *Config {
	return &Config{
		HTTPAddr:             getenv("DBMOVE_HTTP_ADDR", ":8080"),
		DatabaseURL:          getenv("DBMOVE_DATABASE_URL", "postgres://dbmove:dbmove123@localhost:5432/dbmove?sslmode=disable"),
		EncryptionKey:        getenv("DBMOVE_ENCRYPTION_KEY", ""),
		ExecutionMode:        getenv("DBMOVE_EXECUTION_MODE", "docker"),
		Kubeconfig:           getenv("DBMOVE_KUBECONFIG", ""),
		K8sNamespace:         getenv("DBMOVE_K8S_NAMESPACE", "dbmove"),
		WorkerImage:          getenv("DBMOVE_WORKER_IMAGE", "dbmove-worker:local"),
		WorkerBin:            getenv("DBMOVE_WORKER_BIN", "dbmove-worker"),
		APIURL:               getenv("DBMOVE_API_URL", "http://localhost:8080"),
		DockerNetwork:        getenv("DBMOVE_DOCKER_NETWORK", ""),
		MaxConcurrent:        getenvInt("DBMOVE_MAX_CONCURRENT_MIGRATIONS", 3),
		DataDir:              getenv("DBMOVE_DATA_DIR", "/data"),
		CORSOrigins:          getenv("DBMOVE_CORS_ORIGINS", "*"),
		InternalToken:        getenv("DBMOVE_INTERNAL_TOKEN", ""),
		K8sJobRequestsCPU:    getenv("DBMOVE_K8S_JOB_REQUESTS_CPU", "500m"),
		K8sJobRequestsMemory: getenv("DBMOVE_K8S_JOB_REQUESTS_MEMORY", "512Mi"),
		K8sJobLimitsCPU:      getenv("DBMOVE_K8S_JOB_LIMITS_CPU", "4"),
		K8sJobLimitsMemory:   getenv("DBMOVE_K8S_JOB_LIMITS_MEMORY", "4Gi"),
		K8sJobTTLSeconds:     getenvInt("DBMOVE_K8S_JOB_TTL_SECONDS", 3600),
	}
}
