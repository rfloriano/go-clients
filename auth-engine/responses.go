package auth

import "github.com/vtex/go-clients/common"

type Permissions struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

type Body struct {
	Resource string              `json:"resource"`
	Context  map[string][]string `json:"context"`
	Policies []common.Policy     `json:"policies"`
}
