/*
Copyright 2019 The Kubernetes Authors.

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

package virtualmachines

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/compute/mgmt/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
)

// Spec input specification for Get/CreateOrUpdate/Delete calls
type Spec struct {
	Name                   string
	NICNames               []string
	SSHKeyData             string
	Size                   string
	Zone                   string
	Image                  *infrav1.Image
	Identity               infrav1.VMIdentity
	OSDisk                 infrav1.OSDisk
	DataDisks              []infrav1.DataDisk
	CustomData             string
	UserAssignedIdentities []infrav1.UserAssignedIdentity
	SpotVMOptions          *infrav1.SpotVMOptions
}

// Get provides information about a virtual machine.
func (s *Service) Get(ctx context.Context, vmSpec *Spec) (*infrav1.VM, error) {
	vm, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), vmSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "VM %s not found", vmSpec.Name)
	} else if err != nil {
		return nil, err
	}

	convertedVM, err := converters.SDKToVM(vm)
	if err != nil {
		return convertedVM, err
	}

	// Discover addresses for NICs associated with the VM
	// and add them to our converted vm struct
	addresses, err := s.getAddresses(ctx, vm)
	if err != nil {
		return convertedVM, err
	}
	convertedVM.Addresses = addresses
	return convertedVM, nil
}

// Reconcile gets/creates/updates a virtual machine.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	vmSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid VM specification")
	}

	storageProfile, err := s.generateStorageProfile(ctx, *vmSpec)
	if err != nil {
		return err
	}

	nicRefs := make([]compute.NetworkInterfaceReference, len(vmSpec.NICNames))
	for i, nicName := range vmSpec.NICNames {
		primary := i == 0
		s.Scope.V(2).Info("getting network interface", "network interface", nicName)
		nic, err := s.InterfacesClient.Get(ctx, s.Scope.ResourceGroup(), nicName)
		if err != nil {
			return err
		}
		s.Scope.V(2).Info("got network interface", "network interface", nicName)
		nicRefs[i] = compute.NetworkInterfaceReference{
			ID: nic.ID,
			NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
				Primary: to.BoolPtr(primary),
			},
		}
	}

	s.Scope.V(2).Info("creating VM", "vm", vmSpec.Name)

	// Make sure to use the MachineScope here to get the merger of AzureCluster and AzureMachine tags
	additionalTags := s.MachineScope.AdditionalTags()
	// Set the cloud provider tag
	additionalTags[infrav1.ClusterAzureCloudProviderTagKey(s.MachineScope.Name())] = string(infrav1.ResourceLifecycleOwned)

	/*
		priority, evictionPolicy, billingProfile, err := getSpotVMOptions(vmSpec.SpotVMOptions)
		if err != nil {
			return errors.Wrapf(err, "failed to get Spot VM options")
		}
	*/

	virtualMachine := compute.VirtualMachine{
		Location: to.StringPtr(s.Scope.Location()),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.Scope.ClusterName(),
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        to.StringPtr(s.MachineScope.Name()),
			Role:        to.StringPtr(s.MachineScope.Role()),
			Additional:  additionalTags,
		})),
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			HardwareProfile: &compute.HardwareProfile{
				VMSize: compute.VirtualMachineSizeTypes(vmSpec.Size),
			},
			StorageProfile: storageProfile,
			OsProfile: &compute.OSProfile{
				ComputerName:  to.StringPtr(vmSpec.Name),
				AdminUsername: to.StringPtr(azure.DefaultUserName),
				CustomData:    to.StringPtr(vmSpec.CustomData),
				LinuxConfiguration: &compute.LinuxConfiguration{
					DisablePasswordAuthentication: to.BoolPtr(true),
					SSH: &compute.SSHConfiguration{
						PublicKeys: &[]compute.SSHPublicKey{
							{
								Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
								KeyData: to.StringPtr(vmSpec.SSHKeyData),
							},
						},
					},
				},
			},
			NetworkProfile: &compute.NetworkProfile{
				NetworkInterfaces: &nicRefs,
			},
		},
	}

	s.Scope.V(2).Info("Setting zone", "zone", vmSpec.Zone)

	if vmSpec.Zone != "" {
		zones := []string{vmSpec.Zone}
		virtualMachine.Zones = &zones
	}

	if vmSpec.Identity == infrav1.VMIdentitySystemAssigned {
		virtualMachine.Identity = &compute.VirtualMachineIdentity{
			Type: compute.ResourceIdentityTypeSystemAssigned,
		}
	} else if vmSpec.Identity == infrav1.VMIdentityUserAssigned {
		if len(vmSpec.UserAssignedIdentities) == 0 {
			return errors.Wrapf(err, "cannot create VM: The user-assigned identity provider ids must not be null or empty for 'UserAssigned' identity type.")
		}
		// UserAssignedIdentities - The list of user identities associated with the Virtual Machine.
		// The user identity dictionary key references will be ARM resource ids in the form:
		// '/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/{identityName}'.
		userIdentities := make([]string, len(vmSpec.UserAssignedIdentities))
		for _, id := range vmSpec.UserAssignedIdentities {
			key := id.ProviderID
			if strings.HasPrefix(id.ProviderID, "azure:///") {
				key = strings.TrimPrefix(key, "azure:///")
			}
			userIdentities = append(userIdentities, key)
		}
		virtualMachine.Identity = &compute.VirtualMachineIdentity{
			Type:        compute.ResourceIdentityTypeUserAssigned,
			IdentityIds: &userIdentities,
		}
		/*
			userIdentitiesMap := make(map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue, len(vmSpec.UserAssignedIdentities))
			for _, id := range vmSpec.UserAssignedIdentities {
				key := id.ProviderID
				if strings.HasPrefix(id.ProviderID, "azure:///") {
					key = strings.TrimPrefix(key, "azure:///")
				}
				userIdentitiesMap[key] = &compute.VirtualMachineIdentityUserAssignedIdentitiesValue{}
			}
			virtualMachine.Identity = &compute.VirtualMachineIdentity{
				Type:                   compute.ResourceIdentityTypeUserAssigned,
				UserAssignedIdentities: userIdentitiesMap,
			}
		*/
	}

	err = s.Client.CreateOrUpdate(
		ctx,
		s.Scope.ResourceGroup(),
		vmSpec.Name,
		virtualMachine)
	if err != nil {
		return errors.Wrapf(err, "cannot create VM")
	}

	s.Scope.V(2).Info("successfully created VM", "vm", vmSpec.Name)
	return nil
}

