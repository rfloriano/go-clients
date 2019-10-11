package sphinx

import (
	"encoding/json"
	"fmt"
	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
	"strings"
)

const (
	pathToPolicyValidation = "/policies/validate"
)

type Sphinx interface {
	ValidatePolicies(policies json.RawMessage) error
}

type SphinxClient struct {
	http *gentleman.Client
}

type ValidationError struct {
	Policies   []PolicyName        `json:"policies,omitempty"`
	Reason     string              `json:"reason"`
	Attributes map[string][]string `json:"attrs,omitempty"`
	Code       string              `json:"code"`
}

type PolicyName struct {
	Name string `json:"name"`
}

func (v ValidationError) Error() string {
	var sb strings.Builder
	sb.WriteString(v.Reason)

	if len(v.Policies) > 0 {
		sb.WriteString(": ")
		sb.WriteString(fmt.Sprint(v.Policies))
	} else if len(v.Attributes) > 0 {
		sb.WriteString(": ")
		attrs, _ := json.Marshal(v.Attributes)
		sb.Write(attrs)
	}

	return sb.String()
}

func NewSphinxClient(config *clients.Config) Sphinx {
	cl := clients.CreateClient("sphinx", config, true)
	return &SphinxClient{cl}
}

func (cl *SphinxClient) ValidatePolicies(policies json.RawMessage) error {
	res, err := cl.http.Post().AddPath(pathToPolicyValidation).Send()
	if err != nil {
		return err
	}

	if res.Ok {
		return nil
	}

	var v ValidationError

	if err := res.JSON(&v); err != nil {
		return err
	}

	return v
}
