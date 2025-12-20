// Package config
package config

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Сохраняем оригинальные флаги и окружение
	originalArgs := os.Args
	originalEnv := make(map[string]string)
	envVars := []string{"ADDRESS", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE", "DATABASE_DSN", "KEY"}
	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	defer func() {
		os.Args = originalArgs
		for _, env := range envVars {
			if val, ok := originalEnv[env]; ok {
				os.Setenv(env, val)
			} else {
				os.Unsetenv(env)
			}
		}
		// Сбрасываем флаги для следующих тестов
		resetFlags()
	}()

	t.Run("загружает значения по умолчанию", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		cfg := Load()

		assert.Equal(t, "localhost:8080", cfg.Address)
		assert.Equal(t, 300, cfg.StoreInterval)
		assert.Equal(t, filepath.Join(os.TempDir(), "metrics.json"), cfg.FileStoragePath)
		assert.True(t, cfg.Restore)
		assert.Equal(t, "", cfg.DatabaseDSN)
		assert.Equal(t, "", cfg.HashKey)
	})

	t.Run("загружает значения из флагов", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test",
			"-a", "127.0.0.1:9090",
			"-i", "60",
			"-f", "/tmp/test.json",
			"-r=false",
			"-d", "postgres://test:test@localhost:5432/testdb",
			"-k", "secret-key",
		}

		cfg := Load()

		assert.Equal(t, "127.0.0.1:9090", cfg.Address)
		assert.Equal(t, 60, cfg.StoreInterval)
		assert.Equal(t, "/tmp/test.json", cfg.FileStoragePath)
		assert.False(t, cfg.Restore)
		assert.Equal(t, "postgres://test:test@localhost:5432/testdb", cfg.DatabaseDSN)
		assert.Equal(t, "secret-key", cfg.HashKey)
	})

	t.Run("переменные окружения имеют приоритет над флагами", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test",
			"-a", "127.0.0.1:9090",
			"-i", "60",
			"-f", "/tmp/test.json",
			"-r=false",
			"-d", "postgres://flag:flag@localhost:5432/testdb",
			"-k", "flag-key",
		}

		os.Setenv("ADDRESS", "0.0.0.0:8080")
		os.Setenv("STORE_INTERVAL", "120")
		os.Setenv("FILE_STORAGE_PATH", "/var/lib/metrics.json")
		os.Setenv("RESTORE", "true")
		os.Setenv("DATABASE_DSN", "postgres://env:env@localhost:5432/testdb")
		os.Setenv("KEY", "env-key")

		defer func() {
			os.Unsetenv("ADDRESS")
			os.Unsetenv("STORE_INTERVAL")
			os.Unsetenv("FILE_STORAGE_PATH")
			os.Unsetenv("RESTORE")
			os.Unsetenv("DATABASE_DSN")
			os.Unsetenv("KEY")
		}()

		cfg := Load()

		assert.Equal(t, "0.0.0.0:8080", cfg.Address)
		assert.Equal(t, 120, cfg.StoreInterval)
		assert.Equal(t, "/var/lib/metrics.json", cfg.FileStoragePath)
		assert.True(t, cfg.Restore)
		assert.Equal(t, "postgres://env:env@localhost:5432/testdb", cfg.DatabaseDSN)
		assert.Equal(t, "env-key", cfg.HashKey)
	})

	t.Run("обрабатывает некорректные значения из окружения", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		os.Setenv("STORE_INTERVAL", "not-a-number")
		os.Setenv("RESTORE", "not-a-bool")

		defer func() {
			os.Unsetenv("STORE_INTERVAL")
			os.Unsetenv("RESTORE")
		}()

		cfg := Load()

		// Должны остаться значения по умолчанию
		assert.Equal(t, 300, cfg.StoreInterval)
		assert.True(t, cfg.Restore)
	})
}

