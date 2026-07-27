#!/bin/bash

# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
# 
#    http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# --- Configuration ---
PROVIDER_NAMESPACE="apache"
PROVIDER_TYPE="iceberg"
# Local dev build, not a release. Override with VERSION=0.1.0 ./build.sh
VERSION="${VERSION:-0.0.0}"
BINARY_BASE_NAME="terraform-provider-iceberg"
BINARY_NAME="${BINARY_BASE_NAME}_v${VERSION}"

# 1. Detect OS and Architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
[[ "$ARCH" == "x86_64" ]] && ARCH="amd64"
[[ "$ARCH" == "arm64" || "$ARCH" == "aarch64" ]] && ARCH="arm64"
OS_ARCH="${OS}_${ARCH}"

# 2. Define Paths
PLUGIN_BASE_DIR="$(pwd)/terraform-plugins"
SPECIFIC_PATH="${PLUGIN_BASE_DIR}/registry.terraform.io/${PROVIDER_NAMESPACE}/${PROVIDER_TYPE}/${VERSION}/${OS_ARCH}"

# 3. Build the Binary
echo "Building provider for $OS_ARCH..."
# We use -o to specify the exact filename Terraform expects
go build -o "$BINARY_NAME"

if [ $? -ne 0 ]; then
    echo "Error: Go build failed. Ensure you are in the provider source root and Go is installed."
    exit 1
fi

# 4. Create Directory Structure
echo "Creating directory: $SPECIFIC_PATH"
mkdir -p "$SPECIFIC_PATH"

# 5. Move binary to the mirror
mv "$BINARY_NAME" "$SPECIFIC_PATH/"
chmod +x "$SPECIFIC_PATH/$BINARY_NAME"
echo "Success: Binary built and moved to local mirror."

# 6. Output .terraformrc snippet
echo "-----------------------------------------------------------"
echo "Add the following to your ~/.terraformrc file:"
echo "-----------------------------------------------------------"
cat <<EOF
provider_installation {
  filesystem_mirror {
    path    = "$PLUGIN_BASE_DIR"
    include = ["registry.terraform.io/${PROVIDER_NAMESPACE}/${PROVIDER_TYPE}"]
  }
  direct {
    exclude = ["registry.terraform.io/${PROVIDER_NAMESPACE}/${PROVIDER_TYPE}"]
  }
}
EOF
