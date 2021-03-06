package housekeeper

import (
	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
)

var housekeeperService = clients.Service{Name: "housekeeper", Major: 0}

func NewHousekeeperGentleman(config *clients.Config) *gentleman.Client {
	return clients.CreateInfraClient(&housekeeperService, config)
}
