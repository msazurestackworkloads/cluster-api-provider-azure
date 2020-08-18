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
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets/mock_subnets"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualnetworks/mock_virtualnetworks"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers"

	"k8s.io/klog/klogr"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/loadbalancers/mock_loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/network/mgmt/network"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

func TestReconcileLoadBalancer(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
			mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder)
	}{
		{
			name:          "public IP does not exist",
			expectedError: "public ip my-publicip not found in RG my-rg: #: Not found: StatusCode=404",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:         "my-publiclb",
						PublicIPName: "my-publicip",
						Role:         infrav1.APIServerRole,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				mPublicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "public IP retrieval fails",
			expectedError: "failed to look for existing public IP: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:         "my-publiclb",
						PublicIPName: "my-publicip",
						Role:         infrav1.APIServerRole,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				mPublicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "fail to create a public LB",
			expectedError: "failed to create load balancer my-publiclb: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:         "my-publiclb",
						PublicIPName: "my-publicip",
						Role:         infrav1.APIServerRole,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("testlocation")
				s.ClusterName().AnyTimes().Return("my-cluster")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				mPublicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil)
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", gomock.AssignableToTypeOf(network.LoadBalancer{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "create apiserver LB",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:          "my-publiclb",
						PublicIPName:  "my-publicip",
						Role:          infrav1.APIServerRole,
						APIServerPort: 6443,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("testlocation")
				s.ClusterName().AnyTimes().Return("my-cluster")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				gomock.InOrder(
					mPublicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{Name: to.StringPtr("my-publicip")}, nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", matchers.DiffEq(network.LoadBalancer{
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
							"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr(infrav1.APIServerRole),
						},
						Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
						Location: to.StringPtr("testlocation"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									Name: to.StringPtr("my-publiclb-frontEnd"),
									FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
										PrivateIPAllocationMethod: network.Dynamic,
										PublicIPAddress:           &network.PublicIPAddress{Name: to.StringPtr("my-publicip")},
									},
								},
							},
							BackendAddressPools: &[]network.BackendAddressPool{
								{
									Name: to.StringPtr("my-publiclb-backendPool"),
								},
							},
							LoadBalancingRules: &[]network.LoadBalancingRule{
								{
									Name: to.StringPtr("LBRuleHTTPS"),
									LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
										DisableOutboundSnat:  to.BoolPtr(true),
										Protocol:             network.TransportProtocolTCP,
										FrontendPort:         to.Int32Ptr(6443),
										BackendPort:          to.Int32Ptr(6443),
										IdleTimeoutInMinutes: to.Int32Ptr(4),
										EnableFloatingIP:     to.BoolPtr(false),
										LoadDistribution:     "Default",
										FrontendIPConfiguration: &network.SubResource{
											ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd"),
										},
										BackendAddressPool: &network.SubResource{
											ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
										},
										Probe: &network.SubResource{
											ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/probes/HTTPSProbe"),
										},
									},
								},
							},
							Probes: &[]network.Probe{
								{
									Name: to.StringPtr("tcpHTTPSProbe"),
									ProbePropertiesFormat: &network.ProbePropertiesFormat{
										Protocol:          network.ProbeProtocolTCP,
										Port:              to.Int32Ptr(6443),
										RequestPath:       to.StringPtr("/healthz"),
										IntervalInSeconds: to.Int32Ptr(15),
										NumberOfProbes:    to.Int32Ptr(4),
									},
								},
							},
							OutboundNatRules: &[]network.OutboundNatRule{
								{
									Name: to.StringPtr("OutboundNATAllProtocols"),
									OutboundNatRulePropertiesFormat: &network.OutboundNatRulePropertiesFormat{
										FrontendIPConfigurations: &[]network.SubResource{
											{ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd")},
										},
										BackendAddressPool: &network.SubResource{
											ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
										},
									},
								},
							},
						},
					})).Return(nil))
			},
		},
		{
			name:          "create node outbound LB",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:         "cluster-name",
						PublicIPName: "outbound-publicip",
						Role:         infrav1.NodeOutboundRole,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("testlocation")
				s.ClusterName().AnyTimes().Return("cluster-name")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				gomock.InOrder(
					mPublicIP.Get(context.TODO(), "my-rg", "outbound-publicip").Return(network.PublicIPAddress{Name: to.StringPtr("outbound-publicip")}, nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "cluster-name", matchers.DiffEq(network.LoadBalancer{
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster-name": to.StringPtr("owned"),
							"sigs.k8s.io_cluster-api-provider-azure_role":                 to.StringPtr(infrav1.NodeOutboundRole),
						},
						Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
						Location: to.StringPtr("testlocation"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									Name: to.StringPtr("cluster-name-frontEnd"),
									FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
										PrivateIPAllocationMethod: network.Dynamic,
										PublicIPAddress:           &network.PublicIPAddress{Name: to.StringPtr("outbound-publicip")},
									},
								},
							},
							BackendAddressPools: &[]network.BackendAddressPool{
								{
									Name: to.StringPtr("cluster-name-outboundBackendPool"),
								},
							},
							OutboundNatRules: &[]network.OutboundNatRule{
								{
									Name: to.StringPtr("OutboundNATAllProtocols"),
									OutboundNatRulePropertiesFormat: &network.OutboundNatRulePropertiesFormat{
										FrontendIPConfigurations: &[]network.SubResource{
											{ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/cluster-name/frontendIPConfigurations/cluster-name-frontEnd")},
										},
										BackendAddressPool: &network.SubResource{
											ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/cluster-name/backendAddressPools/cluster-name-outboundBackendPool"),
										},
									},
								},
							},
						},
					})).Return(nil))
			},
		},
		{
			name:          "internal load balancer does not exist",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:             "my-lb",
						SubnetCidr:       "10.0.0.0/16",
						SubnetName:       "my-subnet",
						PrivateIPAddress: "10.0.0.10",
						Role:             infrav1.InternalRole,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{
					ResourceGroup: "my-rg",
					Name:          "my-vnet",
				})
				s.Location().AnyTimes().Return("testlocation")
				s.ClusterName().AnyTimes().Return("cluster-name")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				mVnet.CheckIPAddressAvailability(context.TODO(), "my-rg", "my-vnet", "10.0.0.10").Return(network.IPAddressAvailabilityResult{Available: to.BoolPtr(true)}, nil)
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-lb", gomock.AssignableToTypeOf(network.LoadBalancer{}))
			},
		},
		{
			name:          "internal load balancer retrieval fails",
			expectedError: "failed to look for existing internal LB: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:             "my-lb",
						SubnetCidr:       "10.0.0.0/16",
						SubnetName:       "my-subnet",
						PrivateIPAddress: "10.0.0.10",
						Role:             infrav1.InternalRole,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{
					ResourceGroup: "my-rg",
					Name:          "my-vnet",
				})
				s.Location().AnyTimes().Return("testlocation")
				s.ClusterName().AnyTimes().Return("cluster-name")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "internal load balancer exists",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:             "my-lb",
						SubnetCidr:       "10.0.0.0/16",
						SubnetName:       "my-subnet",
						PrivateIPAddress: "10.0.0.10",
						Role:             infrav1.InternalRole,
						APIServerPort:    100,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{
					ResourceGroup: "my-rg",
					Name:          "my-vnet",
				})
				s.Location().AnyTimes().Return("testlocation")
				s.ClusterName().AnyTimes().Return("my-cluster")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{
					LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
						FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
							{
								FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
									PrivateIPAddress:          to.StringPtr("10.0.0.10"),
									PrivateIPAllocationMethod: network.Static,
								},
							},
						}}}, nil)
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-lb", matchers.DiffEq(network.LoadBalancer{
					Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
					Location: to.StringPtr("testlocation"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr(infrav1.InternalRole),
					},
					LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
						FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
							{
								Name: to.StringPtr("my-lb-frontEnd"),
								FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
									PrivateIPAllocationMethod: network.Static,
									PrivateIPAddress:          to.StringPtr("10.0.0.10"),
									Subnet:                    &network.Subnet{},
								},
							},
						},
						Probes: &[]network.Probe{
							{
								Name: to.StringPtr("tcpHTTPSProbe"),
								ProbePropertiesFormat: &network.ProbePropertiesFormat{
									Protocol:          network.ProbeProtocolTCP,
									RequestPath:       to.StringPtr("/healthz"),
									Port:              to.Int32Ptr(100),
									IntervalInSeconds: to.Int32Ptr(15),
									NumberOfProbes:    to.Int32Ptr(4),
								},
							},
						},
						BackendAddressPools: &[]network.BackendAddressPool{
							{
								Name: to.StringPtr("my-lb-backendPool"),
							},
						},
						LoadBalancingRules: &[]network.LoadBalancingRule{
							{
								Name: to.StringPtr("LBRuleHTTPS"),
								LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
									Protocol:             network.TransportProtocolTCP,
									FrontendPort:         to.Int32Ptr(100),
									BackendPort:          to.Int32Ptr(100),
									IdleTimeoutInMinutes: to.Int32Ptr(4),
									EnableFloatingIP:     to.BoolPtr(false),
									LoadDistribution:     "Default",
									FrontendIPConfiguration: &network.SubResource{
										ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-lb/frontendIPConfigurations/my-lb-frontEnd"),
									},
									BackendAddressPool: &network.SubResource{
										ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-lb/backendAddressPools/my-lb-backendPool"),
									},
									Probe: &network.SubResource{
										ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-lb/probes/HTTPSProbe"),
									},
								},
							},
						},
					},
				}))
			},
		},
		{
			name:          "internal load balancer does not exist and IP is not available",
			expectedError: "IP 10.0.0.10 is not available in VNet my-vnet and there were no other available IPs found",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:             "my-lb",
						SubnetCidr:       "10.0.0.0/16",
						SubnetName:       "my-subnet",
						PrivateIPAddress: "10.0.0.10",
						Role:             infrav1.InternalRole,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{
					ResourceGroup: "my-rg",
					Name:          "my-vnet",
				})
				s.Location().AnyTimes().Return("testlocation")
				s.ClusterName().AnyTimes().Return("cluster-name")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				mVnet.CheckIPAddressAvailability(context.TODO(), "my-rg", "my-vnet", "10.0.0.10").Return(network.IPAddressAvailabilityResult{Available: to.BoolPtr(false)}, nil)
			},
		},
		{
			name:          "internal load balancer does not exist and subnet does not exist",
			expectedError: "failed to get subnet: #: Not found: StatusCode=404",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:             "my-lb",
						SubnetCidr:       "10.0.0.0/16",
						SubnetName:       "my-subnet",
						PrivateIPAddress: "10.0.0.10",
						Role:             infrav1.InternalRole,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{
					ResourceGroup: "my-rg",
					Name:          "my-vnet",
				})
				s.Location().AnyTimes().Return("testlocation")
				s.ClusterName().AnyTimes().Return("cluster-name")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				mVnet.CheckIPAddressAvailability(context.TODO(), "my-rg", "my-vnet", "10.0.0.10").Return(network.IPAddressAvailabilityResult{Available: to.BoolPtr(true)}, nil)
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "create multiple LBs",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder, mSubnet *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:             "my-lb",
						SubnetCidr:       "10.0.0.0/16",
						SubnetName:       "my-subnet",
						PrivateIPAddress: "10.0.0.10",
						APIServerPort:    6443,
						Role:             infrav1.InternalRole,
					},
					{
						Name:          "my-lb-2",
						APIServerPort: 6443,
						PublicIPName:  "my-apiserver-ip",
						Role:          infrav1.APIServerRole,
					},
					{
						Name:         "my-lb-3",
						PublicIPName: "my-node-ip",
						Role:         infrav1.NodeOutboundRole,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{
					ResourceGroup: "my-rg",
					Name:          "my-vnet",
				})
				s.Location().AnyTimes().Return("testlocation")
				s.ClusterName().AnyTimes().Return("cluster-name")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				mVnet.CheckIPAddressAvailability(context.TODO(), "my-rg", "my-vnet", "10.0.0.10").Return(network.IPAddressAvailabilityResult{Available: to.BoolPtr(true)}, nil)
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				mPublicIP.Get(context.TODO(), "my-rg", "my-apiserver-ip").Return(network.PublicIPAddress{Name: to.StringPtr("my-apiserver-ip")}, nil)
				mPublicIP.Get(context.TODO(), "my-rg", "my-node-ip").Return(network.PublicIPAddress{Name: to.StringPtr("my-node-ip")}, nil)
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-lb", gomock.AssignableToTypeOf(network.LoadBalancer{}))
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-lb-2", gomock.AssignableToTypeOf(network.LoadBalancer{}))
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-lb-3", gomock.AssignableToTypeOf(network.LoadBalancer{}))
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_loadbalancers.NewMockLBScope(mockCtrl)
			clientMock := mock_loadbalancers.NewMockClient(mockCtrl)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)
			vnetMock := mock_virtualnetworks.NewMockClient(mockCtrl)
			subnetMock := mock_subnets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), publicIPsMock.EXPECT(), vnetMock.EXPECT(), subnetMock.EXPECT())

			s := &Service{
				Scope:                 scopeMock,
				Client:                clientMock,
				PublicIPsClient:       publicIPsMock,
				VirtualNetworksClient: vnetMock,
				SubnetsClient:         subnetMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteLoadBalancer(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete an existing load balancer",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name: "my-internallb",
					},
					{
						Name: "my-publiclb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-internallb")
				m.Delete(context.TODO(), "my-rg", "my-publiclb")
			},
		},
		{
			name:          "load balancer already deleted",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name: "my-publiclb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-publiclb").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "load balancer deletion fails",
			expectedError: "failed to delete load balancer my-publiclb in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name: "my-publiclb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-publiclb").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_loadbalancers.NewMockLBScope(mockCtrl)
			publicLBMock := mock_loadbalancers.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), publicLBMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: publicLBMock,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestGetAvailablePrivateIP(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name       string
		subnetCidr string
		expectedIP string
		expect     func(s *mock_loadbalancers.MockLBScopeMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder)
	}{
		{
			name:       "internal load balancer with a valid subnet cidr",
			subnetCidr: "10.0.8.0/16",
			expectedIP: "10.0.8.0",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder) {
				mVnet.CheckIPAddressAvailability(context.TODO(), "my-rg", "my-vnet", "10.0.8.0").Return(network.IPAddressAvailabilityResult{Available: to.BoolPtr(true)}, nil)
			},
		},
		{
			name:       "internal load balancer subnet cidr not 8 characters in length",
			subnetCidr: "10.64.8.0",
			expectedIP: "10.64.8.0",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder) {
				mVnet.CheckIPAddressAvailability(context.TODO(), "my-rg", "my-vnet", "10.64.8.0").Return(network.IPAddressAvailabilityResult{Available: to.BoolPtr(true)}, nil)
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			scopeMock := mock_loadbalancers.NewMockLBScope(mockCtrl)
			vnetMock := mock_virtualnetworks.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), vnetMock.EXPECT())

			s := &Service{
				Scope:                 scopeMock,
				VirtualNetworksClient: vnetMock,
			}

			resultIP, err := s.getAvailablePrivateIP(context.TODO(), "my-rg", "my-vnet", tc.subnetCidr, "")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resultIP).To(Equal(tc.expectedIP))
		})
	}
}
