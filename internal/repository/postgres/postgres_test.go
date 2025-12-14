// Package postgres
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	errPostgres "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/postgres/errors"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestablePostgresStorage для тестирования
type TestablePostgresStorage struct {
	*PostgresStorage
}

func NewTestableStorage(db *sql.DB) *TestablePostgresStorage {
	return &TestablePostgresStorage{
		PostgresStorage: &PostgresStorage{
			db:              db,
			retryConfig:     DefaultRetryConfig(),
			errorClassifier: errPostgres.NewPostgresErrorClassifier(),
		},
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()
	assert.Equal(t, 3, config.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 5*time.Second, config.MaxDelay)
}

func TestRetryLogic(t *testing.T) {
	// Вместо mock классификатора, тестируем с реальными ошибками PostgreSQL
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	storage := NewTestableStorage(db)
	storage.retryConfig = RetryConfig{MaxAttempts: 3}

	t.Run("успешная операция с первой попытки", func(t *testing.T) {
		callCount := 0
		err := storage.Retry(context.Background(), func() error {
			callCount++
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("неповторяемая ошибка - прекращает попытки", func(t *testing.T) {
		callCount := 0
		nonRetriableErr := errors.New("non-retriable error")

		err := storage.Retry(context.Background(), func() error {
			callCount++
			return nonRetriableErr
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "неповторяемая ошибка")
		assert.Equal(t, 1, callCount)
	})
}
func TestRetryWithPostgresErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	storage := NewTestableStorage(db)
	storage.retryConfig = RetryConfig{MaxAttempts: 2}

	t.Run("retry при временной ошибке PostgreSQL (connection error)", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "08000"}

		callCount := 0
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("test", "gauge", 1.0, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := storage.Retry(context.Background(), func() error {
			callCount++
			if callCount == 1 {
				return pgErr
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})

	t.Run("retry при deadlock", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: pgerrcode.DeadlockDetected}

		callCount := 0
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("test2", "gauge", 2.0, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := storage.Retry(context.Background(), func() error {
			callCount++
			if callCount == 1 {
				return pgErr
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})
}

func TestPostgresStorage_UpsertGauge(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	storage := NewTestableStorage(db)
	storage.retryConfig = RetryConfig{MaxAttempts: 2}

	t.Run("успешное сохранение gauge", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("temperature", "gauge", 25.5, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := storage.UpsertGauge(context.Background(), "temperature", 25.5)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ошибка при сохранении gauge", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("pressure", "gauge", 1013.2, sqlmock.AnyArg()).
			WillReturnError(errors.New("db error"))

		err := storage.UpsertGauge(context.Background(), "pressure", 1013.2)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("retry при временной ошибке PostgreSQL", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "08000"}
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("test", "gauge", 1.0, sqlmock.AnyArg()).
			WillReturnError(pgErr)
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("test", "gauge", 1.0, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := storage.UpsertGauge(context.Background(), "test", 1.0)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStorage_UpsertCounter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	storage := NewTestableStorage(db)
	storage.retryConfig = RetryConfig{MaxAttempts: 2}

	t.Run("успешное сохранение counter", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("requests", "counter", int64(5), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := storage.UpsertCounter(context.Background(), "requests", 5)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ошибка при сохранении counter", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("errors", "counter", int64(1), sqlmock.AnyArg()).
			WillReturnError(errors.New("db error"))

		err := storage.UpsertCounter(context.Background(), "errors", 1)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStorage_GetGauge(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	storage := NewTestableStorage(db)

	t.Run("успешное получение gauge", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"value"}).AddRow(25.5)
		mock.ExpectQuery("SELECT value FROM metrics WHERE mtype = \\$1 AND id = \\$2 AND value IS NOT NULL").
			WithArgs("gauge", "temperature").
			WillReturnRows(rows)

		value, ok := storage.GetGauge(context.Background(), "temperature")
		assert.True(t, ok)
		assert.Equal(t, 25.5, value)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("gauge не найден", func(t *testing.T) {
		mock.ExpectQuery("SELECT value FROM metrics WHERE mtype = \\$1 AND id = \\$2 AND value IS NOT NULL").
			WithArgs("gauge", "nonexistent").
			WillReturnError(sql.ErrNoRows)

		value, ok := storage.GetGauge(context.Background(), "nonexistent")
		assert.False(t, ok)
		assert.Equal(t, 0.0, value)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStorage_GetCounter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	storage := NewTestableStorage(db)

	t.Run("успешное получение counter", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"delta"}).AddRow(int64(100))
		mock.ExpectQuery("SELECT delta FROM metrics WHERE mtype = \\$1 AND id = \\$2 AND delta IS NOT NULL").
			WithArgs("counter", "requests").
			WillReturnRows(rows)

		value, ok := storage.GetCounter(context.Background(), "requests")
		assert.True(t, ok)
		assert.Equal(t, int64(100), value)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("counter не найден", func(t *testing.T) {
		mock.ExpectQuery("SELECT delta FROM metrics WHERE mtype = \\$1 AND id = \\$2 AND delta IS NOT NULL").
			WithArgs("counter", "nonexistent").
			WillReturnError(sql.ErrNoRows)

		value, ok := storage.GetCounter(context.Background(), "nonexistent")
		assert.False(t, ok)
		assert.Equal(t, int64(0), value)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStorage_GetAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	storage := NewTestableStorage(db)

	t.Run("успешное получение всех метрик", func(t *testing.T) {
		gaugeRows := sqlmock.NewRows([]string{"id", "value"}).
			AddRow("temperature", 25.5).
			AddRow("pressure", 1013.2)
		mock.ExpectQuery("SELECT id, value FROM metrics WHERE mtype = 'gauge' AND value IS NOT NULL").
			WillReturnRows(gaugeRows)

		counterRows := sqlmock.NewRows([]string{"id", "delta"}).
			AddRow("requests", int64(100)).
			AddRow("errors", int64(5))
		mock.ExpectQuery("SELECT id, delta FROM metrics WHERE mtype = 'counter' AND delta IS NOT NULL").
			WillReturnRows(counterRows)

		gauges, counters := storage.GetAll(context.Background())

		assert.Len(t, gauges, 2)
		assert.Equal(t, 25.5, gauges["temperature"])
		assert.Equal(t, 1013.2, gauges["pressure"])

		assert.Len(t, counters, 2)
		assert.Equal(t, int64(100), counters["requests"])
		assert.Equal(t, int64(5), counters["errors"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("нет метрик", func(t *testing.T) {
		gaugeRows := sqlmock.NewRows([]string{"id", "value"})
		mock.ExpectQuery("SELECT id, value FROM metrics WHERE mtype = 'gauge' AND value IS NOT NULL").
			WillReturnRows(gaugeRows)

		counterRows := sqlmock.NewRows([]string{"id", "delta"})
		mock.ExpectQuery("SELECT id, delta FROM metrics WHERE mtype = 'counter' AND delta IS NOT NULL").
			WillReturnRows(counterRows)

		gauges, counters := storage.GetAll(context.Background())
		assert.Empty(t, gauges)
		assert.Empty(t, counters)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStorage_UpdateMetricsBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	storage := NewTestableStorage(db)
	storage.retryConfig = RetryConfig{MaxAttempts: 2}

	t.Run("успешное пакетное обновление", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("temperature", "gauge", 25.5, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("requests", "counter", int64(10), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		metrics := []model.Metrics{
			{
				ID:    "temperature",
				MType: model.Gauge,
				Value: func() *float64 { v := 25.5; return &v }(),
			},
			{
				ID:    "requests",
				MType: model.Counter,
				Delta: func() *int64 { v := int64(10); return &v }(),
			},
		}

		err := storage.UpdateMetricsBatch(context.Background(), metrics)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("откат транзакции при ошибке", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("test", "gauge", 1.0, sqlmock.AnyArg()).
			WillReturnError(errors.New("db error"))
		mock.ExpectRollback()

		metrics := []model.Metrics{
			{
				ID:    "test",
				MType: model.Gauge,
				Value: func() *float64 { v := 1.0; return &v }(),
			},
		}

		err := storage.UpdateMetricsBatch(context.Background(), metrics)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("retry при временной ошибке в транзакции", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: pgerrcode.SerializationFailure}

		// Первая попытка
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("test", "gauge", 1.0, sqlmock.AnyArg()).
			WillReturnError(pgErr)
		mock.ExpectRollback()

		// Вторая попытка
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO metrics").
			WithArgs("test", "gauge", 1.0, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		metrics := []model.Metrics{
			{
				ID:    "test",
				MType: model.Gauge,
				Value: func() *float64 { v := 1.0; return &v }(),
			},
		}

		err := storage.UpdateMetricsBatch(context.Background(), metrics)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPostgresStorage_Close(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	mock.ExpectClose()

	storage := NewTestableStorage(db)

	err = storage.Close()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
