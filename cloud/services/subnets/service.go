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

package subnets

import (
	"github.com/chlau-az/cluster-api-provider-azure/cloud/scope"
	"github.com/chlau-az/cluster-api-provider-azure/cloud/services/routetables"
	"github.com/chlau-az/cluster-api-provider-azure/cloud/services/securitygroups"
)

// Service provides operations on azure resources
type Service struct {
	Scope *scope.ClusterScope
	Client
	SecurityGroupsClient securitygroups.Client
	RouteTablesClient    routetables.Client
}

// NewService creates a new service.
func NewService(scope *scope.ClusterScope) *Service {
	return &Service{
		Scope:                scope,
		Client:               NewClient(scope),
		SecurityGroupsClient: securitygroups.NewClient(scope),
		RouteTablesClient:    routetables.NewClient(scope),
	}
}
