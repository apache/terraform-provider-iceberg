// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const (
	providerConfig = `
provider "iceberg" {
  catalog_uri = "%s"
}
`
)

func testAccPreCheck(t *testing.T) {
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"iceberg": providerserver.NewProtocol6WithError(New()()),
}

func TestAccIcebergNamespace(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIcebergNamespaceResourceConfig(providerCfg, "test description"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_namespace.test", "name.0", "db1"),
					resource.TestCheckResourceAttr("iceberg_namespace.test", "user_properties.description", "test description"),
					resource.TestCheckResourceAttr("iceberg_namespace.test", "server_properties.description", "test description"),
				),
			},
			{
				Config: testAccIcebergNamespaceResourceConfig(providerCfg, "updated description"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_namespace.test", "name.0", "db1"),
					resource.TestCheckResourceAttr("iceberg_namespace.test", "user_properties.description", "updated description"),
					resource.TestCheckResourceAttr("iceberg_namespace.test", "server_properties.description", "updated description"),
				),
			},
			{
				Config: testAccIcebergNamespaceResourceConfig(providerCfg, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_namespace.test", "name.0", "db1"),
					resource.TestCheckNoResourceAttr("iceberg_namespace.test", "user_properties.description"),
					resource.TestCheckNoResourceAttr("iceberg_namespace.test", "server_properties.description"),
				),
			},
			{
				ResourceName:      "iceberg_namespace.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"server_properties",
				},
			},
		},
	})
}

func TestAccIcebergNamespaceNested(t *testing.T) {
	catalogURI := os.Getenv("ICEBERG_CATALOG_URI")
	if catalogURI == "" {
		catalogURI = "http://localhost:8181"
	}

	providerCfg := fmt.Sprintf(providerConfig, catalogURI)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create a multi-level namespace with a property. This exercises
				// the URL-encoded unit separator (%1F) used to join the name
				// parts and confirms properties round-trip on nested namespaces.
				Config: testAccIcebergNamespaceNestedConfig(providerCfg, "data-team"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_namespace.nested", "name.0", "analytics"),
					resource.TestCheckResourceAttr("iceberg_namespace.nested", "name.1", "prod"),
					resource.TestCheckResourceAttr("iceberg_namespace.nested", "id", "analytics.prod"),
					resource.TestCheckResourceAttr("iceberg_namespace.nested", "user_properties.owner", "data-team"),
					resource.TestCheckResourceAttr("iceberg_namespace.nested", "server_properties.owner", "data-team"),
				),
			},
			{
				// Drop the property so the state has no user_properties before
				// import (import cannot reconstruct user-managed properties).
				Config: testAccIcebergNamespaceNestedConfig(providerCfg, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_namespace.nested", "id", "analytics.prod"),
					resource.TestCheckNoResourceAttr("iceberg_namespace.nested", "user_properties.owner"),
				),
			},
			{
				ResourceName:      "iceberg_namespace.nested",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"server_properties",
				},
			},
		},
	})
}

func testAccIcebergNamespaceNestedConfig(providerCfg string, owner string) string {
	propsStr := ""
	if owner != "" {
		propsStr = fmt.Sprintf(`user_properties = {
    owner = "%s"
  }`, owner)
	}

	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "nested" {
  name = ["analytics", "prod"]
  %s
}
`, propsStr)
}

func testAccIcebergNamespaceResourceConfig(providerCfg string, description string) string {
	propsStr := ""
	if description != "" {
		propsStr = fmt.Sprintf(`user_properties = {
    description = "%s"
  }`, description)
	}

	return providerCfg + fmt.Sprintf(`
resource "iceberg_namespace" "test" {
  name        = ["db1"]
  %s
}
`, propsStr)
}
