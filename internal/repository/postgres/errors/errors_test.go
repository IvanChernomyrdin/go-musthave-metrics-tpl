package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestPostgresErrorClassifier_Classify(t *testing.T) {
	classifier := NewPostgresErrorClassifier()

	tests := []struct {
		name        string
		err         error
		expected    ErrorClassification
		description string
	}{
		{
			name:        "nil ошибка",
			err:         nil,
			expected:    NonRetriable,
			description: "nil должен быть NonRetriable",
		},
		{
			name:        "обычная ошибка Go",
			err:         errors.New("some error"),
			expected:    NonRetriable,
			description: "не-PostgreSQL ошибки должны быть NonRetriable",
		},
		{
			name: "ошибка класса 08 (connection exception)",
			err: &pgconn.PgError{
				Code: "08000",
			},
			expected:    Retriable,
			description: "ошибки класса 08 должны быть Retriable",
		},
		{
			name: "ошибка класса 08 с подклассом",
			err: &pgconn.PgError{
				Code: "08006",
			},
			expected:    Retriable,
			description: "любые ошибки начинающиеся с 08 должны быть Retriable",
		},
		{
			name: "serialization_failure",
			err: &pgconn.PgError{
				Code: pgerrcode.SerializationFailure,
			},
			expected:    Retriable,
			description: "serialization_failure должна быть Retriable",
		},
		{
			name: "deadlock_detected",
			err: &pgconn.PgError{
				Code: pgerrcode.DeadlockDetected,
			},
			expected:    Retriable,
			description: "deadlock_detected должна быть Retriable",
		},
		{
			name: "admin_shutdown",
			err: &pgconn.PgError{
				Code: pgerrcode.AdminShutdown,
			},
			expected:    Retriable,
			description: "admin_shutdown должна быть Retriable",
		},
		{
			name: "crash_shutdown",
			err: &pgconn.PgError{
				Code: pgerrcode.CrashShutdown,
			},
			expected:    Retriable,
			description: "crash_shutdown должна быть Retriable",
		},
		{
			name: "cannot_connect_now",
			err: &pgconn.PgError{
				Code: pgerrcode.CannotConnectNow,
			},
			expected:    Retriable,
			description: "cannot_connect_now должна быть Retriable",
		},
		{
			name: "другие PostgreSQL ошибки",
			err: &pgconn.PgError{
				Code: "23505",
			},
			expected:    NonRetriable,
			description: "ошибки не из списка retriable должны быть NonRetriable",
		},
		{
			name: "syntax_error",
			err: &pgconn.PgError{
				Code: pgerrcode.SyntaxError,
			},
			expected:    NonRetriable,
			description: "синтаксические ошибки не должны быть retriable",
		},
		{
			name: "foreign_key_violation",
			err: &pgconn.PgError{
				Code: pgerrcode.ForeignKeyViolation,
			},
			expected:    NonRetriable,
			description: "нарушения внешних ключей не должны быть retriable",
		},
		{
			name: "invalid_authorization_specification",
			err: &pgconn.PgError{
				Code: "28000",
			},
			expected:    NonRetriable,
			description: "ошибки авторизации не должны быть retriable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.Classify(tt.err)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestPostgresErrorClassifier_ClassifyPostgresError(t *testing.T) {
	classifier := NewPostgresErrorClassifier()

	t.Run("разные префиксы 08", func(t *testing.T) {
		prefixes := []string{
			"08000",
			"08001",
			"08003",
			"08004",
			"08006",
			"08007",
			"08999",
		}

		for _, code := range prefixes {
			t.Run(code, func(t *testing.T) {
				err := &pgconn.PgError{Code: code}
				result := classifier.Classify(err)
				assert.Equal(t, Retriable, result, "код %s должен быть Retriable", code)
			})
		}
	})

	t.Run("не-PgError ошибки", func(t *testing.T) {
		tests := []struct {
			name     string
			err      error
			expected ErrorClassification
		}{
			{
				name:     "error wrapping",
				err:      errors.New("wrapped: " + (&pgconn.PgError{Code: "08000"}).Error()),
				expected: NonRetriable,
			},
			{
				name:     "custom error type",
				err:      &customError{msg: "custom"},
				expected: NonRetriable,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := classifier.Classify(tt.err)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}

func TestNewPostgresErrorClassifier(t *testing.T) {
	t.Run("создает новый классификатор", func(t *testing.T) {
		classifier := NewPostgresErrorClassifier()
		assert.NotNil(t, classifier)
	})

	t.Run("разные экземпляры работают одинаково", func(t *testing.T) {
		classifier1 := NewPostgresErrorClassifier()
		classifier2 := NewPostgresErrorClassifier()

		err := &pgconn.PgError{Code: "08000"}

		result1 := classifier1.Classify(err)
		result2 := classifier2.Classify(err)

		assert.Equal(t, result1, result2)
		assert.Equal(t, Retriable, result1)
	})
}

func TestErrorClassification_String(t *testing.T) {
	t.Run("значения констант", func(t *testing.T) {
		assert.Equal(t, ErrorClassification(0), NonRetriable)
		assert.Equal(t, ErrorClassification(1), Retriable)
	})
}

type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}
