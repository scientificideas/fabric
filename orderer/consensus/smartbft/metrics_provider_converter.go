package smartbft

import (
	"github.com/SmartBFT-Go/consensus/pkg/api"
	"github.com/hyperledger/fabric/common/metrics"
)

type MetricProviderConverter struct {
	metricsProvider metrics.Provider
}

func (m *MetricProviderConverter) NewCounter(opts api.CounterOpts) api.Counter {
	o := metrics.CounterOpts{
		Namespace:    opts.Namespace,
		Subsystem:    opts.Subsystem,
		Name:         opts.Name,
		Help:         opts.Help,
		LabelNames:   opts.LabelNames,
		LabelHelp:    opts.LabelHelp,
		StatsdFormat: opts.StatsdFormat,
	}
	// return m.metricsProvider.NewCounter(o)
	return &CounterConverter{
		counter: m.metricsProvider.NewCounter(o),
	}
}

func (m *MetricProviderConverter) NewGauge(opts api.GaugeOpts) api.Gauge {
	o := metrics.GaugeOpts{
		Namespace:    opts.Namespace,
		Subsystem:    opts.Subsystem,
		Name:         opts.Name,
		Help:         opts.Help,
		LabelNames:   opts.LabelNames,
		LabelHelp:    opts.LabelHelp,
		StatsdFormat: opts.StatsdFormat,
	}
	return &GaugeConverter{
		gauge: m.metricsProvider.NewGauge(o),
	}
}

func (m *MetricProviderConverter) NewHistogram(opts api.HistogramOpts) api.Histogram {
	o := metrics.HistogramOpts{
		Namespace:    opts.Namespace,
		Subsystem:    opts.Subsystem,
		Name:         opts.Name,
		Help:         opts.Help,
		LabelNames:   opts.LabelNames,
		LabelHelp:    opts.LabelHelp,
		StatsdFormat: opts.StatsdFormat,
		Buckets:      opts.Buckets,
	}
	return &HistogramConverter{
		histogram: m.metricsProvider.NewHistogram(o),
	}
}

type CounterConverter struct {
	counter metrics.Counter
}

func (c *CounterConverter) With(labelValues ...string) api.Counter {
	c.counter = c.counter.With(labelValues...)
	return c
}

func (c *CounterConverter) Add(delta float64) {
	c.counter.Add(delta)
}

type GaugeConverter struct {
	gauge metrics.Gauge
}

func (g *GaugeConverter) With(labelValues ...string) api.Gauge {
	g.gauge = g.gauge.With(labelValues...)
	return g
}

func (g *GaugeConverter) Add(delta float64) {
	g.gauge.Add(delta)
}

func (g *GaugeConverter) Set(value float64) {
	g.gauge.Set(value)
}

type HistogramConverter struct {
	histogram metrics.Histogram
}

func (h *HistogramConverter) With(labelValues ...string) api.Histogram {
	h.histogram = h.histogram.With(labelValues...)
	return h
}

func (h *HistogramConverter) Observe(value float64) {
	h.histogram.Observe(value)
}
