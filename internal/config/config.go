// config/config.go
package config

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Address         string
	StoreInterval   int
	FileStoragePath string
	Restore         bool
	DatabaseDSN     string
	HashKey         string
	AuditFile       string
	AuditURL        string
}

func Load() *Config {
	cfg := &Config{}

	defaultFileStoragePath := filepath.Join(os.TempDir(), "metrics.json")

	flag.StringVar(&cfg.Address, "a", "localhost:8080", "адрес HTTP-сервера")
	flag.IntVar(&cfg.StoreInterval, "i", 300, "интервал сохранения в секундах")
	flag.StringVar(&cfg.FileStoragePath, "f", defaultFileStoragePath, "путь к файлу метрик")
	flag.BoolVar(&cfg.Restore, "r", true, "загружать метрики при запуске")
	flag.StringVar(&cfg.DatabaseDSN, "d", cfg.DatabaseDSN, "Database connection string")
	flag.StringVar(&cfg.HashKey, "k", "", "ключ подписики по алгоритму sha256")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "audit path logs file")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "audit url push logs")
	flag.Parse()

	cfg.applyEnv()

	return cfg
}

func (cfg *Config) applyEnv() {
	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		cfg.Address = envAddr
	}
	if envInterval := os.Getenv("STORE_INTERVAL"); envInterval != "" {
		if interval, err := strconv.Atoi(envInterval); err == nil {
			cfg.StoreInterval = interval
		}
	}
	if envPath := os.Getenv("FILE_STORAGE_PATH"); envPath != "" {
		cfg.FileStoragePath = envPath
	}
	if envRestore := os.Getenv("RESTORE"); envRestore != "" {
		if restore, err := strconv.ParseBool(envRestore); err == nil {
			cfg.Restore = restore
		}
	}
	if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
		cfg.DatabaseDSN = envDSN
	}
	if envHashKey := os.Getenv("KEY"); envHashKey != "" {
		cfg.HashKey = envHashKey
	}
	if envAuditFile := os.Getenv("AUDIT_FILE"); envAuditFile != "" {
		cfg.AuditFile = envAuditFile
	}
	if envAuditURL := os.Getenv("AUDIT_URL"); envAuditURL != "" {
		cfg.AuditURL = envAuditURL
	}
}

// переводим
func (cfg *Config) GetStoreIntervalDuration() time.Duration {
	return time.Duration(cfg.StoreInterval) * time.Second
}
