#!/usr/bin/env bash
#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#
# Verifies a release candidate of the Apache Iceberg Terraform provider so you
# can cast an informed vote. It downloads the source release from the Apache dev
# SVN and checks its GPG signature and checksum, runs the Apache RAT license
# check, then builds and tests the provider from source in a throwaway sandbox.
#
# Usage: dev/release/verify_rc.sh <version> <rc>
#  e.g.: dev/release/verify_rc.sh 0.7.0 1
#
# A valid signature from a key in the KEYS file, a matching checksum, a clean RAT
# check, and a clean build/test is a +1.

set -eu

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOP_SOURCE_DIR="$(dirname "$(dirname "${SOURCE_DIR}")")"

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <version> <rc>"
  echo " e.g.: $0 0.7.0 1"
  exit 1
fi

set -o pipefail
set -x

VERSION="$1"
RC="$2"

ICEBERG_DIST_BASE_URL="https://downloads.apache.org/iceberg"
DOWNLOAD_RC_BASE_URL="https://dist.apache.org/repos/dist/dev/iceberg/apache-iceberg-terraform-${VERSION}-rc${RC}"
ARCHIVE_BASE_NAME="apache-iceberg-terraform-${VERSION}"

REPOSITORY="apache/terraform-provider-iceberg"
RC_TAG="v${VERSION}-rc${RC}"
# The convenience binaries and their SHA256SUMS are named with the final version.
BINARY_BASE_NAME="terraform-provider-iceberg_${VERSION}"

: "${VERIFY_DEFAULT:=1}"
: "${VERIFY_DOWNLOAD:=${VERIFY_DEFAULT}}"
: "${VERIFY_FORCE_USE_GO_BINARY:=0}"
: "${VERIFY_SIGN:=${VERIFY_DEFAULT}}"
: "${VERIFY_RAT:=${VERIFY_DEFAULT}}"
: "${VERIFY_BINARY:=${VERIFY_DEFAULT}}"

VERIFY_SUCCESS=no

setup_tmpdir() {
  cleanup() {
    if [ "${VERIFY_SUCCESS}" = "yes" ]; then
      rm -rf "${VERIFY_TMPDIR}"
    else
      echo "Failed to verify release candidate. See ${VERIFY_TMPDIR} for details."
    fi
  }

  if [ -z "${VERIFY_TMPDIR:-}" ]; then
    VERIFY_TMPDIR="$(mktemp -d -t "$1.XXXXX")"
    trap cleanup EXIT
  else
    mkdir -p "${VERIFY_TMPDIR}"
  fi
}

download() {
  curl \
    --fail \
    --location \
    --remote-name \
    --show-error \
    --silent \
    "$1"
}

download_rc_file() {
  if [ "${VERIFY_DOWNLOAD}" -gt 0 ]; then
    download "${DOWNLOAD_RC_BASE_URL}/$1"
  else
    cp "${TOP_SOURCE_DIR}/$1" "$1"
  fi
}

import_gpg_keys() {
  if [ "${VERIFY_SIGN}" -gt 0 ]; then
    download "${ICEBERG_DIST_BASE_URL}/KEYS"
    gpg --import KEYS
  fi
}

if type shasum >/dev/null 2>&1; then
  sha512_verify="shasum -a 512 -c"
  sha256_verify="shasum -a 256 -c"
else
  sha512_verify="sha512sum -c"
  sha256_verify="sha256sum -c"
fi

fetch_archive() {
  download_rc_file "${ARCHIVE_BASE_NAME}.tar.gz"
  if [ "${VERIFY_SIGN}" -gt 0 ]; then
    download_rc_file "${ARCHIVE_BASE_NAME}.tar.gz.asc"
    gpg --verify "${ARCHIVE_BASE_NAME}.tar.gz.asc" "${ARCHIVE_BASE_NAME}.tar.gz"
  fi
  download_rc_file "${ARCHIVE_BASE_NAME}.tar.gz.sha512"
  ${sha512_verify} "${ARCHIVE_BASE_NAME}.tar.gz.sha512"
}

