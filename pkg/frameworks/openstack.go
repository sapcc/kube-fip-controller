/*******************************************************************************
*
* Copyright 2022 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package frameworks

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/tokens"
	neutronfip "github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/subnets"
	"github.com/pkg/errors"

	"github.com/sapcc/kube-fip-controller/pkg/config"
	"github.com/sapcc/kube-fip-controller/pkg/metrics"
)

const (
	statusActive                 = "ACTIVE"
	createFIPDescription         = "Floating IP allocated by kube-fip-controller"
	createFIPDescriptionNodepool = "Floating IP allocated by kube-fip-controller nodepool=%s"
)

var allProjectsHeader = map[string]string{"X-Auth-All-Projects": "true"}

// OSFramework is the OpenStack Framework.
type OSFramework struct {
	computeClient,
	neutronClient *gophercloud.ServiceClient
	logger log.Logger
	opts   config.Options
	context.Context
}

// NewOSFramework returns a new OSFramework.
func NewOSFramework(opts config.Options, logger log.Logger) (*OSFramework, error) {
	provider, err := newAuthenticatedProviderClient(opts.Auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed to authenticate")
	}

	endpointOpts := gophercloud.EndpointOpts{}
	cClient, err := openstack.NewComputeV2(provider, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create compute v2 client")
	}

	nClient, err := openstack.NewNetworkV2(provider, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create network v2 client")
	}

	return &OSFramework{
		computeClient: cClient,
		neutronClient: nClient,
		logger:        log.With(logger, "component", "osFramework"),
		opts:          opts,
		Context:       context.Background(),
	}, nil
}

func newAuthenticatedProviderClient(auth *config.Auth) (*gophercloud.ProviderClient, error) {
	opts := &tokens.AuthOptions{
		IdentityEndpoint: auth.AuthURL,
		Username:         auth.Username,
		Password:         auth.Password,
		DomainName:       auth.UserDomainName,
		AllowReauth:      true,
		Scope: tokens.Scope{
			ProjectName: auth.ProjectName,
			DomainName:  auth.ProjectDomainName,
		},
	}

	provider, err := openstack.NewClient(auth.AuthURL)
	if err != nil {
		return nil, err
	}

	err = openstack.AuthenticateV3(context.Background(), provider, opts, gophercloud.EndpointOpts{})
	return provider, err
}

// GetServerByName returns an openstack server found by name or an error.
func (o *OSFramework) GetServerByName(name string) (*servers.Server, error) {
	listOpts := servers.ListOpts{
		Name:       name,
		AllTenants: true,
	}

	allPages, err := servers.List(o.computeClient, listOpts).AllPages(o.Context)
	if err != nil {
		return nil, err
	}

	allServers, err := servers.ExtractServers(allPages)
	if err != nil {
		return nil, err
	}

	for _, s := range allServers {
		if s.Name == name {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("no server with name %s found", name)
}

// GetServerByID returns the server or an error.
func (o *OSFramework) GetServerByID(id string) (*servers.Server, error) {
	return servers.Get(o.Context, o.computeClient, id).Extract()
}

// GetNetworkIDByName returns the id of the network found by name or an error.
func (o *OSFramework) GetNetworkIDByName(name string) (string, error) {
	url := o.neutronClient.ServiceURL("networks")
	listOpts := networks.ListOpts{
		Name:   name,
		Status: statusActive,
	}

	listOptsStr, err := listOpts.ToNetworkListQuery()
	if err != nil {
		return "", err
	}
	url += listOptsStr

	var (
		res     gophercloud.Result
		resData struct {
			Networks []networks.Network `json:"networks"`
		}
	)

	opts := gophercloud.RequestOpts{
		MoreHeaders: allProjectsHeader,
	}

	_, res.Err = o.neutronClient.Get(o.Context, url, &res.Body, &opts)
	if err := res.ExtractInto(&resData); err != nil {
		return "", err
	}

	for _, net := range resData.Networks {
		if net.Name == name {
			return net.ID, nil
		}
	}

	return "", fmt.Errorf("no network with name %s found", name)
}

// GetSubnetIDByName returns the subnet's id for the given name or an error.
func (o *OSFramework) GetSubnetIDByName(name string) (string, error) {
	listOpts := subnets.ListOpts{
		Name: name,
	}

	allPages, err := subnets.List(o.neutronClient, listOpts).AllPages(o.Context)
	if err != nil {
		return "", err
	}

	allSubnets, err := subnets.ExtractSubnets(allPages)
	if err != nil {
		return "", err
	}

	for _, sub := range allSubnets {
		if sub.Name == name {
			return sub.ID, nil
		}
	}

	return "", fmt.Errorf("no subnet with name %s found", name)
}

// GetOrCreateFloatingIP gets and existing or create a new neutron floating IP and returns it or an error.
func (o *OSFramework) GetOrCreateFloatingIP(floatingIP, floatingNetworkID, subnetID, projectID, nodepool string, reuse bool) (*neutronfip.FloatingIP, error) {
	fip, err := o.getFloatingIP(floatingIP, projectID, nodepool, reuse)
	if err == nil {
		return fip, nil
	}

	if IsFIPNotFound(err) {
		return o.createFloatingIP(floatingIP, floatingNetworkID, subnetID, projectID, nodepool)
	}

	return nil, err
}

// EnsureAssociatedInstanceAndFIP ensures the given floating IP is associated with the given server.
func (o *OSFramework) EnsureAssociatedInstanceAndFIP(server *servers.Server, fip *neutronfip.FloatingIP) error {
	// Get the floating IPs port.
	port, err := o.getPortByID(fip.PortID)
	if err != nil {
		return err
	}

	switch port.DeviceID {
	case "":
		return o.associateInstanceAndFIP(server, fip.FloatingIP)
	case server.ID:
		// If the port belongs to the server, we can assume the FIP is already associated with the server and return here.
		//nolint:errcheck
		_ = level.Info(o.logger).Log("msg", "FIP already attached to instance", "fip", fip.FloatingIP, "serverID", server.ID)
		return nil
	default:
		return fmt.Errorf("FIP already associated with another server %s", server.Name)
	}
}

func (o *OSFramework) associateInstanceAndFIP(server *servers.Server, floatingIP string) error {
	opts := neutronfip.UpdateOpts{
		FixedIP: floatingIP,
	}
	//nolint:errcheck
	_ = level.Info(o.logger).Log("msg", "attaching FIP to instance", "fip", floatingIP, "serverID", server.ID)
	_, err := neutronfip.Update(o.Context, o.neutronClient, server.ID, opts).Extract()
	if err != nil {
		//nolint:errcheck
		_ = level.Error(o.logger).Log("msg", "error attaching FIP to instance", "fip", floatingIP, "serverID", server.ID, "err", err)
		metrics.MetricErrorAssociateInstanceAndFIP.Inc()
		return err
	}
	return nil
}

func (o *OSFramework) getPortByID(id string) (*ports.Port, error) {
	return ports.Get(o.Context, o.neutronClient, id).Extract()
}

func (o *OSFramework) createFloatingIP(floatingIP, floatingNetworkID, subnetID, projectID, nodepool string) (*neutronfip.FloatingIP, error) {
	description := createFIPDescription
	if nodepool != "" {
		description = fmt.Sprintf(createFIPDescriptionNodepool, nodepool)
	}

	createOpts := neutronfip.CreateOpts{
		FloatingNetworkID: floatingNetworkID,
		SubnetID:          subnetID,
		FloatingIP:        floatingIP,
		ProjectID:         projectID,
		Description:       description,
	}
	fip, err := neutronfip.Create(o.Context, o.neutronClient, createOpts).Extract()
	if err != nil {
		//nolint:errcheck
		_ = level.Error(o.logger).Log("msg", "error creating floating ip", "floatingIP", floatingIP, "err", err)
		metrics.MetricErrorCreateFIP.Inc()
		return nil, err
	}
	//nolint:errcheck
	_ = level.Info(o.logger).Log("msg", "created floating ip", "floatingIP", fip.FloatingIP, "id", fip.ID)
	return fip, nil
}

func (o *OSFramework) getFloatingIP(floatingIP, projectID, nodepool string, reuse bool) (*neutronfip.FloatingIP, error) {
	listOpts := neutronfip.ListOpts{
		FloatingIP: floatingIP,
		ProjectID:  projectID,
	}
	if reuse && floatingIP == "" && nodepool != "" {
		listOpts.Description = fmt.Sprintf(createFIPDescriptionNodepool, nodepool)
	}
	allPages, err := neutronfip.List(o.neutronClient, listOpts).AllPages(o.Context)
	if err != nil {
		return nil, err
	}

	allFIPs, err := neutronfip.ExtractFloatingIPs(allPages)
	if err != nil {
		return nil, err
	}

	for _, fip := range allFIPs {
		if fip.FloatingIP == floatingIP {
			return &fip, nil
		}
		if reuse && floatingIP == "" && nodepool != "" && fip.FixedIP == "" {
			return &fip, nil
		}
	}

	return nil, ErrFIPNotFound
}
