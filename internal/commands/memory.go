package commands

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

// MemoryMonitor provides memory usage tracking and limits for large repository processing
type MemoryMonitor struct {
	maxMemoryMB      int64
	warningThreshold float64
	checkInterval    time.Duration

	// Atomic counters for concurrent access
	currentFiles   int64
	processedFiles int64
	skippedFiles   int64

	// Memory stats
	lastMemCheck time.Time
	peakMemoryMB int64
}

// MemoryStats represents current memory usage statistics
type MemoryStats struct {
	AllocMB        int64     `json:"alloc_mb"`
	SysMB          int64     `json:"sys_mb"`
	NumGC          uint32    `json:"num_gc"`
	ProcessedFiles int64     `json:"processed_files"`
	SkippedFiles   int64     `json:"skipped_files"`
	PeakMemoryMB   int64     `json:"peak_memory_mb"`
	Timestamp      time.Time `json:"timestamp"`
}

// NewMemoryMonitor creates a new memory monitor with the specified limits
func NewMemoryMonitor(maxMemoryMB int64) *MemoryMonitor {
	if maxMemoryMB <= 0 {
		maxMemoryMB = 512 // Default 512MB limit
	}

	return &MemoryMonitor{
		maxMemoryMB:      maxMemoryMB,
		warningThreshold: 0.8, // Warn at 80% of limit
		checkInterval:    time.Second * 5,
		lastMemCheck:     time.Now(),
	}
}

// CheckMemoryPressure returns true if memory usage is above the warning threshold
func (m *MemoryMonitor) CheckMemoryPressure() (bool, *MemoryStats, error) {
	stats := m.GetMemoryStats()

	// Update peak memory
	if stats.AllocMB > atomic.LoadInt64(&m.peakMemoryMB) {
		atomic.StoreInt64(&m.peakMemoryMB, stats.AllocMB)
	}

	threshold := float64(m.maxMemoryMB) * m.warningThreshold
	isUnderPressure := float64(stats.AllocMB) > threshold

	return isUnderPressure, stats, nil
}

// GetMemoryStats returns current memory usage statistics
func (m *MemoryMonitor) GetMemoryStats() *MemoryStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &MemoryStats{
		AllocMB:        int64(memStats.Alloc / 1024 / 1024),
		SysMB:          int64(memStats.Sys / 1024 / 1024),
		NumGC:          memStats.NumGC,
		ProcessedFiles: atomic.LoadInt64(&m.processedFiles),
		SkippedFiles:   atomic.LoadInt64(&m.skippedFiles),
		PeakMemoryMB:   atomic.LoadInt64(&m.peakMemoryMB),
		Timestamp:      time.Now(),
	}
}

// IncrementProcessedFiles atomically increments the processed files counter
func (m *MemoryMonitor) IncrementProcessedFiles() {
	atomic.AddInt64(&m.processedFiles, 1)
}

// IncrementSkippedFiles atomically increments the skipped files counter
func (m *MemoryMonitor) IncrementSkippedFiles() {
	atomic.AddInt64(&m.skippedFiles, 1)
}

// SetCurrentFiles atomically sets the current files being processed
func (m *MemoryMonitor) SetCurrentFiles(count int64) {
	atomic.StoreInt64(&m.currentFiles, count)
}

// ForceGC triggers garbage collection if memory pressure is high
func (m *MemoryMonitor) ForceGC() {
	runtime.GC()
	runtime.GC() // Double GC to ensure cleanup
}

// StartMemoryWatcher starts a background goroutine to monitor memory usage
func (m *MemoryMonitor) StartMemoryWatcher(ctx context.Context, onPressure func(*MemoryStats)) {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Since(m.lastMemCheck) < m.checkInterval {
				continue
			}

			underPressure, stats, err := m.CheckMemoryPressure()
			if err != nil {
				continue // Skip this check on error
			}

			m.lastMemCheck = time.Now()

			if underPressure && onPressure != nil {
				onPressure(stats)
			}
		}
	}
}

// MemoryConfig holds configuration for memory-optimized processing
type MemoryConfig struct {
	MaxMemoryMB      int64         `json:"max_memory_mb"`
	BatchSize        int           `json:"batch_size"`
	WorkerPoolSize   int           `json:"worker_pool_size"`
	EnableStreaming  bool          `json:"enable_streaming"`
	ForceGCInterval  time.Duration `json:"force_gc_interval"`
	ProgressInterval time.Duration `json:"progress_interval"`
}

// DefaultMemoryConfig returns sensible defaults for memory configuration
func DefaultMemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		MaxMemoryMB:      512,
		BatchSize:        50, // Process 50 files at a time
		WorkerPoolSize:   4,  // 4 concurrent workers
		EnableStreaming:  true,
		ForceGCInterval:  30 * time.Second,
		ProgressInterval: 5 * time.Second,
	}
}

// OptimizeForLargeRepo adjusts config for repositories with >1000 files
func (c *MemoryConfig) OptimizeForLargeRepo(fileCount int) {
	if fileCount > 1000 {
		c.BatchSize = 25                     // Smaller batches
		c.WorkerPoolSize = 2                 // Fewer workers
		c.ForceGCInterval = 15 * time.Second // More frequent GC
		c.MaxMemoryMB = 256                  // Lower memory limit
	}

	if fileCount > 5000 {
		c.BatchSize = 10     // Even smaller batches
		c.WorkerPoolSize = 1 // Single worker
		c.ForceGCInterval = 10 * time.Second
		c.MaxMemoryMB = 128 // Minimal memory limit
	}
}

// LogMemoryStats logs formatted memory statistics
func LogMemoryStats(stats *MemoryStats, verbose bool) {
	if verbose {
		fmt.Printf("Memory: %dMB allocated, %dMB system, %d GCs, Peak: %dMB\n",
			stats.AllocMB, stats.SysMB, stats.NumGC, stats.PeakMemoryMB)
		fmt.Printf("Files: %d processed, %d skipped\n",
			stats.ProcessedFiles, stats.SkippedFiles)
	}
}
