/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package loadbalancers

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/network/mgmt/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
)

// Reconcile gets/creates/updates a load balancer.
func (s *Service) Reconcile(ctx context.Context) error {
	for _, lbSpec := range s.Scope.LBSpecs() {
		frontEndIPConfigName := fmt.Sprintf("%s-%s", lbSpec.Name, "frontEnd")
		backEndAddressPoolName := fmt.Sprintf("%s-%s", lbSpec.Name, "backendPool")
		if lbSpec.Role == infrav1.NodeOutboundRole {
			backEndAddressPoolName = fmt.Sprintf("%s-%s", lbSpec.Name, "outboundBackendPool")
		}
		idPrefix := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers", s.Scope.SubscriptionID(), s.Scope.ResourceGroup())

		s.Scope.V(2).Info("creating load balancer", "load balancer", lbSpec.Name)

		var frontIPConfig network.FrontendIPConfigurationPropertiesFormat
		if lbSpec.Role == infrav1.InternalRole {
			var privateIP string
			internalLB, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), lbSpec.Name)
			if err == nil {
				ipConfigs := internalLB.LoadBalancerPropertiesFormat.FrontendIPConfigurations
				if ipConfigs != nil && len(*ipConfigs) > 0 {
					privateIP = to.String((*ipConfigs)[0].FrontendIPConfigurationPropertiesFormat.PrivateIPAddress)
				}
			} else if azure.ResourceNotFound(err) {
				s.Scope.V(2).Info("internalLB not found in RG", "internal lb", lbSpec.Name, "resource group", s.Scope.ResourceGroup())
				privateIP = "10.0.0.100"
				/*
					privateIP, err = s.getAvailablePrivateIP(ctx, s.Scope.Vnet().ResourceGroup, s.Scope.Vnet().Name, lbSpec.SubnetCidr, lbSpec.PrivateIPAddress)
					if err != nil {
						return err
					}
				*/
				s.Scope.V(2).Info("setting internal load balancer IP", "private ip", privateIP)
			} else {
				return errors.Wrap(err, "failed to look for existing internal LB")
			}
			s.Scope.V(2).Info("getting subnet", "subnet", lbSpec.SubnetName)
			subnet, err := s.SubnetsClient.Get(ctx, s.Scope.Vnet().ResourceGroup, s.Scope.Vnet().Name, lbSpec.SubnetName)
			if err != nil {
				return errors.Wrap(err, "failed to get subnet")
			}
			s.Scope.V(2).Info("successfully got subnet", "subnet", lbSpec.SubnetName)
			frontIPConfig = network.FrontendIPConfigurationPropertiesFormat{
				PrivateIPAllocationMethod: network.Static,
				Subnet:                    &subnet,
				PrivateIPAddress:          to.StringPtr(privateIP),
			}
		} else {
			s.Scope.V(2).Info("getting public ip", "public ip", lbSpec.PublicIPName)
			publicIP, err := s.PublicIPsClient.Get(ctx, s.Scope.ResourceGroup(), lbSpec.PublicIPName)
			if err != nil && azure.ResourceNotFound(err) {
				return errors.Wrap(err, fmt.Sprintf("public ip %s not found in RG %s", lbSpec.PublicIPName, s.Scope.ResourceGroup()))
			} else if err != nil {
				return errors.Wrap(err, "failed to look for existing public IP")
			}
			s.Scope.V(2).Info("successfully got public ip", "public ip", lbSpec.PublicIPName)
			frontIPConfig = network.FrontendIPConfigurationPropertiesFormat{
				PrivateIPAllocationMethod: network.Dynamic,
				PublicIPAddress:           &publicIP,
			}
		}

		lb := network.LoadBalancer{
			Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameBasic},
			Location: to.StringPtr(s.Scope.Location()),
			Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
				ClusterName: s.Scope.ClusterName(),
				Lifecycle:   infrav1.ResourceLifecycleOwned,
				Role:        to.StringPtr(lbSpec.Role),
				Additional:  s.Scope.AdditionalTags(),
			})),
			LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
				FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
					{
						Name:                                    &frontEndIPConfigName,
						FrontendIPConfigurationPropertiesFormat: &frontIPConfig,
					},
				},
				BackendAddressPools: &[]network.BackendAddressPool{
					{
						Name: &backEndAddressPoolName,
					},
				},
				OutboundNatRules: &[]network.OutboundNatRule{
					{
						Name: to.StringPtr("OutboundNATAllProtocols"),
						OutboundNatRulePropertiesFormat: &network.OutboundNatRulePropertiesFormat{
							FrontendIPConfigurations: &[]network.SubResource{
								{
									ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbSpec.Name, frontEndIPConfigName)),
								},
							},
							BackendAddressPool: &network.SubResource{
								ID: to.StringPtr(fmt.Sprintf("/%s/%s/backendAddressPools/%s", idPrefix, lbSpec.Name, backEndAddressPoolName)),
							},
						},
					},
				},
			},
		}

		if lbSpec.Role == infrav1.APIServerRole || lbSpec.Role == infrav1.InternalRole {
			probeName := "tcpHTTPSProbe"
			lb.LoadBalancerPropertiesFormat.Probes = &[]network.Probe{
				{
					Name: to.StringPtr(probeName),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolTCP,
						Port:              to.Int32Ptr(lbSpec.APIServerPort),
						IntervalInSeconds: to.Int32Ptr(15),
						NumberOfProbes:    to.Int32Ptr(4),
					},
				},
			}
			lbRule := network.LoadBalancingRule{
				Name: to.StringPtr("LBRuleHTTPS"),
				LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
					Protocol:             network.TransportProtocolTCP,
					FrontendPort:         to.Int32Ptr(lbSpec.APIServerPort),
					BackendPort:          to.Int32Ptr(lbSpec.APIServerPort),
					IdleTimeoutInMinutes: to.Int32Ptr(4),
					EnableFloatingIP:     to.BoolPtr(false),
					LoadDistribution:     "Default",
					FrontendIPConfiguration: &network.SubResource{
						ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbSpec.Name, frontEndIPConfigName)),
					},
					BackendAddressPool: &network.SubResource{
						ID: to.StringPtr(fmt.Sprintf("/%s/%s/backendAddressPools/%s", idPrefix, lbSpec.Name, backEndAddressPoolName)),
					},
					Probe: &network.SubResource{
						ID: to.StringPtr(fmt.Sprintf("/%s/%s/probes/%s", idPrefix, lbSpec.Name, probeName)),
					},
				},
			}

			if lbSpec.Role == infrav1.APIServerRole {
				// We disable outbound SNAT explicitly in the HTTPS LB rule and enable TCP and UDP outbound NAT with an outbound rule.
				// For more information on Standard LB outbound connections see https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections.
				lbRule.LoadBalancingRulePropertiesFormat.DisableOutboundSnat = to.BoolPtr(true)
			} else if lbSpec.Role == infrav1.InternalRole {
				lb.LoadBalancerPropertiesFormat.OutboundNatRules = nil
			}
			lb.LoadBalancerPropertiesFormat.LoadBalancingRules = &[]network.LoadBalancingRule{lbRule}
		}

		err := s.Client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), lbSpec.Name, lb)

		if err != nil {
			return errors.Wrapf(err, "failed to create load balancer %s", lbSpec.Name)
		}

		s.Scope.V(2).Info("successfully created load balancer", "load balancer", lbSpec.Name)
	}
	return nil
}

