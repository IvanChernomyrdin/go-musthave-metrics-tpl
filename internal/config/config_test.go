// Package config
package config

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.Equal(t, "", cfg.CryptoKey)
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
			"-crypto-key", "/path/to/key.pem",
		}

		cfg := Load()

		assert.Equal(t, "127.0.0.1:9090", cfg.Address)
		assert.Equal(t, 60, cfg.StoreInterval)
		assert.Equal(t, "/tmp/test.json", cfg.FileStoragePath)
		assert.False(t, cfg.Restore)
		assert.Equal(t, "postgres://test:test@localhost:5432/testdb", cfg.DatabaseDSN)
		assert.Equal(t, "secret-key", cfg.HashKey)
		assert.Equal(t, "/path/to/key.pem", cfg.CryptoKey)
	})

	t.Run("загружает значения из JSON файла через флаг -c", func(t *testing.T) {
		// Создаем временный JSON файл
		configJSON := `{
			"address": "json-address:7070",
			"store_interval": 120,
			"store_file": "/json/path.json",
			"restore": false,
			"database_dsn": "postgres://json:json@localhost:5432/jsondb",
			"crypto_key": "/json/key.pem"
		}`

		tmpFile, err := os.CreateTemp("", "config-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte(configJSON))
		require.NoError(t, err)
		tmpFile.Close()

		resetFlags()
		os.Args = []string{"test", "-c", tmpFile.Name()}

		cfg := Load()

		assert.Equal(t, "json-address:7070", cfg.Address)
		assert.Equal(t, 120, cfg.StoreInterval)
		assert.Equal(t, "/json/path.json", cfg.FileStoragePath)
		assert.False(t, cfg.Restore)
		assert.Equal(t, "postgres://json:json@localhost:5432/jsondb", cfg.DatabaseDSN)
		assert.Equal(t, "/json/key.pem", cfg.CryptoKey)
	})

	t.Run("загружает значения из JSON файла через переменную CONFIG", func(t *testing.T) {
		// Создаем временный JSON файл
		configJSON := `{
			"address": "json-env-address:8080",
			"store_interval": 150,
			"store_file": "/json/env/path.json",
			"restore": true,
			"database_dsn": "postgres://env:env@localhost:5432/envdb",
			"crypto_key": "/json/env/key.pem"
		}`

		tmpFile, err := os.CreateTemp("", "config-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte(configJSON))
		require.NoError(t, err)
		tmpFile.Close()

		// Устанавливаем переменную окружения CONFIG
		os.Setenv("CONFIG", tmpFile.Name())
		defer os.Unsetenv("CONFIG")

		resetFlags()
		os.Args = []string{"test"}

		cfg := Load()

		assert.Equal(t, "json-env-address:8080", cfg.Address)
		assert.Equal(t, 150, cfg.StoreInterval)
		assert.Equal(t, "/json/env/path.json", cfg.FileStoragePath)
		assert.True(t, cfg.Restore)
		assert.Equal(t, "postgres://env:env@localhost:5432/envdb", cfg.DatabaseDSN)
		assert.Equal(t, "/json/env/key.pem", cfg.CryptoKey)
	})

	t.Run("флаги имеют приоритет над JSON файлом", func(t *testing.T) {
		// Создаем временный JSON файл
		configJSON := `{
			"address": "json-address:7070",
			"store_interval": 120,
			"store_file": "/json/path.json",
			"restore": false,
			"database_dsn": "postgres://json:json@localhost:5432/jsondb",
			"crypto_key": "/json/key.pem"
		}`

		tmpFile, err := os.CreateTemp("", "config-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte(configJSON))
		require.NoError(t, err)
		tmpFile.Close()

		resetFlags()
		os.Args = []string{"test", "-c", tmpFile.Name(), "-a", "flag-address:9090", "-i", "30"}

		cfg := Load()

		// Флаги должны переопределить JSON
		assert.Equal(t, "flag-address:9090", cfg.Address)                              // из флага
		assert.Equal(t, 30, cfg.StoreInterval)                                         // из флага
		assert.Equal(t, "/json/path.json", cfg.FileStoragePath)                        // из JSON (флаг не задан)
		assert.False(t, cfg.Restore)                                                   // из JSON
		assert.Equal(t, "postgres://json:json@localhost:5432/jsondb", cfg.DatabaseDSN) // из JSON
		assert.Equal(t, "/json/key.pem", cfg.CryptoKey)                                // из JSON
	})

	t.Run("переменные окружения имеют приоритет над JSON файлом", func(t *testing.T) {
		// Создаем временный JSON файл
		configJSON := `{
			"address": "json-address:7070",
			"store_interval": 120,
			"store_file": "/json/path.json",
			"restore": false,
			"database_dsn": "postgres://json:json@localhost:5432/jsondb",
			"crypto_key": "/json/key.pem"
		}`

		tmpFile, err := os.CreateTemp("", "config-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte(configJSON))
		require.NoError(t, err)
		tmpFile.Close()

		// Устанавливаем переменные окружения
		os.Setenv("CONFIG", tmpFile.Name())
		os.Setenv("ADDRESS", "env-address:8080")
		os.Setenv("STORE_INTERVAL", "240")
		defer func() {
			os.Unsetenv("CONFIG")
			os.Unsetenv("ADDRESS")
			os.Unsetenv("STORE_INTERVAL")
		}()

		resetFlags()
		os.Args = []string{"test"}

		cfg := Load()

		// Env должны переопределить JSON
		assert.Equal(t, "env-address:8080", cfg.Address)                               // из env
		assert.Equal(t, 240, cfg.StoreInterval)                                        // из env
		assert.Equal(t, "/json/path.json", cfg.FileStoragePath)                        // из JSON (env не задан)
		assert.False(t, cfg.Restore)                                                   // из JSON
		assert.Equal(t, "postgres://json:json@localhost:5432/jsondb", cfg.DatabaseDSN) // из JSON
		assert.Equal(t, "/json/key.pem", cfg.CryptoKey)                                // из JSON
	})

	t.Run("порядок приоритетов: флаги > env > JSON", func(t *testing.T) {
		// Создаем временный JSON файл
		configJSON := `{
			"address": "json-address:7070",
			"store_interval": 120,
			"store_file": "/json/path.json",
			"restore": false,
			"database_dsn": "postgres://json:json@localhost:5432/jsondb",
			"crypto_key": "/json/key.pem"
		}`

		tmpFile, err := os.CreateTemp("", "config-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte(configJSON))
		require.NoError(t, err)
		tmpFile.Close()

		// Устанавливаем env (должны переопределить JSON)
		os.Setenv("CONFIG", tmpFile.Name())
		os.Setenv("STORE_INTERVAL", "180")
		os.Setenv("RESTORE", "true")
		defer func() {
			os.Unsetenv("CONFIG")
			os.Unsetenv("STORE_INTERVAL")
			os.Unsetenv("RESTORE")
		}()

		// Устанавливаем флаги (должны переопределить всё)
		resetFlags()
		os.Args = []string{"test", "-a", "flag-address:9999", "-f", "/flag/path.json"}

		cfg := Load()

		// Проверяем приоритеты
		assert.Equal(t, "flag-address:9999", cfg.Address)                              // из флага
		assert.Equal(t, 180, cfg.StoreInterval)                                        // из env (флаг не задан)
		assert.Equal(t, "/flag/path.json", cfg.FileStoragePath)                        // из флага
		assert.True(t, cfg.Restore)                                                    // из env
		assert.Equal(t, "postgres://json:json@localhost:5432/jsondb", cfg.DatabaseDSN) // из JSON
		assert.Equal(t, "/json/key.pem", cfg.CryptoKey)                                // из JSON
	})

	t.Run("невалидный JSON файл игнорируется", func(t *testing.T) {
		// Создаем невалидный JSON файл
		tmpFile, err := os.CreateTemp("", "config-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte("{ invalid json }"))
		require.NoError(t, err)
		tmpFile.Close()

		// Через флаг
		resetFlags()
		os.Args = []string{"test", "-c", tmpFile.Name()}

		cfg1 := Load()

		// Должны быть значения по умолчанию
		assert.Equal(t, "localhost:8080", cfg1.Address)
		assert.Equal(t, 300, cfg1.StoreInterval)

		// Через переменную окружения
		os.Setenv("CONFIG", tmpFile.Name())
		defer os.Unsetenv("CONFIG")

		resetFlags()
		os.Args = []string{"test"}

		cfg2 := Load()

		// Должны быть значения по умолчанию
		assert.Equal(t, "localhost:8080", cfg2.Address)
		assert.Equal(t, 300, cfg2.StoreInterval)
	})

	t.Run("JSON файл с частичными настройками", func(t *testing.T) {
		configJSON := `{
			"address": "partial-address:8080",
			"crypto_key": "/partial/key.pem"
		}`

		tmpFile, err := os.CreateTemp("", "config-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte(configJSON))
		require.NoError(t, err)
		tmpFile.Close()

		resetFlags()
		os.Args = []string{"test", "-c", tmpFile.Name()}

		cfg := Load()

		assert.Equal(t, "partial-address:8080", cfg.Address)
		assert.Equal(t, 300, cfg.StoreInterval)                                           // значение по умолчанию
		assert.Equal(t, filepath.Join(os.TempDir(), "metrics.json"), cfg.FileStoragePath) // значение по умолчанию
		assert.True(t, cfg.Restore)                                                       // значение по умолчанию
		assert.Equal(t, "/partial/key.pem", cfg.CryptoKey)
	})
}

func TestApplyEnv(t *testing.T) {
	// Очищаем переменные окружения перед тестом
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
				CryptoKey:       "",
			},
			expectedCfg: &Config{
				Address:         "0.0.0.0:9090",
				StoreInterval:   60,
				FileStoragePath: "/custom/path.json",
				Restore:         false,
				DatabaseDSN:     "postgres://env:env@localhost:5432/db",
				HashKey:         "env-secret",
				CryptoKey:       "",
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
				CryptoKey:       "",
			},
			expectedCfg: &Config{
				Address:         "0.0.0.0:9090",
				StoreInterval:   300,
				FileStoragePath: "/default/path.json",
				Restore:         true,
				DatabaseDSN:     "",
				HashKey:         "secret",
				CryptoKey:       "",
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
			assert.Equal(t, tt.expectedCfg.CryptoKey, cfg.CryptoKey)
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
		resetFlags()
		os.Args = []string{"test"}

		cfg := Load()

		expectedPath := filepath.Join(os.TempDir(), "metrics.json")
		assert.Equal(t, expectedPath, cfg.FileStoragePath)
	})

	t.Run("DatabaseDSN по умолчанию пустая строка", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		cfg := Load()

		assert.Equal(t, "", cfg.DatabaseDSN)
	})

	t.Run("CryptoKey по умолчанию пустая строка", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		cfg := Load()

		assert.Equal(t, "", cfg.CryptoKey)
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

func TestJSONConfigStructure(t *testing.T) {
	// Тест на правильность структуры JSON
	configJSON := `{
		"address": "test:8080",
		"store_interval": 60,
		"store_file": "/test/path.json",
		"restore": false,
		"database_dsn": "test-dsn",
		"crypto_key": "/test/key.pem"
	}`

	var fileCfg struct {
		Address       string `json:"address"`
		StoreInterval int    `json:"store_interval"`
		StoreFile     string `json:"store_file"`
		Restore       bool   `json:"restore"`
		DatabaseDSN   string `json:"database_dsn"`
		CryptoKey     string `json:"crypto_key"`
	}

	err := json.Unmarshal([]byte(configJSON), &fileCfg)
	require.NoError(t, err)

	assert.Equal(t, "test:8080", fileCfg.Address)
	assert.Equal(t, 60, fileCfg.StoreInterval)
	assert.Equal(t, "/test/path.json", fileCfg.StoreFile)
	assert.False(t, fileCfg.Restore)
	assert.Equal(t, "test-dsn", fileCfg.DatabaseDSN)
	assert.Equal(t, "/test/key.pem", fileCfg.CryptoKey)
}

func resetFlags() {
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
}
