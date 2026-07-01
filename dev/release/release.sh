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
# Finalizes a release after the vote has passed. It promotes the exact artifacts
# that were voted on -- nothing is rebuilt:
#
#   1. Tags v<version> at the RC commit and pushes it.
#   2. Moves the source release from the Apache dev SVN to the release SVN.
#   3. Creates the final (non-prerelease) GitHub release, attaching the source
#      release and the already-signed convenience binaries from the RC
#      prerelease. The Terraform/OpenTofu registries ingest this release.
#   4. Removes superseded releases from the Apache release SVN.
#
# Usage: dev/release/release.sh <version> <rc>
#  e.g.: dev/release/release.sh 0.7.0 1
#
# <rc> is the number of the release candidate that passed the vote.

set -eu

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <version> <rc>"
  echo " e.g.: $0 0.7.0 1"
  exit 1
fi

version=$1
rc=$2

repository="apache/terraform-provider-iceberg"

git_origin_url="$(git remote get-url origin)"
if [ "${git_origin_url}" != "git@github.com:apache/terraform-provider-iceberg.git" ]; then
  echo "This script must be run with a working copy of apache/terraform-provider-iceberg."
  echo "The origin's URL: ${git_origin_url}"
  exit 1
fi

tag="v${version}"
rc_tag="${tag}-rc${rc}"

echo "Tagging for release: ${tag}"
git tag "${tag}" "${rc_tag}^{}" -m "Apache Iceberg Terraform provider ${version}"
git push origin "${tag}"

release_id="apache-iceberg-terraform-${version}"
dist_dev_url="https://dist.apache.org/repos/dist/dev/iceberg"
dist_url="https://dist.apache.org/repos/dist/release/iceberg"

echo "Promoting the source release from dist/dev to dist/release"
svn mv \
  "${dist_dev_url}/${release_id}-rc${rc}" \
  "${dist_url}/${release_id}" \
  -m "Apache Iceberg Terraform provider ${version}"

# Assemble the final release assets: the promoted source release plus the
# already-signed convenience binaries from the RC prerelease.
rm -rf "${release_id}"
svn export "${dist_url}/${release_id}" "${release_id}"
echo "Downloading the voted convenience binaries from the RC prerelease"
gh release download "${rc_tag}" \
  --dir "${release_id}" \
  --repo "${repository}" \
  --pattern 'terraform-provider-iceberg_*' \
  --skip-existing

pushd "${release_id}"
# shellcheck disable=SC2046
gh release create "${tag}" \
  --repo "${repository}" \
  --title "Apache Iceberg Terraform provider ${version}" \
  --generate-notes \
  --verify-tag \
  $(ls)
popd

rm -rf "${release_id}"

echo "Keeping only the latest release in dist/release"
old_releases=$(
  svn ls "${dist_url}" |
  grep -E '^apache-iceberg-terraform-' |
  sort --version-sort --reverse |
  tail -n +2
)
for old_release in ${old_releases}; do
  echo "Removing old release ${old_release}"
  svn delete \
    -m "Remove old Apache Iceberg Terraform provider release: ${old_release}" \
    "${dist_url}/${old_release}"
done

echo
echo "Success! The release is available here:"
echo "  ${dist_url}/${release_id}"
echo "  https://github.com/${repository}/releases/tag/${tag}"
echo
echo "Add this release to ASF's report database:"
echo "  https://reporter.apache.org/addrelease.html?iceberg"