// Delete deletes the virtual machine with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	vmSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid VM specification")
	}
	s.Scope.V(2).Info("deleting VM", "vm", vmSpec.Name)
	err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), vmSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete VM %s in resource group %s", vmSpec.Name, s.Scope.ResourceGroup())
	}

	s.Scope.V(2).Info("successfully deleted VM", "vm", vmSpec.Name)
	return nil
}

func (s *Service) getAddresses(ctx context.Context, vm compute.VirtualMachine) ([]corev1.NodeAddress, error) {

	addresses := []corev1.NodeAddress{}

	if vm.NetworkProfile.NetworkInterfaces == nil {
		return addresses, nil
	}
	for _, nicRef := range *vm.NetworkProfile.NetworkInterfaces {

		// The full ID includes the name at the very end. Split the string and pull the last element
		// Ex: /subscriptions/$SUB/resourceGroups/$RG/providers/Microsoft.Network/networkInterfaces/$NICNAME
		// We'll check to see if ID is nil and bail early if we don't have it
		if nicRef.ID == nil {
			continue
		}
		nicName := getResourceNameByID(to.String(nicRef.ID))

		// Fetch nic and append its addresses
		nic, err := s.InterfacesClient.Get(ctx, s.Scope.ResourceGroup(), nicName)
		if err != nil {
			return addresses, err
		}

		if nic.IPConfigurations == nil {
			continue
		}
		for _, ipConfig := range *nic.IPConfigurations {
			if ipConfig.PrivateIPAddress != nil {
				addresses = append(addresses,
					corev1.NodeAddress{
						Type:    corev1.NodeInternalIP,
						Address: to.String(ipConfig.PrivateIPAddress),
					},
				)
			}

			if ipConfig.PublicIPAddress == nil {
				continue
			}
			// For some reason, the ID seems to be the only field populated in PublicIPAddress.
			// Thus, we have to go fetch the publicIP with the name.
			publicIPName := getResourceNameByID(to.String(ipConfig.PublicIPAddress.ID))
			publicNodeAddress, err := s.getPublicIPAddress(ctx, publicIPName)
			if err != nil {
				return addresses, err
			}
			addresses = append(addresses, publicNodeAddress)
		}
	}

	return addresses, nil
}

