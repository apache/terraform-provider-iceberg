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

Before cutting a release candidate:

* **Remove deprecated attributes and resources** scheduled for removal in this
  version — schema attributes, resources, and data sources carrying a
  `DeprecationMessage`, and Go symbols marked `// Deprecated:` — and update the
  `docs/` and `examples/` that reference them.
* **Confirm the provider address** in [`main.go`](../main.go) is
  `registry.terraform.io/apache/iceberg`.

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

### Cut the Release Candidate

From a clean checkout on the correct branch (`main` for a major/minor release,
`iceberg-terraform-X.Y.x` for a patch), run:

```bash
./dev/release/release_rc.sh <version> <rc>
# e.g. ./dev/release/release_rc.sh 0.7.0 1
```

The script:

1. Tags `v<version>-rc<rc>` and pushes it, which triggers the
   [`Terraform Build Release Candidate`](../.github/workflows/tf-release.yml)
   workflow. The workflow builds the source tarball and the convenience binaries
   and publishes them as a GitHub **prerelease**.
2. Waits for the workflow, downloads the artifacts, and signs them with your
   KEYS-registered GPG key — a detached `.asc` over the source tarball and a
   detached `.sig` over the binary `SHA256SUMS` — then uploads the signatures
   back onto the prerelease.
3. Uploads the signed **source** release to the Apache dev SVN at
   `dev/iceberg/apache-iceberg-terraform-<version>-rc<rc>/`. Only the source is
   voted on and archived in SVN; the convenience binaries stay on the GitHub
   prerelease.
4. Prints the `[VOTE]` email.

Set `RELEASE_PUSH_TAG=0`, `RELEASE_SIGN=0`, or `RELEASE_UPLOAD=0` (or
`RELEASE_DEFAULT=0`) to dry-run individual stages — for example, to re-run
signing without re-tagging.

> **Note:** Only 64-bit targets (`amd64`, `arm64`) are built for
> linux/darwin/windows/freebsd. A dependency (`github.com/apache/thrift`, via
> `iceberg-go`) does not compile on 32-bit. See
> [`.goreleaser.yml`](../.goreleaser.yml).

## Vote

Review the `[VOTE]` email the script printed, add a summary of the high-level
features, and send it to `dev@iceberg.apache.org`. The vote runs for at least
72 hours and needs at least 3 binding +1 votes to pass.

### Vote has failed

If there are concerns with the RC, address them and cut another candidate by
re-running the script with the next RC number:

```bash
./dev/release/release_rc.sh <version> <next-rc>
```

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

Then finalize the release. From a clean checkout, run the script with the
version and the RC number that passed the vote:

```bash
./dev/release/release.sh <version> <rc>
# e.g. ./dev/release/release.sh 0.7.0 1
```

The script:

1. Tags `v<version>` at the RC commit and pushes it.
2. Moves the **source** release from the dev SVN to the release SVN at
   `release/iceberg/apache-iceberg-terraform-<version>/`, and removes the now
   superseded RC folder from the dev SVN.
3. Creates the final (non-prerelease) GitHub release, attaching the promoted
   source release and the **already-signed** convenience binaries from the RC
   prerelease. Nothing is rebuilt, so what users install is bit-for-bit what was
   voted on. The Terraform and OpenTofu registries ingest this release.
4. Removes superseded releases from the release SVN, keeping only the latest.

> **Note:** Moving artifacts into the release SVN requires Iceberg PMC
> privileges. Work with a PMC member if you do not have them.

The final GitHub release carries exactly the files the registry requires: one
`terraform-provider-iceberg_<version>_<os>_<arch>.zip` per platform, one
`..._SHA256SUMS`, one `..._SHA256SUMS.sig` signed by the KEYS-registered key,
and one `..._manifest.json`.

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

`release.sh` already created the GitHub release with auto-generated notes. Open
it in the browser, refine the notes if needed (the **Generate release notes**
button, with the previous release tag as the **Previous tag**), and confirm it is
marked as the latest release. Check the `changelog` label on GitHub for anything
worth highlighting.

### Update the GitHub Issue Template

Create a PR to add the new version to the
[GitHub issue template](https://github.com/apache/terraform-provider-iceberg/tree/main/.github/ISSUE_TEMPLATE)
version dropdown (if present).

## Verifying a Release

Reviewers validate a candidate with:

```bash
./dev/release/verify_rc.sh <version> <rc>
# e.g. ./dev/release/verify_rc.sh 0.7.0 1
```

The script imports the Apache Iceberg `KEYS`, downloads the source release from
the dev SVN, verifies its GPG signature and SHA-512 checksum, runs the Apache
RAT license check, verifies the convenience binaries against their signed
`SHA256SUMS`, and builds and tests the provider from source in a throwaway
sandbox.

It expects `git`, `gpg`, `curl`, and `shasum`/`sha512sum` on your `PATH`.

The RAT License check needs `java` and `unzip`, but can be skipped with `VERIFY_RAT=0`.
Binary verification needs [`gh`](https://cli.github.com/) and can be skipped with `VERIFY_BINARY=0`.

A valid signature from a key in the `KEYS` file, a matching checksum, a clean RAT
check, verified binaries, and a clean build and test is a `+1`.

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
