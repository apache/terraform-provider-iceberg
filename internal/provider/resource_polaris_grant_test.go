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

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const testGrantCatalogName = "test_grant_catalog"

func TestAccPolarisGrantCatalog(t *testing.T) {
	catalogURI := os.Getenv("POLARIS_CATALOG_URI")
	if catalogURI == "" {
		t.Skip("POLARIS_CATALOG_URI not set, skipping real-cluster test")
	}
	managementURI := os.Getenv("POLARIS_MANAGEMENT_URI")
	if managementURI == "" {
		managementURI = catalogURI + "/api/management/v1"
	}
	token := os.Getenv("POLARIS_TOKEN")

	providerCfg := testAccPolarisProviderConfigWithToken(catalogURI, managementURI, token)

	roleName := "test-grant-catalog-role"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerCfg + fmt.Sprintf(`
resource "iceberg_polaris_catalog_role" "test" {
  catalog_name = "%s"
  name         = "%s"
}

resource "iceberg_polaris_grant" "test" {
  catalog_name      = iceberg_polaris_catalog_role.test.catalog_name
  catalog_role_name = iceberg_polaris_catalog_role.test.name
  privilege         = "TABLE_LIST"
  catalog           = iceberg_polaris_catalog_role.test.catalog_name
}
`, testGrantCatalogName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("iceberg_polaris_grant.test", "privilege", "TABLE_LIST"),
				),
			},
		},
	})
}
