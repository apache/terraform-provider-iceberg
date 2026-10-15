// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"github.com/apache/iceberg-go/table"
)

// commitRequirements builds optimistic-concurrency requirements for a table
// commit, matching Iceberg's UpdateRequirements.forUpdateTable rules:
//
//   - always assert table UUID
//   - add-schema → assert last-assigned field ID
//   - set-current-schema → assert current schema ID
//   - add-spec → assert last-assigned partition ID
//   - set-default-spec → assert default partition spec ID
//   - set-default-sort-order → assert default sort order ID
//
// Each assert is included at most once, even if multiple matching updates are
// present. Without these requirements, concurrent schema/partition commits can
// silently overwrite each other or allocate colliding IDs.
func commitRequirements(meta table.Metadata, updates []table.Update) []table.Requirement {
	reqs := []table.Requirement{
		table.AssertTableUUID(meta.TableUUID()),
	}

	var (
		addedSchema  bool
		setSchemaID  bool
		addedSpec    bool
		setSpecID    bool
		setSortOrder bool
	)

	for _, u := range updates {
		switch u.Action() {
		case table.UpdateAddSchema:
			if !addedSchema {
				reqs = append(reqs, table.AssertLastAssignedFieldID(meta.LastColumnID()))
				addedSchema = true
			}
		case table.UpdateSetCurrentSchema:
			if !setSchemaID {
				reqs = append(reqs, table.AssertCurrentSchemaID(meta.CurrentSchema().ID))
				setSchemaID = true
			}
		case table.UpdateAddSpec:
			if !addedSpec {
				if lastPartitionID := meta.LastPartitionSpecID(); lastPartitionID != nil {
					reqs = append(reqs, table.AssertLastAssignedPartitionID(*lastPartitionID))
				}
				addedSpec = true
			}
		case table.UpdateSetDefaultSpec:
			if !setSpecID {
				reqs = append(reqs, table.AssertDefaultSpecID(meta.DefaultPartitionSpec()))
				setSpecID = true
			}
		case table.UpdateSetDefaultSortOrder:
			if !setSortOrder {
				reqs = append(reqs, table.AssertDefaultSortOrderID(meta.DefaultSortOrder()))
				setSortOrder = true
			}
		}
	}

	return reqs
}
