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
# Cuts a release candidate for the Apache Iceberg Terraform provider:
#
#   1. Tags v<version>-rc<rc> and pushes it, which triggers the
#      "Terraform Build Release Candidate" workflow (../../.github/workflows/tf-release.yml).
#   2. Waits for that workflow to publish the GitHub prerelease, then downloads
#      its artifacts.
#   3. Signs the source tarball (.asc) and the binary SHA256SUMS (.sig) with your
#      KEYS-registered GPG key and uploads the signatures back to the prerelease.
#   4. Uploads the signed *source* release to the Apache dev SVN (the artifact the
#      PMC votes on). Convenience binaries stay on the GitHub prerelease.
#   5. Prints the [VOTE] email for dev@iceberg.apache.org.
#
# Usage: dev/release/release_rc.sh <version> <rc>
#  e.g.: dev/release/release_rc.sh 0.7.0 1
#
# Set RELEASE_PUSH_TAG=0 / RELEASE_SIGN=0 / RELEASE_UPLOAD=0 (or RELEASE_DEFAULT=0)
# to dry-run individual stages.

set -eu

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_TOP_DIR="$(cd "${SOURCE_DIR}/../../" && pwd)"

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <version> <rc>"
  echo " e.g.: $0 0.7.0 1"
  exit 1
fi

version=$1
rc=$2

: "${RELEASE_DEFAULT:=1}"
: "${RELEASE_PULL:=${RELEASE_DEFAULT}}"
: "${RELEASE_PUSH_TAG:=${RELEASE_DEFAULT}}"
: "${RELEASE_SIGN:=${RELEASE_DEFAULT}}"
: "${RELEASE_UPLOAD:=${RELEASE_DEFAULT}}"

cd "${SOURCE_TOP_DIR}"

if [ "${RELEASE_PULL}" -gt 0 ] || [ "${RELEASE_PUSH_TAG}" -gt 0 ]; then
  git_origin_url="$(git remote get-url origin)"
  if [ "${git_origin_url}" != "git@github.com:apache/terraform-provider-iceberg.git" ]; then
    echo "This script must be run with a working copy of apache/terraform-provider-iceberg."
    echo "The origin's URL: ${git_origin_url}"
    exit 1
  fi
fi

if [ "${RELEASE_PULL}" -gt 0 ]; then
  # Refresh the current branch (main for major/minor, iceberg-terraform-X.Y.x
  # for a patch release). Check out the correct branch before running.
  echo "Ensuring the current branch is up to date"
  git pull --rebase --prune
fi

rc_tag="v${version}-rc${rc}"
if [ "${RELEASE_PUSH_TAG}" -gt 0 ]; then
  echo "Tagging for RC: ${rc_tag}"
  git tag -a -m "Apache Iceberg Terraform provider ${version} RC${rc}" "${rc_tag}"
  git push origin "${rc_tag}"
fi

rc_hash="$(git rev-list --max-count=1 "${rc_tag}")"

repository="apache/terraform-provider-iceberg"

# Names of the artifacts published by the workflow to the GitHub prerelease.
rc_tar_gz="apache-iceberg-terraform-${version}-rc${rc}.tar.gz"
sha256sums="terraform-provider-iceberg_${version}_SHA256SUMS"

# id is both the local staging directory and the Apache dev SVN directory name.
# It carries the -rc suffix; the files inside are renamed to drop it below.
id="apache-iceberg-terraform-${version}-rc${rc}"

