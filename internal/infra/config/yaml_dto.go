package config

type YAMLCollection struct {
	Name     string            `yaml:"name"`
	Vars     map[string]string `yaml:"vars"`
	Requests []YAMLRequest     `yaml:"requests"`
}

type YAMLRequest struct {
	Name    string            `yaml:"name"`
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`

	JSON        map[string]any    `yaml:"json"`
	Form        map[string]string `yaml:"form"`
	Raw         string            `yaml:"raw"`
	ContentType string            `yaml:"content_type"`

	Assert  YAMLAssertions    `yaml:"assert"`
	Extract map[string]string `yaml:"extract"`
}

type YAMLAssertions struct {
	Status *int `yaml:"status"`
	MaxMS  *int `yaml:"max_ms"`

	JSONPath map[string]YAMLJSONPathAssertion `yaml:"jsonpath"`
}

type YAMLJSONPathAssertion struct {
	Exists bool `yaml:"exists"`
}

type YAMLEnvironment struct {
	Vars map[string]string `yaml:"vars"`
}
