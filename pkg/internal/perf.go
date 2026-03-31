package internal

import (
	"fmt"
	"os"
	"runtime/pprof"
	"time"
)

// Profiler wraps pprof profiling functionality
type Profiler struct {
	cpuProfile *os.File
	startTime  time.Time
	steps      []StepTiming
}

// StepTiming records timing for a single step
type StepTiming struct {
	Name     string
	Duration time.Duration
}

// NewProfiler creates a new profiler
func NewProfiler(cpuProfilePath string) (*Profiler, error) {
	p := &Profiler{
		steps: make([]StepTiming, 0),
	}

	if cpuProfilePath != "" {
		f, err := os.Create(cpuProfilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create CPU profile: %w", err)
		}
		p.cpuProfile = f
		if err := pprof.StartCPUProfile(f); err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to start CPU profile: %w", err)
		}
	}

	p.startTime = time.Now()
	return p, nil
}

// Stop stops profiling and closes the profile file
func (p *Profiler) Stop() error {
	if p.cpuProfile != nil {
		pprof.StopCPUProfile()
		if err := p.cpuProfile.Close(); err != nil {
			return fmt.Errorf("failed to close CPU profile: %w", err)
		}
		fmt.Printf("CPU profile saved to: %s\n", p.cpuProfile.Name())
		fmt.Printf("To view: go tool pprof %s\n", p.cpuProfile.Name())
	}
	return nil
}

// TimeStep records the duration of a named step
func (p *Profiler) TimeStep(name string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	p.steps = append(p.steps, StepTiming{
		Name:     name,
		Duration: duration,
	})
	return err
}

// PrintTimings prints all step timings
func (p *Profiler) PrintTimings() {
	total := time.Since(p.startTime)
	fmt.Println("\n=== Performance Summary ===")
	fmt.Printf("Total time: %s\n\n", total)
	fmt.Println("Step timings:")
	for _, step := range p.steps {
		percentage := float64(step.Duration) / float64(total) * 100
		fmt.Printf("  %-40s %10s (%5.1f%%)\n", step.Name, step.Duration, percentage)
	}
	fmt.Println()
}
