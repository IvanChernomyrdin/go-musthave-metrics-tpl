package agent

import (
	"sync"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/pool"
)

type SafeMetrics struct {
	mu   sync.Mutex
	cur  *model.MetricsBatch
	pool *pool.Pool[*model.MetricsBatch]
}

func NewSafeMetrics() *SafeMetrics {
	p := pool.New(func() *model.MetricsBatch {
		return &model.MetricsBatch{
			Item: make([]model.Metrics, 0, 29),
		}
	})

	return &SafeMetrics{
		cur:  p.Get(),
		pool: p,
	}
}

func (sm *SafeMetrics) Append(metrics []model.Metrics) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.cur.Item = append(sm.cur.Item, metrics...)
}

func (sm *SafeMetrics) Len() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return len(sm.cur.Item)
}

// GetAndClear отдаёт текущий батч наружу и берёт новый из пула.
// Наружу возвращаем *MetricsBatch, чтобы потом его можно было Put() обратно в пул.
func (sm *SafeMetrics) GetAndClear() *model.MetricsBatch {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	out := sm.cur
	sm.cur = sm.pool.Get()
	return out
}

func (sm *SafeMetrics) PutBatch(b *model.MetricsBatch) {
	sm.pool.Put(b) // Put вызовет Reset(), и Item станет [:0]
}
