package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// TLSConfig holds paths to the TLS certificate and key.
// Leave both empty to run without TLS (development mode).
// Set via TLS_CERT_FILE / TLS_KEY_FILE environment variables in K8s.
type TLSConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// Config is the top-level application configuration.
type Config struct {
	App           AppConfig           `yaml:"app"`
	Postgres      PostgresConfig      `yaml:"postgres"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	Redis         RedisConfig         `yaml:"redis"`
	Kafka         KafkaConfig         `yaml:"kafka"`
	Search        SearchConfig        `yaml:"search"`
	TLS           TLSConfig           `yaml:"tls"`
}

type AppConfig struct {
	Port           int    `yaml:"port"`
	Env            string `yaml:"env"`
	WarmupDatasets int    `yaml:"warmup_datasets"`
}

type PostgresConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	DBName         string `yaml:"dbname"`
	MaxConnections int    `yaml:"max_connections"`
	MaxIdle        int    `yaml:"max_idle"`
}

// DSN returns a PostgreSQL connection string.
func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		p.Host, p.Port, p.User, p.Password, p.DBName,
	)
}

type ElasticsearchConfig struct {
	Host            string        `yaml:"host"`
	IndexPrefix     string        `yaml:"index_prefix"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	Password        string        `yaml:"password"` // injected via ES_PASSWORD env var in K8s
}

type RedisConfig struct {
	Host           string `yaml:"host"`
	Password       string `yaml:"password"`        // injected via REDIS_PASSWORD env var in K8s
	MaxMemory      string `yaml:"max_memory"`
	EvictionPolicy string `yaml:"eviction_policy"`
	MemoryCapacity int    `yaml:"memory_capacity"` // in-process LRU entry limit (default 1000)
}

type KafkaConfig struct {
	Broker               string      `yaml:"broker"`
	Topics               KafkaTopics `yaml:"topics"`
	ConsumerLagThreshold int         `yaml:"consumer_lag_threshold"`
}

type KafkaTopics struct {
	Upserted string `yaml:"upserted"`
	Deleted  string `yaml:"deleted"`
	Changed  string `yaml:"changed"`
}

type SearchConfig struct {
	InMemoryLimit            int           `yaml:"in_memory_limit"`
	BleveFileLimit           int           `yaml:"bleve_file_limit"`
	BleveDataDir             string        `yaml:"bleve_data_dir"`       // Phase 5: directory for file-backed indexes
	DefaultResultLimit       int           `yaml:"default_result_limit"` // Phase 5: max hits per search response
	StabilityThreshold       float64       `yaml:"stability_threshold"`
	StabilityTick            float64       `yaml:"stability_tick"`
	StabilityDecay           float64       `yaml:"stability_decay"`
	TierUpgradeConfirmations int           `yaml:"tier_upgrade_confirmations"`
	OutboxPollInterval       time.Duration `yaml:"outbox_poll_interval"`
	OutboxMaxAttempts        int           `yaml:"outbox_max_attempts"`
	BatchSize                int           `yaml:"batch_size"`
	WorkerCount              int           `yaml:"worker_count"`
	ReconcileInterval        time.Duration `yaml:"reconcile_interval"` // Phase 9: drift detection cycle (default 5m)
}

// Load reads and parses the YAML config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.applyEnvOverrides()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides replaces config fields with environment variable values
// when set. This allows K8s Secrets to inject sensitive credentials without
// embedding them in the ConfigMap or config.yaml, and lets docker-compose
// override localhost hostnames with container service names.
func (c *Config) applyEnvOverrides() {
	// Hosts (docker-compose service names / K8s service DNS)
	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		c.Postgres.Host = v
	}
	if v := os.Getenv("POSTGRES_USER"); v != "" {
		c.Postgres.User = v
	}
	if v := os.Getenv("POSTGRES_DB"); v != "" {
		c.Postgres.DBName = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		c.Redis.Host = v
	}
	if v := os.Getenv("ELASTICSEARCH_HOST"); v != "" {
		c.Elasticsearch.Host = v
	}
	if v := os.Getenv("KAFKA_BROKER"); v != "" {
		c.Kafka.Broker = v
	}
	// Secrets (injected via K8s Secrets or .env)
	if v := os.Getenv("POSTGRES_PASSWORD"); v != "" {
		c.Postgres.Password = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		c.Redis.Password = v
	}
	if v := os.Getenv("ES_PASSWORD"); v != "" {
		c.Elasticsearch.Password = v
	}
	if v := os.Getenv("TLS_CERT_FILE"); v != "" {
		c.TLS.CertFile = v
	}
	if v := os.Getenv("TLS_KEY_FILE"); v != "" {
		c.TLS.KeyFile = v
	}
}

func (c *Config) validate() error {
	if c.App.Port == 0 {
		return fmt.Errorf("app.port is required")
	}
	if c.Postgres.Host == "" {
		return fmt.Errorf("postgres.host is required")
	}
	if c.Postgres.DBName == "" {
		return fmt.Errorf("postgres.dbname is required")
	}
	if c.TLS.CertFile != "" && c.TLS.KeyFile == "" {
		return fmt.Errorf("tls.key_file is required when tls.cert_file is set")
	}
	if c.TLS.KeyFile != "" && c.TLS.CertFile == "" {
		return fmt.Errorf("tls.cert_file is required when tls.key_file is set")
	}
	return nil
}
