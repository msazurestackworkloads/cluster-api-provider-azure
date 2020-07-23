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
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
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
	// AzureStackCloud
	AzureStackCloud = "AzureStackCloud"
)

// AzureClients contains all the Azure clients used by the scopes.
type AzureClients struct {
	SubscriptionID             string
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

	armEndpoint := os.Getenv("AZURE_ARM_ENDPOINT")
	log.Println("HERE armEndpoint: ", armEndpoint)
	env, err := azure.EnvironmentFromURL(armEndpoint)
	if err != nil {
		log.Println("HERE error getting environment from armEndpoint: ", armEndpoint)
		return err
	}

	c.ResourceManagerEndpoint = env.ResourceManagerEndpoint
	log.Println("HERE c.ResourceManagerEndpoint: ", c.ResourceManagerEndpoint)
	c.ResourceManagerVMDNSSuffix = GetAzureDNSZoneForEnvironment("AzureStackCloud")
	log.Println("HERE c.ResourceManagerVMDNSSuffix: ", c.ResourceManagerVMDNSSuffix)
	c.Authorizer, err = getAuthorizerForResource(env)
	log.Println("HERE c.Authorizer: ", c.Authorizer)
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
	case AzureStackCloud:
		armEndpoint := os.Getenv("AZURE_ARM_ENDPOINT")
		azsFQDNSuffix := getAzureStackFQDNSuffix(armEndpoint)
		return fmt.Sprintf("cloudapp.%s", azsFQDNSuffix)
	default:
		return "cloudapp.azure.com"
	}
}

func getAzureStackFQDNSuffix(portalURL string) string {
	azsFQDNSuffix := strings.Replace(portalURL, "https://management.", "", -1)
	azsFQDNSuffix = strings.Join(strings.Split(azsFQDNSuffix, ".")[1:], ".") //remove location prefix
	azsFQDNSuffix = strings.TrimSuffix(azsFQDNSuffix, "/")
	return azsFQDNSuffix
}

// getAuthorizerForResource gets an OAuthTokenAuthorizer for Azure Resource Manager
func getAuthorizerForResource(env azure.Environment) (autorest.Authorizer, error) {
	var a autorest.Authorizer
	var err error
	var oauthConfig *adal.OAuthConfig

	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	tenantID := os.Getenv("AZURE_TENANT_ID")
	log.Println("HERE client id: ", clientID)
	log.Println("HERE client secret: ", clientSecret)
	log.Println("HERE tenant id: ", tenantID)

	tokenAudience := env.TokenAudience
	log.Println("HERE TokenAudience: ", env.TokenAudience)
	log.Println("HERE ActiveDirectoryEndpoint: ", env.ActiveDirectoryEndpoint)

	identitySystem := os.Getenv("IDENTITY_SYSTEM")
	log.Println("HERE identity system: ", identitySystem)
	if identitySystem == "adfs" {
		oauthConfig, err = adal.NewOAuthConfig(
			env.ActiveDirectoryEndpoint, "adfs")
	} else {
		oauthConfig, err = adal.NewOAuthConfig(
			env.ActiveDirectoryEndpoint, tenantID)
	}
	if err != nil {
		return nil, err
	}

	token, err := adal.NewServicePrincipalToken(
		*oauthConfig,
		clientID,
		clientSecret,
		tokenAudience)

	log.Println("HERE generated token")
	a = autorest.NewBearerAuthorizer(token)
	log.Println("HERE generated authorizer")
	return a, err
}
