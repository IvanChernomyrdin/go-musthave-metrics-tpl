package model

type MetricsBatch struct {
	Item []Metrics
}

func (mb *MetricsBatch) Reset() {
	if mb == nil {
		return
	}
	mb.Item = mb.Item[:0]
}
