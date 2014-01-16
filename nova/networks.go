// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see COPYING and COPYING.LESSER file for details.

package nova

import (
	"launchpad.net/goose/client"
	"launchpad.net/goose/errors"
	goosehttp "launchpad.net/goose/http"
)

const (
	apiNetworks       = "os-networks"
	apiTenantNetworks = "os-tenant-networks"
)

// Network contains details about a labeled network
type Network struct {
	Id    string `json:"id"`    // UUID of the resource
	Label string `json:"label"` // User-provided name for the network range
	Cidr  string `json:"cidr"`  // IP range covered by the network
}

// ListNetworks gives details on available networks
func (c *Client) ListNetworks() ([]Network, error) {
	var resp struct {
		Networks []Network `json:"networks"`
	}
	requestData := goosehttp.RequestData{RespValue: &resp}
	err := c.client.SendRequest(client.GET, "compute", apiNetworks, &requestData)
	if err != nil {
		return nil, errors.Newf(err, "failed to get list of networks")
	}
	return resp.Networks, nil
}
