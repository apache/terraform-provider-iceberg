<!--
  - Licensed to the Apache Software Foundation (ASF) under one
  - or more contributor license agreements.  See the NOTICE file
  - distributed with this work for additional information
  - regarding copyright ownership.  The ASF licenses this file
  - to you under the Apache License, Version 2.0 (the
  - "License"); you may not use this file except in compliance
  - with the License.  You may obtain a copy of the License at
  -
  -   http://www.apache.org/licenses/LICENSE-2.0
  -
  - Unless required by applicable law or agreed to in writing,
  - software distributed under the License is distributed on an
  - "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
  - KIND, either express or implied.  See the License for the
  - specific language governing permissions and limitations
  - under the License.
  -->

# How to Release

This guide outlines the process for releasing the Apache Iceberg Terraform
provider in accordance with the [Apache Release Process](https://infra.apache.org/release-publishing.html).
The steps include:

1. Preparing for a release
2. Publishing a Release Candidate (RC)
3. Community Voting and Validation
4. Publishing the Final Release (if the vote passes)
5. Post-Release Steps

## Requirements

* A GPG key must be registered and published in the
  [Apache Iceberg KEYS file](https://downloads.apache.org/iceberg/KEYS). Follow
  [the instructions for setting up a GPG key and uploading it to the KEYS file](#set-up-gpg-key-and-upload-to-apache-iceberg-keys-file).
    * Permission to update the `KEYS` artifact in the
      [Apache release distribution](https://dist.apache.org/repos/dist/release/iceberg/)
      (requires Iceberg PMC privileges).
* SVN Access
    * Permission to upload artifacts to the
      [Apache development distribution](https://dist.apache.org/repos/dist/dev/iceberg/)
      (requires Apache Committer access).
    * Permission to upload artifacts to the
      [Apache release distribution](https://dist.apache.org/repos/dist/release/iceberg/)
      (requires Apache PMC access).
* Terraform / OpenTofu Registry Access
    * The [`apache` namespace on the Terraform Registry](https://registry.terraform.io/providers/apache)
      must have the `terraform-provider-iceberg` provider claimed, with the
      release manager's GPG public key (the same key in `KEYS`) registered under
      **Settings → GPG Keys**. The OpenTofu Registry mirrors GitHub Releases for
      the same namespace and requires no separate upload.
* Tooling installed locally
    * `git`, `svn`, `gpg`, and `shasum`/`sha512sum`.
    * [`gh`](https://cli.github.com/) (GitHub CLI), used to watch the build and
      create the final GitHub Release.
    * [`goreleaser`](https://goreleaser.com/install/) (v2), only needed if you
      want to reproduce the convenience binaries locally; the GitHub Action
      builds them for you.

## Preparing for a Release

## Publishing a Release Candidate (RC)

### Versioning

The **git tag** carries the version. There is no version string to bump in a
source file (unlike `pyproject.toml` for PyIceberg) — the registry and GoReleaser
both derive the version from the tag. You choose the version when you create the
tag, below.

Versions follow [semantic versioning](https://semver.org/) with a `v` prefix
(`v0.7.0`). Release candidates add a `-rcN` suffix (`v0.7.0-rc1`).


### Release Types

#### Major/Minor Release

* Use the `main` branch for the release.
* Includes new features, enhancements, and any necessary backward-compatible
  changes.
* Examples: `v0.8.0`, `v0.9.0`, `v1.0.0`.

#### Patch Release

* Use the branch corresponding to the patch version, such as
  `iceberg-terraform-0.8.x`.
* Focuses on critical bug fixes or security patches that maintain backward
  compatibility.
* Examples: `v0.8.1`, `v0.8.2`.

To create a patch branch from the latest release tag:

```bash
# Fetch all tags
git fetch --tags

# Assuming v0.8.0 is the latest release tag
git checkout -b iceberg-terraform-0.8.x v0.8.0

# Cherry-pick commits for the upcoming patch release
git cherry-pick <commit>

# Push the new branch
git push git@github.com:apache/terraform-provider-iceberg.git iceberg-terraform-0.8.x
```

### Create Tag

Ensure you are on the correct branch:

* For a major/minor release, use the `main` branch.
* For a patch release, use the branch corresponding to the patch version, i.e.
  `iceberg-terraform-0.6.x`.

Set the release variables. Replace `VERSION` and `RC` with the appropriate
values for the release.

```bash
export VERSION=0.7.0
export RC=1

export VERSION_WITH_RC=${VERSION}-rc${RC}
export GIT_TAG=v${VERSION_WITH_RC}        # e.g. v0.7.0-rc1

git tag -s ${GIT_TAG} -m "Apache Iceberg Terraform provider ${VERSION_WITH_RC}"
git push git@github.com:apache/terraform-provider-iceberg.git ${GIT_TAG}
```

### Create Artifacts

The [`Terraform Build Release Candidate`](../../.github/workflows/tf-release.yml)
GitHub Action runs automatically when the tag is pushed.

It produces two artifacts:

* `svn-release-candidate-${VERSION_WITH_RC}` — the source tarball
  (`apache-iceberg-terraform-${VERSION_WITH_RC}.tar.gz`), built with
  `git archive` from the tagged source tree. **This is what the PMC votes on.**
* `registry-release-candidate-${VERSION_WITH_RC}` — the convenience binaries
  built by GoReleaser: a `.zip` per OS/arch, the
  `terraform-provider-iceberg_<version>_SHA256SUMS` checksum file, and the
  `terraform-provider-iceberg_<version>_manifest.json` registry manifest. Only
  64-bit targets (`amd64`, `arm64`) are built for linux/darwin/windows/freebsd —
  a dependency (`github.com/apache/thrift`, via `iceberg-go`) does not compile on
  32-bit. See [`.goreleaser.yml`](../../.goreleaser.yml).

CI neither signs nor publishes either artifact — you do that locally below.

Watch the build with `gh`:

```bash
: "${GIT_TAG:?ERROR: GIT_TAG is not set or is empty}"

RUN_ID=$(gh run list --repo apache/terraform-provider-iceberg \
  --workflow "Terraform Build Release Candidate" --branch "${GIT_TAG}" \
  --event push --json databaseId -q '.[0].databaseId')
: "${RUN_ID:?ERROR: RUN_ID could not be determined}"

echo "Waiting for workflow to complete, this will take several minutes..."
gh run watch $RUN_ID --repo apache/terraform-provider-iceberg
```

Download both artifacts:

```bash
: "${RUN_ID:?ERROR: RUN_ID is not set or is empty}"

gh run download $RUN_ID --repo apache/terraform-provider-iceberg
```

### Publish Release Candidate (RC)

#### Sign and Generate Checksums

For every artifact, generate:

* a `.asc` file: a detached, armored GPG signature (proves authenticity).
* a `.sha512` file: a SHA-512 checksum (verifies integrity).

The registry also needs a detached signature over the `SHA256SUMS` file. It
verifies that one signature and trusts the checksums listed inside.

```bash
: "${VERSION_WITH_RC:?ERROR: VERSION_WITH_RC is not set or is empty}"

# Source tarball (for the vote).
(
    cd svn-release-candidate-${VERSION_WITH_RC}
    for name in apache-iceberg-terraform-*.tar.gz; do
        gpg --yes --armor --output "${name}.asc" --detach-sig "${name}"
        shasum -a 512 "${name}" > "${name}.sha512"
    done
)
```

The parentheses run each block in a subshell, so `cd` does not affect your
current directory.

#### Upload Artifacts to Apache Dev SVN

Stage both directories under a single versioned RC folder in the dev SVN:

```bash
: "${VERSION_WITH_RC:?ERROR: VERSION_WITH_RC is not set or is empty}"

SVN_DEV="https://dist.apache.org/repos/dist/dev/iceberg/iceberg-terraform-${VERSION_WITH_RC}"

svn mkdir "${SVN_DEV}" -m "Iceberg Terraform: stage ${VERSION_WITH_RC}"
svn import "svn-release-candidate-${VERSION_WITH_RC}"      "${SVN_DEV}/source" \
  -m "Iceberg Terraform ${VERSION_WITH_RC}: source"
svn import "registry-release-candidate-${VERSION_WITH_RC}" "${SVN_DEV}/binaries" \
  -m "Iceberg Terraform ${VERSION_WITH_RC}: convenience binaries"
```

Verify the artifacts are uploaded to
[https://dist.apache.org/repos/dist/dev/iceberg](https://dist.apache.org/repos/dist/dev/iceberg/).

#### Remove Old Artifacts From Apache Dev SVN

Clean up superseded RC artifacts:

```bash
svn delete https://dist.apache.org/repos/dist/dev/iceberg/iceberg-terraform-<OLD_RC_VERSION> \
  -m "Remove old RC artifacts"
```

## Vote

### Generate Vote Email

Generate the email to the dev mailing list:

```bash
: "${GIT_TAG:?ERROR: GIT_TAG is not set or is empty}"
: "${VERSION:?ERROR: VERSION is not set or is empty}"
: "${VERSION_WITH_RC:?ERROR: VERSION_WITH_RC is not set or is empty}"

export GIT_TAG_REF=$(git show-ref ${GIT_TAG})
export GIT_TAG_HASH=${GIT_TAG_REF:0:40}
export LAST_COMMIT_ID=$(git rev-list ${GIT_TAG} 2> /dev/null | head -n 1)

cat << EOF > release-announcement-email.txt
To: dev@iceberg.apache.org
Subject: [VOTE] Apache Iceberg Terraform provider $VERSION_WITH_RC
Hi Everyone,

I propose that we release the following RC as the official Apache Iceberg
Terraform provider $VERSION release.

A summary of the high level features:

* <Add summary by hand>

The commit ID is $LAST_COMMIT_ID

* This corresponds to the tag: $GIT_TAG ($GIT_TAG_HASH)
* https://github.com/apache/terraform-provider-iceberg/releases/tag/$GIT_TAG
* https://github.com/apache/terraform-provider-iceberg/tree/$LAST_COMMIT_ID

The release tarball, signature, and checksums are here:

* https://dist.apache.org/repos/dist/dev/iceberg/iceberg-terraform-$VERSION_WITH_RC/

You can find the KEYS file here:

* https://downloads.apache.org/iceberg/KEYS

Convenience binaries (zips, SHA256SUMS, signature, and registry manifest) are
staged alongside the source:

* https://dist.apache.org/repos/dist/dev/iceberg/iceberg-terraform-$VERSION_WITH_RC/binaries/

Instructions for verifying a release can be found here:

* https://github.com/apache/terraform-provider-iceberg/blob/main/dev/release/README.md#verifying-a-release

Please download, verify, and test.

Please vote in the next 72 hours.
[ ] +1 Release this as Apache Iceberg Terraform provider $VERSION
[ ] +0
[ ] -1 Do not release this because...
EOF
```

### Send Vote Email

Verify the content of `release-announcement-email.txt` and send it to
`dev@iceberg.apache.org` with the corresponding subject line.

## Vote has failed

If there are concerns with the RC, address the issues and generate another RC
(increment `RC` and repeat from [Create Tag](#create-tag)).

## Publish the Final Release (Vote has passed)

An RC passes with at least 3 binding +1 votes. Once it passes, close the vote
thread:

```text
Thanks everyone for voting! The 72 hours have passed, and a minimum of 3 binding
votes have been cast:

+1 Foo Bar (non-binding)
...
+1 Fokko Driesprong (binding)

The release candidate has been accepted as Apache Iceberg Terraform provider
<VERSION>. Thanks everyone; when all artifacts are published the announcement
will be sent out.

Kind regards,
```

### Create the Final Tag

The registries publish from a final (non-pre-release) tag. Create a signed
`vVERSION` tag on the commit that was voted on:

```bash
: "${VERSION:?ERROR: VERSION is not set or is empty}"
: "${GIT_TAG:?ERROR: GIT_TAG is not set or is empty}"   # the RC tag, e.g. v0.7.0-rc1

export RELEASE_TAG=v${VERSION}
export RELEASE_COMMIT=$(git rev-list -n 1 ${GIT_TAG})

git tag -s ${RELEASE_TAG} ${RELEASE_COMMIT} -m "Apache Iceberg Terraform provider ${VERSION}"
git push git@github.com:apache/terraform-provider-iceberg.git ${RELEASE_TAG}
```

### Move the Accepted RC to the Apache Release SVN

> **Note:** Only a PMC member has permission to upload an artifact to the SVN
> release dist.

Promote the **source** artifact from `dev` to `release`. Only the source release
lives in the Apache release SVN; the binaries are convenience artifacts hosted on
the GitHub Release and registries.

```bash
: "${VERSION_WITH_RC:?ERROR: VERSION_WITH_RC is not set or is empty}"
: "${VERSION:?ERROR: VERSION is not set or is empty}"

export SVN_DEV_SOURCE="https://dist.apache.org/repos/dist/dev/iceberg/iceberg-terraform-${VERSION_WITH_RC}/source"
export SVN_RELEASE_VERSIONED="https://dist.apache.org/repos/dist/release/iceberg/iceberg-terraform-${VERSION}"

svn mv ${SVN_DEV_SOURCE} ${SVN_RELEASE_VERSIONED} \
  -m "Iceberg Terraform: add release ${VERSION}"
```

Verify the artifact is uploaded to
[https://dist.apache.org/repos/dist/release/iceberg](https://dist.apache.org/repos/dist/release/iceberg/).

### Remove Old Artifacts From Apache Release SVN

Only the latest release is hosted. Clean up old release artifacts:

```bash
svn delete https://dist.apache.org/repos/dist/release/iceberg/iceberg-terraform-<OLD_RELEASE_VERSION> \
  -m "Remove old release artifacts"
```

Also remove the now-empty RC folder from the dev SVN:

```bash
svn delete https://dist.apache.org/repos/dist/dev/iceberg/iceberg-terraform-${VERSION_WITH_RC} \
  -m "Remove RC staging for ${VERSION_WITH_RC}"
```

### Publish the Convenience Binaries

Both registries ingest a GitHub Release. Publish the **already-voted,
already-signed** binaries from the RC — do not rebuild them, so what users install
is bit-for-bit what was voted on.

Using the `registry-release-candidate-${VERSION_WITH_RC}` directory you signed
during the RC, create the GitHub Release on the final tag and attach the files:

```bash
: "${VERSION:?ERROR: VERSION is not set or is empty}"
: "${VERSION_WITH_RC:?ERROR: VERSION_WITH_RC is not set or is empty}"

export RELEASE_TAG=v${VERSION}

gh release create ${RELEASE_TAG} \
  --repo apache/terraform-provider-iceberg \
  --title "${RELEASE_TAG}" \
  --notes "Apache Iceberg Terraform provider ${VERSION}." \
  registry-release-candidate-${VERSION_WITH_RC}/*.zip \
  registry-release-candidate-${VERSION_WITH_RC}/*_SHA256SUMS \
  registry-release-candidate-${VERSION_WITH_RC}/*_SHA256SUMS.sig \
  registry-release-candidate-${VERSION_WITH_RC}/*_manifest.json
```

> **Note:** The Terraform Registry requires exactly these files attached to the
> release: one `..._<os>_<arch>.zip` per platform, one `..._SHA256SUMS`, one
> `..._SHA256SUMS.sig` signed by a GPG key registered in the registry, and one
> `..._manifest.json`. The signing key must be the same one published in the
> Apache `KEYS` file.

Within a few minutes the
[Terraform Registry](https://registry.terraform.io/providers/apache/iceberg) and
[OpenTofu Registry](https://search.opentofu.org/provider/apache/iceberg) detect
the release and publish the new version. Verify it appears, then run
`terraform init` against a configuration pinning the new version to confirm it
installs.

## Post Release

### Send out Release Announcement Email

Send an announcement to the dev mailing list:

```text
To: dev@iceberg.apache.org
Subject: [ANNOUNCE] Apache Iceberg Terraform provider <VERSION>

I'm pleased to announce the release of the Apache Iceberg Terraform provider
<VERSION>!

Apache Iceberg is an open table format for huge analytic datasets. This Terraform
(and OpenTofu) provider lets you manage Iceberg resources -- such as namespaces
and tables -- as infrastructure as code.

The provider can be used from the Terraform Registry:
https://registry.terraform.io/providers/apache/iceberg/<VERSION>

and the OpenTofu Registry:
https://search.opentofu.org/provider/apache/iceberg/<VERSION>

Thanks to everyone for contributing!
```

### Create a GitHub Release Note

The `gh release create` step above already created the GitHub Release. Open it
in the browser, use **Generate release notes** (with the previous release tag as
the **Previous tag**) to populate the changelog, and set it as the latest
release. Check the `changelog` label on GitHub for anything worth highlighting.

### Update the GitHub Issue Template

Create a PR to add the new version to the
[GitHub issue template](https://github.com/apache/terraform-provider-iceberg/tree/main/.github/ISSUE_TEMPLATE)
version dropdown (if present).

## Verifying a Release

Reviewers verifying an RC should:

```bash
export VERSION_WITH_RC=0.7.0-rc1
BASE=https://dist.apache.org/repos/dist/dev/iceberg/iceberg-terraform-${VERSION_WITH_RC}

# 1. Import the Apache Iceberg signing keys.
curl https://downloads.apache.org/iceberg/KEYS | gpg --import

# 2. Download the source tarball and its signature + checksum.
svn export ${BASE}/source iceberg-terraform-${VERSION_WITH_RC}
cd iceberg-terraform-${VERSION_WITH_RC}

# 3. Verify the signature and checksum.
gpg --verify apache-iceberg-terraform-${VERSION_WITH_RC}.tar.gz.asc
shasum -a 512 -c apache-iceberg-terraform-${VERSION_WITH_RC}.tar.gz.sha512

# 4. Unpack, build, and test from source.
tar xzf apache-iceberg-terraform-${VERSION_WITH_RC}.tar.gz
cd apache-iceberg-terraform-${VERSION_WITH_RC}
go build ./...
go test ./...
```

A valid signature from a key in the `KEYS` file, a matching checksum, and a clean
build and test is a `+1`.

## Misc

### Set up GPG key and Upload to Apache Iceberg KEYS file

To set up a GPG key locally, see the
[instructions](http://www.apache.org/dev/openpgp.html#key-gen-generate-key).

Then publish your GPG key to the
[Apache Iceberg KEYS file](https://downloads.apache.org/iceberg/KEYS):

```bash
svn co https://dist.apache.org/repos/dist/release/iceberg icebergsvn
cd icebergsvn
echo "" >> KEYS # append a newline
gpg --list-sigs <YOUR KEY ID HERE> >> KEYS # append signatures
gpg --armor --export <YOUR KEY ID HERE> >> KEYS # append public key block
svn commit -m "add key for <YOUR NAME HERE>" # this requires Iceberg PMC privileges
```

Register the **same** key under your account on the Terraform Registry
(**Settings → GPG Keys**) so the registry can verify the `SHA256SUMS.sig` you
attach to GitHub Releases.

> **Note:** Updating the `KEYS` artifact in the `release/` distribution requires
> Iceberg PMC privileges. Please work with a PMC member to update the file.
