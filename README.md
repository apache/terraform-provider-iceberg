<!--
 - Licensed to the Apache Software Foundation (ASF) under one or more
 - contributor license agreements.  See the NOTICE file distributed with
 - this work for additional information regarding copyright ownership.
 - The ASF licenses this file to You under the Apache License, Version 2.0
 - (the "License"); you may not use this file except in compliance with
 - the License.  You may obtain a copy of the License at
 -
 -   http://www.apache.org/licenses/LICENSE-2.0
 -
 - Unless required by applicable law or agreed to in writing, software
 - distributed under the License is distributed on an "AS IS" BASIS,
 - WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 - See the License for the specific language governing permissions and
 - limitations under the License.
 -->

# Iceberg Terraform Provider

This [Terraform](https://terraform.io) and [OpenTofu](https://www.opentofu.org/) provider allows you to manage Iceberg resources, such as namespaces and tables.

## Supported Data Sources

- `iceberg_namespace`: Read metadata for an existing namespace (catalog properties).
- `iceberg_table`: Read metadata for an existing table (schema, partition spec, sort order, and catalog properties).
- `iceberg_tables`: List table names in a namespace (catalog discovery).

## Supported Resources

- `iceberg_namespace`: Manage Iceberg namespaces and their properties.
- `iceberg_table`: Manage Iceberg tables, including schema definitions and properties.

See the [provider documentation](docs/index.md) for examples and attribute reference.

## Local Development

### Prerequisites

- [Go](https://golang.org/doc/install) (1.25.1 or later)
- [Terraform](https://www.terraform.io/downloads.html) or [OpenTofu](https://opentofu.org/docs/intro/install/)

### Building the Provider

To build the provider and install it locally , run the `build.sh` script:

```bash
./build.sh
```

This script will:
1. Compile the provider binary.
2. Create a local provider registry at `./terraform-plugins`.
3. Provide a configuration snippet for your `~/.terraformrc` (or `terraform.rc`) file to point Terraform to this local registry.

## Get in Touch

- [Iceberg community](https://iceberg.apache.org/community/)