ensure_source_directory() {
  tar xf "${ARCHIVE_BASE_NAME}".tar.gz
}

verify_binary_distribution() {
  if [ "${VERIFY_BINARY}" -le 0 ]; then
    return
  fi
  # TF Registry consumes convenience binaries.
  # These lives on GitHub prerelease, not dev SVN.
  local binary_dir="binaries"
  rm -rf "${binary_dir}"
  mkdir -p "${binary_dir}"
  gh release download "${RC_TAG}" \
    --repo "${REPOSITORY}" \
    --dir "${binary_dir}" \
    --pattern "${BINARY_BASE_NAME}_*"
  (
    cd "${binary_dir}"
    if [ "${VERIFY_SIGN}" -gt 0 ]; then
      gpg --verify "${BINARY_BASE_NAME}_SHA256SUMS.sig" "${BINARY_BASE_NAME}_SHA256SUMS"
    fi
    ${sha256_verify} "${BINARY_BASE_NAME}_SHA256SUMS"
  )
}

latest_go_version() {
  local -a options
  options=(
    --fail
    --location
    --show-error
    --silent
  )
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    options+=("--header" "Authorization: Bearer ${GITHUB_TOKEN}")
  fi
  curl \
    "${options[@]}" \
    https://api.github.com/repos/golang/go/git/matching-refs/tags/go |
  jq -r ' .[] | .ref' |
  sort -V |
  tail -1 |
  sed 's,refs/tags/go,,g'
}

ensure_go() {
  if [ "${VERIFY_FORCE_USE_GO_BINARY}" -le 0 ]; then
    if go version; then
      GOPATH="${VERIFY_TMPDIR}/gopath"
      export GOPATH
      mkdir -p "${GOPATH}"
      return
    fi
  fi

  local go_version
  go_version=$(latest_go_version)
  local go_os
  go_os="$(uname)"
  case "${go_os}" in
  Darwin)
    go_os="darwin"
    ;;
  Linux)
    go_os="linux"
    ;;
  esac
  local go_arch
  go_arch="$(arch)"
  case "${go_arch}" in
  i386 | x86_64)
    go_arch="amd64"
    ;;
  aarch64)
    go_arch="arm64"
    ;;
  esac
  local go_binary_tar_gz
  go_binary_tar_gz="go${go_version}.${go_os}-${go_arch}.tar.gz"
  local go_binary_url
  go_binary_url="https://go.dev/dl/${go_binary_tar_gz}"
  curl \
    --fail \
    --location \
    --output "${go_binary_tar_gz}" \
    --show-error \
    --silent \
    "${go_binary_url}"
  tar xf "${go_binary_tar_gz}"
  GOROOT="$(pwd)/go"
  export GOROOT
  GOPATH="$(pwd)/gopath"
  export GOPATH
  mkdir -p "${GOPATH}"
  PATH="${GOROOT}/bin:${GOPATH}/bin:${PATH}"
}

run_rat_check() {
  if [ "${VERIFY_RAT}" -gt 0 ]; then
    # dev/check-license downloads Apache RAT and audits the extracted tree using
    # the provider's own rat_exclude_files.txt.
    ./dev/check-license
  fi
}

build_and_test_source_distribution() {
  go build ./...
  go test ./...
}

setup_tmpdir "iceberg-terraform-${VERSION}-${RC}"
echo "Working in sandbox ${VERIFY_TMPDIR}"
cd "${VERIFY_TMPDIR}"

import_gpg_keys
fetch_archive
verify_binary_distribution
ensure_source_directory
ensure_go
pushd "${ARCHIVE_BASE_NAME}"
run_rat_check
build_and_test_source_distribution
popd

VERIFY_SUCCESS=yes
echo "RC looks good!"
