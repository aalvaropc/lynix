package domain

// Vars is a key/value store used for templating and runtime variable resolution.
type Vars map[string]string

// Environment defines variables for a given runtime context (dev/stg/prod).
// Secrets may be merged on top by infrastructure implementations.
type Environment struct {
	Name string
	Vars Vars
}