// Delete deletes the public load balancer with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	for _, lbSpec := range s.Scope.LBSpecs() {
		klog.V(2).Infof("deleting load balancer %s", lbSpec.Name)
		err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), lbSpec.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete load balancer %s in resource group %s", lbSpec.Name, s.Scope.ResourceGroup())
		}

		klog.V(2).Infof("deleted public load balancer %s", lbSpec.Name)
	}
	return nil
}

// getAvailablePrivateIP checks if the desired private IP address is available in a virtual network.
// If the IP address is taken or empty, it will make an attempt to find an available IP in the same subnet
func (s *Service) getAvailablePrivateIP(ctx context.Context, resourceGroup, vnetName, subnetCIDR, PreferredIPAddress string) (string, error) {
	ip := PreferredIPAddress
	if ip == "" {
		ip = azure.DefaultInternalLBIPAddress
		if subnetCIDR != infrav1.DefaultControlPlaneSubnetCIDR {
			// If the user provided a custom subnet CIDR without providing a private IP, try finding an available IP in the subnet space
			index := strings.LastIndex(subnetCIDR, ".")
			ip = subnetCIDR[0:(index+1)] + "0"
		}
	}
	result, err := s.VirtualNetworksClient.CheckIPAddressAvailability(ctx, resourceGroup, vnetName, ip)
	if err != nil {
		return "", errors.Wrap(err, "failed to check IP availability")
	}
	if !to.Bool(result.Available) {
		if len(to.StringSlice(result.AvailableIPAddresses)) == 0 {
			return "", errors.Errorf("IP %s is not available in VNet %s and there were no other available IPs found", ip, vnetName)
		}
		ip = to.StringSlice(result.AvailableIPAddresses)[0]
	}
	return ip, nil
}