if [ "${RELEASE_SIGN}" -gt 0 ]; then
  echo "Looking for the release workflow run on ${repository}:${rc_tag}"
  run_id=""
  max_attempts=60 # 60 x 5s = 5 minutes
  attempt=0
  while [ -z "${run_id}" ]; do
    attempt=$((attempt + 1))
    if [ "${attempt}" -gt "${max_attempts}" ]; then
      echo "Timed out after $((max_attempts * 5))s waiting for the tf-release.yml run on ${rc_tag}."
      echo "Check: gh run list --repo ${repository} --workflow=tf-release.yml"
      exit 1
    fi
    echo "Waiting for the run to start... (attempt ${attempt}/${max_attempts})"
    run_id=$(gh run list \
      --repo "${repository}" \
      --workflow=tf-release.yml \
      --json 'databaseId,event,headBranch,status' \
      --jq ".[] | select(.event == \"push\" and .headBranch == \"${rc_tag}\") | .databaseId")
    [ -z "${run_id}" ] && sleep 5
  done

  echo "Found workflow run ${run_id}; waiting for it to finish..."
  gh run watch --repo "${repository}" --exit-status "${run_id}"

  rm -rf "${id}"
  mkdir -p "${id}"

  echo "Downloading the source tarball and checksum from the prerelease"
  gh release download "${rc_tag}" \
    --dir "${id}" \
    --pattern "${rc_tar_gz}" \
    --pattern "${rc_tar_gz}.sha512" \
    --repo "${repository}" \
    --skip-existing

  echo "Signing the source tarball"
  (
    cd "${id}"
    gpg --armor --output "${rc_tar_gz}.asc" --detach-sig "${rc_tar_gz}"
  )
  echo "Adding the source signature to the prerelease"
  gh release upload "${rc_tag}" \
    --clobber \
    --repo "${repository}" \
    "${id}/${rc_tar_gz}.asc"

  echo "Signing the binary checksums (SHA256SUMS) for the registry"
  rm -rf binaries-rc
  mkdir -p binaries-rc
  gh release download "${rc_tag}" \
    --dir binaries-rc \
    --pattern "${sha256sums}" \
    --repo "${repository}" \
    --skip-existing
  (
    cd binaries-rc
    gpg --batch --yes --detach-sign --output "${sha256sums}.sig" "${sha256sums}"
  )
  echo "Adding the SHA256SUMS signature to the prerelease"
  gh release upload "${rc_tag}" \
    --clobber \
    --repo "${repository}" \
    "binaries-rc/${sha256sums}.sig"
fi

if [ "${RELEASE_UPLOAD}" -gt 0 ]; then
  echo "Uploading the source release to ASF dist/dev..."
  # Rename the source files to drop the -rc${rc} suffix so the artifact that
  # lands in SVN is byte-identical in name to the final release.
  pushd "${id}"
  for fname in ./*; do
    mv "${fname}" "${fname//-rc${rc}/}"
  done
  # Regenerate the checksum against the renamed tarball.
  tar_gz="apache-iceberg-terraform-${version}.tar.gz"
  sha512sum "${tar_gz}" > "${tar_gz}.sha512"
  popd

  svn import "${id}" \
    "https://dist.apache.org/repos/dist/dev/iceberg/${id}" \
    -m "Apache Iceberg Terraform provider ${version} RC${rc}"
fi

echo "Draft email for the dev@iceberg.apache.org mailing list"
echo ""
echo "---------------------------------------------------------"
cat <<MAIL
To: dev@iceberg.apache.org
Subject: [VOTE][Terraform] Release Apache Iceberg Terraform provider v${version} RC${rc}

Hi,

I would like to propose the following release candidate (RC${rc}) of the
Apache Iceberg Terraform provider version v${version}.

This release candidate is based on commit:
${rc_hash} [1]

The source release rc${rc} is hosted at [2].
Convenience binaries for the Terraform/OpenTofu registries are attached to the
GitHub prerelease at [3].

Please download, verify checksums and signatures, run the tests, and vote on
the release. See [4] for how to validate a release candidate.

The vote will be open for at least 72 hours.

[ ] +1 Release this as Apache Iceberg Terraform provider v${version}
[ ] +0
[ ] -1 Do not release this as Apache Iceberg Terraform provider v${version} because...

[1]: https://github.com/apache/terraform-provider-iceberg/tree/${rc_hash}
[2]: https://dist.apache.org/repos/dist/dev/iceberg/${id}
[3]: https://github.com/apache/terraform-provider-iceberg/releases/tag/${rc_tag}
[4]: https://github.com/apache/terraform-provider-iceberg/blob/main/docs/releasing.md#verifying-a-release
MAIL
echo "---------------------------------------------------------"
