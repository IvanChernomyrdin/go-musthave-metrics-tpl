// Package agent
package agent

import (
	"testing"
)

func BenchmarkRuntimeMetricsCollectorCollect(b *testing.B) {
	collector := NewRuntimeMetricsCollector()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = collector.Collect()
	}
}

func BenchmarkRuntimeMetricsCollectorCollectSystem(b *testing.B) {
	collect := NewRuntimeMetricsCollector()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = collect.CollectSystemMetrics()
	}
}
