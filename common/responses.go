package common

type Policies struct {
	Policies []Policy `json:"policies"`
}

type Policy struct {
	Source      string      `json:"source"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Deprecated  bool        `json:"deprecated"`
	Statements  []Statement `json:"statements"`
}

type Statement struct {
	Resources  []string              `json:"resources"`
	Actions    []string              `json:"actions"`
	Effect     string                `json:"effect"`
	Conditions map[string]Parameters `json:"conditions"`
}

type Parameters map[string][]string
