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
}

func Load() *Config {
	cfg := &Config{}

	defaultFileStoragePath := filepath.Join(os.TempDir(), "metrics.json")

	flag.StringVar(&cfg.Address, "a", "localhost:8080", "адрес HTTP-сервера")
	flag.IntVar(&cfg.StoreInterval, "i", 300, "интервал сохранения в секундах")
	flag.StringVar(&cfg.FileStoragePath, "f", defaultFileStoragePath, "путь к файлу метрик")
	flag.BoolVar(&cfg.Restore, "r", true, "загружать метрики при запуске")

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
}

// переводим
func (cfg *Config) GetStoreIntervalDuration() time.Duration {
	return time.Duration(cfg.StoreInterval) * time.Second
}
