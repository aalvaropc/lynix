package domain

// Vars is a key/value store used for templating and runtime variable resolution.
type Vars map[string]string

// Environment defines variables for a given runtime context (dev/stg/prod).
// Secrets may be merged on top by infrastructure implementations.
type Environment struct {
	Name string
	Vars Vars
}

// Get returns a value for the given key and a boolean indicating if it exists.
func Get(vars Vars, key string) (string, bool) {
	if vars == nil {
		return "", false
	}
	val, ok := vars[key]
	return val, ok
}

// Set sets a key/value in the map, initializing it if needed.
func Set(vars Vars, key, value string) Vars {
	if vars == nil {
		vars = Vars{}
	}
	vars[key] = value
	return vars
}

// Merge merges base and override vars (override wins) and returns a new map.
func Merge(base Vars, override Vars) Vars {
	out := Vars{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}