func TestApplyEnv(t *testing.T) {
	for _, key := range []string{
		"ADDRESS",
		"STORE_INTERVAL",
		"FILE_STORAGE_PATH",
		"RESTORE",
		"DATABASE_DSN",
		"KEY",
	} {
		os.Unsetenv(key)
	}
	tests := []struct {
		name        string
		envVars     map[string]string
		initialCfg  *Config
		expectedCfg *Config
		wantErr     bool
	}{
		{
			name: "устанавливает все переменные окружения",
			envVars: map[string]string{
				"ADDRESS":           "0.0.0.0:9090",
				"STORE_INTERVAL":    "60",
				"FILE_STORAGE_PATH": "/custom/path.json",
				"RESTORE":           "false",
				"DATABASE_DSN":      "postgres://env:env@localhost:5432/db",
				"KEY":               "env-secret",
			},
			initialCfg: &Config{
				Address:         "localhost:8080",
				StoreInterval:   300,
				FileStoragePath: "/default/path.json",
				Restore:         true,
				DatabaseDSN:     "",
				HashKey:         "",
			},
			expectedCfg: &Config{
				Address:         "0.0.0.0:9090",
				StoreInterval:   60,
				FileStoragePath: "/custom/path.json",
				Restore:         false,
				DatabaseDSN:     "postgres://env:env@localhost:5432/db",
				HashKey:         "env-secret",
			},
			wantErr: false,
		},
		{
			name: "игнорирует некорректные числовые значения",
			envVars: map[string]string{
				"STORE_INTERVAL": "invalid",
			},
			initialCfg: &Config{
				StoreInterval: 300,
			},
			expectedCfg: &Config{
				StoreInterval: 300, // должно остаться прежним
			},
			wantErr: true,
		},
		{
			name: "игнорирует некорректные булевы значения",
			envVars: map[string]string{
				"RESTORE": "invalid",
			},
			initialCfg: &Config{
				Restore: true,
			},
			expectedCfg: &Config{
				Restore: true, // должно остаться прежним
			},
			wantErr: true,
		},
		{
			name: "частичное обновление",
			envVars: map[string]string{
				"ADDRESS": "0.0.0.0:9090",
				"KEY":     "secret",
			},
			initialCfg: &Config{
				Address:         "localhost:8080",
				StoreInterval:   300,
				FileStoragePath: "/default/path.json",
				Restore:         true,
				DatabaseDSN:     "",
				HashKey:         "",
			},
			expectedCfg: &Config{
				Address:         "0.0.0.0:9090",
				StoreInterval:   300,
				FileStoragePath: "/default/path.json",
				Restore:         true,
				DatabaseDSN:     "",
				HashKey:         "secret",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Устанавливаем переменные окружения
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			cfg := *tt.initialCfg
			err := cleanenv.ReadEnv(&cfg)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedCfg.Address, cfg.Address)
			assert.Equal(t, tt.expectedCfg.StoreInterval, cfg.StoreInterval)
			assert.Equal(t, tt.expectedCfg.FileStoragePath, cfg.FileStoragePath)
			assert.Equal(t, tt.expectedCfg.Restore, cfg.Restore)
			assert.Equal(t, tt.expectedCfg.DatabaseDSN, cfg.DatabaseDSN)
			assert.Equal(t, tt.expectedCfg.HashKey, cfg.HashKey)
		})
	}
}

func TestGetStoreIntervalDuration(t *testing.T) {
	tests := []struct {
		name     string
		interval int
		expected time.Duration
	}{
		{
			name:     "положительное значение",
			interval: 300,
			expected: 300 * time.Second,
		},
		{
			name:     "нулевое значение",
			interval: 0,
			expected: 0 * time.Second,
		},
		{
			name:     "отрицательное значение",
			interval: -100,
			expected: -100 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{StoreInterval: tt.interval}
			result := cfg.GetStoreIntervalDuration()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigInitialization(t *testing.T) {
	t.Run("путь к файлу по умолчанию", func(t *testing.T) {
		// Временно меняем TempDir для теста
		originalTempDir := os.TempDir()

		resetFlags()
		os.Args = []string{"test"}

		cfg := Load()

		expectedPath := filepath.Join(originalTempDir, "metrics.json")
		assert.Equal(t, expectedPath, cfg.FileStoragePath)
	})

	t.Run("DatabaseDSN по умолчанию пустая строка", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		cfg := Load()

		assert.Equal(t, "", cfg.DatabaseDSN)
	})
}

func TestFlagParsing(t *testing.T) {
	t.Run("обрабатывает булев флаг с разными форматами", func(t *testing.T) {
		testCases := []struct {
			flagValue string
			expected  bool
		}{
			{"true", true},
			{"false", false},
			{"1", true},
			{"0", false},
			{"t", true},
			{"f", false},
		}

		for _, tc := range testCases {
			t.Run(tc.flagValue, func(t *testing.T) {
				resetFlags()
				os.Args = []string{"test", "-r=" + tc.flagValue}

				cfg := Load()
				assert.Equal(t, tc.expected, cfg.Restore)
			})
		}
	})
}
func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}