// getPublicIPAddress will fetch a public ip address resource by name and return a nodeaddresss representation
func (s *Service) getPublicIPAddress(ctx context.Context, publicIPAddressName string) (corev1.NodeAddress, error) {
	retAddress := corev1.NodeAddress{}
	publicIP, err := s.PublicIPsClient.Get(ctx, s.Scope.ResourceGroup(), publicIPAddressName)
	if err != nil {
		return retAddress, err
	}
	retAddress.Type = corev1.NodeExternalIP
	retAddress.Address = to.String(publicIP.IPAddress)

	return retAddress, nil
}

// getResourceNameById takes a resource ID like
// `/subscriptions/$SUB/resourceGroups/$RG/providers/Microsoft.Network/networkInterfaces/$NICNAME`
// and parses out the string after the last slash.
func getResourceNameByID(resourceID string) string {
	explodedResourceID := strings.Split(resourceID, "/")
	resourceName := explodedResourceID[len(explodedResourceID)-1]
	return resourceName
}

// generateStorageProfile generates a pointer to a compute.StorageProfile which can utilized for VM creation.
func (s *Service) generateStorageProfile(ctx context.Context, vmSpec Spec) (*compute.StorageProfile, error) {
	storageProfile := &compute.StorageProfile{
		OsDisk: &compute.OSDisk{
			Name:         to.StringPtr(azure.GenerateOSDiskName(vmSpec.Name)),
			OsType:       compute.OperatingSystemTypes(vmSpec.OSDisk.OSType),
			CreateOption: compute.DiskCreateOptionTypesFromImage,
			DiskSizeGB:   to.Int32Ptr(vmSpec.OSDisk.DiskSizeGB),
			ManagedDisk: &compute.ManagedDiskParameters{
				StorageAccountType: compute.StorageAccountTypes(vmSpec.OSDisk.ManagedDisk.StorageAccountType),
			},
		},
	}

	// enable ephemeral OS
	if vmSpec.OSDisk.DiffDiskSettings != nil {
		sku, err := s.ResourceSKUCache.Get(ctx, vmSpec.Size, resourceskus.VirtualMachines)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get find vm sku %s in compute api", vmSpec.Size)
		}

		if !sku.HasCapability(resourceskus.EphemeralOSDisk) {
			return nil, fmt.Errorf("vm size %s does not support ephemeral os. select a different vm size or disable ephemeral os", vmSpec.Size)
		}

		// storageProfile.OsDisk.DiffDiskSettings = &compute.DiffDiskSettings{
		// 	Option: compute.DiffDiskOptions(vmSpec.OSDisk.DiffDiskSettings.Option),
		// }
	}

	dataDisks := []compute.DataDisk{}
	for _, disk := range vmSpec.DataDisks {
		dataDisks = append(dataDisks, compute.DataDisk{
			CreateOption: compute.DiskCreateOptionTypesEmpty,
			DiskSizeGB:   to.Int32Ptr(disk.DiskSizeGB),
			Lun:          disk.Lun,
			Name:         to.StringPtr(azure.GenerateDataDiskName(vmSpec.Name, disk.NameSuffix)),
		})
	}
	storageProfile.DataDisks = &dataDisks

	imageRef, err := converters.ImageToSDK(vmSpec.Image)
	if err != nil {
		return nil, err
	}

	storageProfile.ImageReference = imageRef

	return storageProfile, nil
}

/*
func getSpotVMOptions(spotVMOptions *infrav1.SpotVMOptions) (compute.VirtualMachinePriorityTypes, compute.VirtualMachineEvictionPolicyTypes, *compute.BillingProfile, error) {
	// Spot VM not requested, return zero values to apply defaults
	if spotVMOptions == nil {
		return "", "", nil, nil
	}
	var billingProfile *compute.BillingProfile
	if spotVMOptions.MaxPrice != nil {
		maxPrice, err := strconv.ParseFloat(*spotVMOptions.MaxPrice, 64)
		if err != nil {
			return compute.VirtualMachinePriorityTypes(""), compute.VirtualMachineEvictionPolicyTypes(""), nil, err
		}
		billingProfile = &compute.BillingProfile{
			MaxPrice: &maxPrice,
		}
	}
	return compute.Spot, compute.Deallocate, billingProfile, nil
}
*/
