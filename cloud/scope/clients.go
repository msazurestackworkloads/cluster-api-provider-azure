/*
Copyright 2018 The Kubernetes Authors.

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

package scope

import (
	"log"
	"os"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
)

const (
	// ChinaCloud is the cloud environment operated in China
	ChinaCloud = "AzureChinaCloud"
	// GermanCloud is the cloud environment operated in Germany
	GermanCloud = "AzureGermanCloud"
	// PublicCloud is the default public Azure cloud environment
	PublicCloud = "AzurePublicCloud"
	// USGovernmentCloud is the cloud environment for the US Government
	USGovernmentCloud = "AzureUSGovernmentCloud"
)

// AzureClients contains all the Azure clients used by the scopes.
type AzureClients struct {
	SubscriptionID             string
	ClientID                   string
	ClientSecret               string
	TenantID                   string
	ResourceManagerEndpoint    string
	ResourceManagerVMDNSSuffix string
	Authorizer                 autorest.Authorizer
}

func (c *AzureClients) setCredentials(subscriptionID string) error {
	subID, err := getSubscriptionID(subscriptionID)
	if err != nil {
		return err
	}
	c.SubscriptionID = subID
	c.ClientID = os.Getenv("AZURE_CLIENT_ID")
	c.ClientSecret = os.Getenv("AZURE_CLIENT_SECRET")
	c.TenantID = os.Getenv("AZURE_TENANT_ID")
	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return err
	}

	// To do: get arm endpoint in helper method
	log.Println("HERE changing environment")
	armEndpoint := os.Getenv("AZURE_ARM_ENDPOINT")
	log.Println("HERE armEndpoint: ", armEndpoint)
	settings.Environment, _ = azure.EnvironmentFromURL(armEndpoint)

	c.ResourceManagerEndpoint = settings.Environment.ResourceManagerEndpoint
	log.Println("HERE resource manager endpoint: ", c.ResourceManagerEndpoint)
	c.ResourceManagerVMDNSSuffix = GetAzureDNSZoneForEnvironment(settings.Environment.Name)
	settings.Values[auth.SubscriptionID] = subscriptionID
	// c.Authorizer, err = settings.GetAuthorizer()
	c.Authorizer, err = c.getAuthorizerForResource(settings.Environment)
	log.Println("HERE c.Authorizer: ", c.Authorizer, "err: ", err)
	return err
}

func getSubscriptionID(subscriptionID string) (string, error) {
	if subscriptionID != "" {
		return subscriptionID, nil
	}
	subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		return "", errors.New("error creating azure services. Environment variable AZURE_SUBSCRIPTION_ID is not set")
	}
	return subscriptionID, nil
}

// GetAzureDNSZoneForEnvironment returnes the DNSZone to be used with the
// cloud environment, the default is the public cloud
func GetAzureDNSZoneForEnvironment(environmentName string) string {
	// default is public cloud
	switch environmentName {
	case ChinaCloud:
		return "cloudapp.chinacloudapi.cn"
	case GermanCloud:
		return "cloudapp.microsoftazure.de"
	case PublicCloud:
		return "cloudapp.azure.com"
	case USGovernmentCloud:
		return "cloudapp.usgovcloudapi.net"
	default:
		return "cloudapp.azure.com"
	}
}

// getAuthorizerForResource gets an OAuthTokenAuthorizer for Azure Resource Manager
func (c *AzureClients) getAuthorizerForResource(env azure.Environment) (autorest.Authorizer, error) {
	var a autorest.Authorizer
	var err error
	var oauthConfig *adal.OAuthConfig

	tokenAudience := env.TokenAudience
	log.Println("HERE TokenAudience: ", env.TokenAudience)
	log.Println("HERE ActiveDirectoryEndpoint: ", env.ActiveDirectoryEndpoint)
	oauthConfig, err = adal.NewOAuthConfig(
		env.ActiveDirectoryEndpoint, c.TenantID)

	if err != nil {
		return nil, err
	}
	token, err := adal.NewServicePrincipalToken(
		*oauthConfig,
		c.ClientID,
		c.ClientSecret,
		tokenAudience)

	log.Println("HERE generated token")
	a = autorest.NewBearerAuthorizer(token)
	log.Println("HERE generated authorizer")
	return a, err
}
