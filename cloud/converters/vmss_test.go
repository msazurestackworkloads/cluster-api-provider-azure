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

package converters_test

import (
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/compute/mgmt/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/onsi/gomega"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
)

func Test_SDKToVMSS(t *testing.T) {
	cases := []struct {
		Name           string
		SubjectFactory func(*gomega.GomegaWithT) (compute.VirtualMachineScaleSet, []compute.VirtualMachineScaleSetVM)
		Expect         func(*gomega.GomegaWithT, *infrav1exp.VMSS)
	}{
		{
			Name: "ShouldPopulateWithData",
			SubjectFactory: func(g *gomega.GomegaWithT) (compute.VirtualMachineScaleSet, []compute.VirtualMachineScaleSetVM) {
				tags := map[string]*string{
					"foo": to.StringPtr("bazz"),
				}
				// zones := []string{"zone0", "zone1"}
				return compute.VirtualMachineScaleSet{
						Sku: &compute.Sku{
							Name:     to.StringPtr("skuName"),
							Tier:     to.StringPtr("skuTier"),
							Capacity: to.Int64Ptr(2),
						},
						// Zones:    to.StringSlicePtr(zones),
						ID:       to.StringPtr("vmssID"),
						Name:     to.StringPtr("vmssName"),
						Location: to.StringPtr("westus2"),
						Tags:     tags,
						VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
							ProvisioningState: to.StringPtr("Succeded"),
						},
					},
					[]compute.VirtualMachineScaleSetVM{
						{
							InstanceID: to.StringPtr("0"),
							ID:         to.StringPtr("vm/0"),
							Name:       to.StringPtr("vm0"),
							// Zones:      to.StringSlicePtr([]string{"zone0"}),
							VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
								ProvisioningState: to.StringPtr("Succeeded"),
							},
						},
						{
							InstanceID: to.StringPtr("1"),
							ID:         to.StringPtr("vm/1"),
							Name:       to.StringPtr("vm1"),
							// Zones:      to.StringSlicePtr([]string{"zone1"}),
							VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
								ProvisioningState: to.StringPtr("Succeeded"),
							},
						},
					}
			},
			Expect: func(g *gomega.GomegaWithT, actual *infrav1exp.VMSS) {
				expected := infrav1exp.VMSS{
					ID:       "vmssID",
					Name:     "vmssName",
					Sku:      "skuName",
					Capacity: 2,
					// Zones:    []string{"zone0", "zone1"},
					State: "Succeeded",
					Tags: map[string]string{
						"foo": "bazz",
					},
					Instances: make([]infrav1exp.VMSSVM, 2),
				}

				for i := 0; i < 2; i++ {
					expected.Instances[i] = infrav1exp.VMSSVM{
						ID:               fmt.Sprintf("vm/%d", i),
						InstanceID:       fmt.Sprintf("%d", i),
						Name:             fmt.Sprintf("vm%d", i),
						AvailabilityZone: fmt.Sprintf("zone%d", i),
						State:            "Succeeded",
					}
				}
				g.Expect(actual).To(gomega.Equal(&expected))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)
			vmss, instances := c.SubjectFactory(g)
			subject := converters.SDKToVMSS(vmss, instances)
			c.Expect(g, subject)
		})
	}
}
