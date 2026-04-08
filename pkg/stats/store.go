// Package stats provides a metrics store with counters, gauges, histograms
// (reservoir sampling), and string sets.
// Source: context/stats.tsx — createStatsStore
package stats

import (
	"math"
	"math/rand"
	"sort"
	"sync"
)

const ReservoirSize = 1024

// Histogram tracks observed values with reservoir sampling (Algorithm R).
type Histogram struct {
	Reservoir []float64
	Count     int
	Sum       float64
	Min       float64
	Max       float64
}

// Store provides counters, gauges, histograms, and string sets.
type Store struct {
	mu         sync.Mutex
	metrics    map[string]float64
	histograms map[string]*Histogram
	sets       map[string]map[string]bool
}

// New creates a new stats store.
func New() *Store {
	return &Store{
		metrics:    make(map[string]float64),
		histograms: make(map[string]*Histogram),
		sets:       make(map[string]map[string]bool),
	}
}

// Increment adds value to a counter (default 1).
func (s *Store) Increment(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics[name] += value
}

// Set overwrites a gauge value.
func (s *Store) Set(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics[name] = value
}

// Observe records a value in a histogram with reservoir sampling.
func (s *Store) Observe(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, ok := s.histograms[name]
	if !ok {
		h = &Histogram{Min: value, Max: value}
		s.histograms[name] = h
	}
	h.Count++
	h.Sum += value
	if value < h.Min {
		h.Min = value
	}
	if value > h.Max {
		h.Max = value
	}

	// Reservoir sampling (Algorithm R)
	if len(h.Reservoir) < ReservoirSize {
		h.Reservoir = append(h.Reservoir, value)
	} else {
		j := rand.Intn(h.Count)
		if j < ReservoirSize {
			h.Reservoir[j] = value
		}
	}
}

// Add adds a string to a named set.
func (s *Store) Add(name string, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sets[name] == nil {
		s.sets[name] = make(map[string]bool)
	}
	s.sets[name][value] = true
}

// GetAll returns all metrics including histogram summaries and set counts.
func (s *Store) GetAll() map[string]float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[string]float64, len(s.metrics))
	for k, v := range s.metrics {
		result[k] = v
	}

	for name, h := range s.histograms {
		sorted := make([]float64, len(h.Reservoir))
		copy(sorted, h.Reservoir)
		sort.Float64s(sorted)

		result[name+"_count"] = float64(h.Count)
		result[name+"_sum"] = h.Sum
		result[name+"_min"] = h.Min
		result[name+"_max"] = h.Max
		if h.Count > 0 {
			result[name+"_avg"] = h.Sum / float64(h.Count)
		}
		if len(sorted) > 0 {
			result[name+"_p50"] = percentile(sorted, 50)
			result[name+"_p95"] = percentile(sorted, 95)
			result[name+"_p99"] = percentile(sorted, 99)
		}
	}

	for name, set := range s.sets {
		result[name+"_unique"] = float64(len(set))
	}

	return result
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := p / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	return sorted[lower] + (sorted[upper]-sorted[lower])*(index-float64(lower))
}
