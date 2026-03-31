package terraform

// Profiler interface for performance profiling (defined in bootstrap package)
type Profiler interface {
	TimeStep(name string, fn func() error) error
}
