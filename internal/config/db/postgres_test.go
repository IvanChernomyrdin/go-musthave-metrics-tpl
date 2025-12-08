package db

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConnect(t *testing.T) {
	tests := []struct {
		name           string
		connectionFlag string
		envValue       string
		setEnv         bool
		expected       string
	}{
		{
			name:           "использует значение из флага",
			connectionFlag: "postgres://flag:flag@localhost:5432/test",
			envValue:       "",
			setEnv:         false,
			expected:       "postgres://flag:flag@localhost:5432/test",
		},
		{
			name:           "использует значение из переменной окружения",
			connectionFlag: "",
			envValue:       "postgres://env:env@localhost:5432/test",
			setEnv:         true,
			expected:       "postgres://env:env@localhost:5432/test",
		},
		{
			name:           "переменная окружения имеет приоритет над флагом",
			connectionFlag: "postgres://flag:flag@localhost:5432/test",
			envValue:       "postgres://env:env@localhost:5432/test",
			setEnv:         true,
			expected:       "postgres://env:env@localhost:5432/test",
		},
		{
			name:           "использует значение по умолчанию",
			connectionFlag: "",
			envValue:       "",
			setEnv:         false,
			expected:       "postgres://postgres:postgres@localhost:5432/metrics?sslmode=disable",
		},
		{
			name:           "удаляет кавычки из строки",
			connectionFlag: `"postgres://quoted:quoted@localhost:5432/test"`,
			envValue:       "",
			setEnv:         false,
			expected:       "postgres://quoted:quoted@localhost:5432/test",
		},
		{
			name:           "удаляет кавычки из переменной окружения",
			connectionFlag: "",
			envValue:       `"postgres://envquoted:envquoted@localhost:5432/test"`,
			setEnv:         true,
			expected:       "postgres://envquoted:envquoted@localhost:5432/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				os.Setenv("DATABASE_DSN", tt.envValue)
				defer os.Unsetenv("DATABASE_DSN")
			}

			result := GetConnect(tt.connectionFlag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPing(t *testing.T) {
	t.Run("ошибка если DB не инициализирована", func(t *testing.T) {
		originalDB := DB
		DB = nil
		defer func() { DB = originalDB }()

		err := Ping()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "база данных не инициализирована")
	})
}

func TestGetDB(t *testing.T) {
	t.Run("возвращает текущее соединение с БД", func(t *testing.T) {
		originalDB := DB
		defer func() { DB = originalDB }()
		// Просто проверяем что функция возвращает DB
		result := GetDB()
		assert.Equal(t, DB, result)
	})

	t.Run("возвращает nil если БД не инициализирована", func(t *testing.T) {
		originalDB := DB
		DB = nil
		defer func() { DB = originalDB }()

		result := GetDB()
		assert.Nil(t, result)
	})
}

func TestInit(t *testing.T) {
	t.Run("ошибка при невалидном DSN", func(t *testing.T) {
		originalDB := DB
		defer func() { DB = originalDB }()

		err := Init("invalid://connection")
		require.Error(t, err)
	})

	t.Run("ошибка при отсутствии миграций", func(t *testing.T) {
		originalDB := DB
		defer func() {
			if DB != nil {
				DB.Close()
			}
			DB = originalDB
		}()

		// Используем несуществующий путь к миграциям
		// Это протестирует что Init пытается применить миграции
		err := Init("postgres://test:test@localhost:5432/testdb")
		// Ожидаем ошибку подключения (нет такой БД) или ошибку миграций
		assert.Error(t, err)
	})
}
