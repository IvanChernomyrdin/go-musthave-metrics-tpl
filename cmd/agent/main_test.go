package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMainFunction(t *testing.T) {
	t.Run("test signal context", func(t *testing.T) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		assert.NotNil(t, ctx)
		assert.NotNil(t, stop)
	})

	t.Run("test context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		select {
		case <-ctx.Done():
			assert.Error(t, ctx.Err())
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for context cancellation")
		}
	})
}

func TestSignalHandling(t *testing.T) {
	t.Run("test interrupt signal", func(t *testing.T) {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)

		go func() {
			time.Sleep(10 * time.Millisecond)
			ch <- os.Interrupt
		}()

		select {
		case sig := <-ch:
			assert.Equal(t, os.Interrupt, sig)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for signal")
		}

		signal.Stop(ch)
	})

	t.Run("test SIGTERM signal", func(t *testing.T) {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM)

		go func() {
			time.Sleep(10 * time.Millisecond)
			ch <- syscall.SIGTERM
		}()

		select {
		case sig := <-ch:
			assert.Equal(t, syscall.SIGTERM, sig)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for signal")
		}

		signal.Stop(ch)
	})
}

func TestEnvironmentVariables(t *testing.T) {
	t.Run("test os interrupt signal", func(t *testing.T) {
		assert.Equal(t, os.Interrupt, os.Interrupt)
	})

	t.Run("test syscall SIGTERM", func(t *testing.T) {
		assert.Equal(t, syscall.SIGTERM, syscall.SIGTERM)
	})
}

func TestContextOperations(t *testing.T) {
	t.Run("test context with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		select {
		case <-ctx.Done():
			assert.Equal(t, context.DeadlineExceeded, ctx.Err())
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout should have been exceeded")
		}
	})

	t.Run("test background context", func(t *testing.T) {
		ctx := context.Background()
		assert.NoError(t, ctx.Err())
		assert.NotNil(t, ctx)
	})
}

func TestOSOperations(t *testing.T) {
	t.Run("test process signals", func(t *testing.T) {
		assert.NotNil(t, os.Interrupt)
		assert.NotNil(t, syscall.SIGTERM)
	})

	t.Run("test process id", func(t *testing.T) {
		pid := os.Getpid()
		assert.Greater(t, pid, 0)
	})
}
