// пакет postgres содержит классификатор ошибок PostgreSQL для стратегии повторных попыток.
// помогает различать повторяемые и неповторяемые ошибки при работе с базой данных.
package postgres

import (
	"errors"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// errorClassification определяет тип классификации ошибки.
// используется для принятия решения о возможности повторной попытки.
type ErrorClassification int

const (
	// nonRetriable указывает, что ошибка не является повторяемой.
	// такие ошибки обычно связаны с логическими проблемами (например, нарушение ограничени
	NonRetriable ErrorClassification = iota
	// retriable указывает, что ошибка является повторяемой.
	// такие ошибки обычно временные (например, проблемы с сетью, блокировки).
	Retriable
)

// классифицирует ошибки PostgreSQL для стратегии повторных попыток.
// анализирует коды ошибок PostgreSQL и определяет, можно ли повторить операцию.
type PostgresErrorClassifier struct{}

// создает новый экземпляр классификатора ошибок
func NewPostgresErrorClassifier() *PostgresErrorClassifier {
	return &PostgresErrorClassifier{}
}

// анализирует ошибку и определяет её классификацию
func (c *PostgresErrorClassifier) Classify(err error) ErrorClassification {
	if err == nil {
		return NonRetriable
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return c.classifyPostgresError(pgErr)
	}

	return NonRetriable
}

// классифицирует ошибку PostgreSQL на основе её кода
func (c *PostgresErrorClassifier) classifyPostgresError(pgErr *pgconn.PgError) ErrorClassification {
	if strings.HasPrefix(pgErr.Code, "08") {
		return Retriable
	}

	// Другие повторяемые ошибки PostgreSQL
	switch pgErr.Code {
	case pgerrcode.SerializationFailure,
		pgerrcode.DeadlockDetected,
		pgerrcode.AdminShutdown,
		pgerrcode.CrashShutdown,
		pgerrcode.CannotConnectNow:
		return Retriable
	}

	return NonRetriable
}
