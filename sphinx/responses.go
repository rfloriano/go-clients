package sphinx

import "github.com/vtex/go-clients/common"

type ResourcePolicies struct {
	Routes map[string]RoutesData `json:"routes"`
}

type RoutesData struct {
	Public   bool            `json:"public"`
	Path     string          `json:"path"`
	Policies []common.Policy `json:"policies"`
}
