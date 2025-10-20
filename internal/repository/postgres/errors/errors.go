package postgres

import (
	"errors"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

type ErrorClassification int

const (
	NonRetriable ErrorClassification = iota
	Retriable
)

type PostgresErrorClassifier struct{}

func NewPostgresErrorClassifier() *PostgresErrorClassifier {
	return &PostgresErrorClassifier{}
}

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
