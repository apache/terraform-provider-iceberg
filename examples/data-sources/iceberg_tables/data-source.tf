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

resource "iceberg_namespace" "example" {
  name = ["example_namespace"]
}

resource "iceberg_table" "events" {
  namespace = iceberg_namespace.example.name
  name      = "events"

  schema = {
    fields = [
      {
        name     = "id"
        type     = "long"
        required = true
      }
    ]
  }
}

resource "iceberg_table" "orders" {
  namespace = iceberg_namespace.example.name
  name      = "orders"

  schema = {
    fields = [
      {
        name     = "id"
        type     = "long"
        required = true
      }
    ]
  }
}

data "iceberg_tables" "example" {
  namespace = iceberg_namespace.example.name

  depends_on = [
    iceberg_table.events,
    iceberg_table.orders,
  ]
}

output "table_names" {
  value = data.iceberg_tables.example.tables
}

output "table_identifiers" {
  value = data.iceberg_tables.example.identifiers
}

output "namespace_id" {
  value = data.iceberg_tables.example.id
}
