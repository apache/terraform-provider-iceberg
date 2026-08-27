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
# Regenerate -- or verify -- the third-party license inventory that ships with
# the convenience binaries: the generated section of LICENSE-binary and the
# per-module license texts in licenses-binary/.
#
# The inventory is derived from the modules the provider binary actually links,
# not from go.mod, which also covers test-only and tooling dependencies that are
# never distributed. The linked set is computed for every GOOS/GOARCH pair that
# .goreleaser.yml builds and then unioned, so a dependency that only appears on,
# say, Windows is still accounted for.
#
# ASF policy is that "LICENSE and NOTICE must exactly represent the contents of
# the distribution they reside in" (https://infra.apache.org/licensing-howto.html),
# so this needs re-running whenever the dependency tree moves -- including when
# an `// indirect` dependency changes, which is where the drift usually hides.
#
#   dev/update_licenses.sh            rewrite LICENSE-binary and licenses-binary/
#   dev/update_licenses.sh --check    report drift, write nothing, exit non-zero
#
# Classifications that the tooling gets wrong are pinned in
# dev/licenses/overrides.tsv rather than patched into the output by hand.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

LICENSE_BINARY="${REPO_ROOT}/LICENSE-binary"
LICENSES_DIR="${REPO_ROOT}/licenses-binary"
OVERRIDES_FILE="${REPO_ROOT}/dev/licenses/overrides.tsv"

BEGIN_MARK='--- BEGIN GENERATED SECTION: dev/update_licenses.sh (do not edit by hand) ---'
END_MARK='--- END GENERATED SECTION ---'

GO_LICENSES_VERSION="${GO_LICENSES_VERSION:-v1.6.0}"

# Keep in sync with builds.goos / builds.goarch in .goreleaser.yml.
DEFAULT_PLATFORMS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64 freebsd/arm64"
PLATFORMS="${LICENSE_PLATFORMS:-${DEFAULT_PLATFORMS}}"

CHECK_ONLY=0
DRIFT=0

usage() {
  cat <<'EOF'
Usage: dev/update_licenses.sh [--check]

  (no flag)  Rewrite the generated section of LICENSE-binary and the license
             texts in licenses-binary/ to match the linked dependencies.
  --check    Report drift and exit 1 without modifying anything.

Environment:
  LICENSE_PLATFORMS     Space-separated GOOS/GOARCH list to analyze. Defaults to
                        the full .goreleaser.yml matrix.
  GO_LICENSES           Path to an existing go-licenses binary.
  GO_LICENSES_VERSION   Version to install if one is not already on PATH.
EOF
}

log() { printf '%s\n' "$*" >&2; }

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

drift() {
  printf '%s\n' "$*" >&2
  DRIFT=1
}

# Filename stem used under licenses-binary/, e.g. github.com/hashicorp/yamux ->
# LICENSE-hashicorp-yamux.txt. Drops the github.com host and the /vN major
# version suffix, both of which are noise, and keeps every other host.
slug_for() {
  local mod="${1#github.com/}"
  mod="$(printf '%s' "${mod}" | sed -E 's|/v[0-9]+$||')"
  printf '%s' "${mod//\//-}"
}

# Section heading used in LICENSE-binary for an SPDX identifier. An unmapped
# identifier is fatal on purpose: a license we have not seen before is a
# decision for a human, not something to guess at.
heading_for() {
  case "$1" in
    BSD-2-Clause) printf 'BSD 2-Clause' ;;
    BSD-3-Clause) printf 'BSD 3-Clause' ;;
    ISC)          printf 'ISC License' ;;
    MIT)          printf 'MIT License' ;;
    MPL-2.0)      printf 'Mozilla Public License 2.0' ;;
    *) return 1 ;;
  esac
}

ensure_go_licenses() {
  if [[ -n "${GO_LICENSES:-}" ]]; then
    [[ -x "${GO_LICENSES}" ]] || fail "GO_LICENSES=${GO_LICENSES} is not executable"
    return
  fi
  if command -v go-licenses >/dev/null 2>&1; then
    GO_LICENSES="$(command -v go-licenses)"
    return
  fi
  local gobin="${REPO_ROOT}/build/tools"
  if [[ ! -x "${gobin}/go-licenses" ]]; then
    log "installing go-licenses ${GO_LICENSES_VERSION} into build/tools"
    # GOFLAGS is cleared so a caller's -mod=vendor/-mod=readonly does not leak
    # into the install of an unrelated module.
    GOFLAGS='' GOBIN="${gobin}" go install "github.com/google/go-licenses@${GO_LICENSES_VERSION}"
  fi
  GO_LICENSES="${gobin}/go-licenses"
}

# Populates ${WORK}/attributed.tsv with one row per (module, license file):
#   module-path <TAB> version <TAB> license-path-within-module <TAB> spdx-id
collect() {
  local template="${WORK}/report.tpl"
  printf '{{range .}}{{.LicenseName}}\t{{.LicensePath}}\n{{end}}' >"${template}"

  local main_module goroot
  main_module="$(cd "${REPO_ROOT}" && go list -m)"
  # go-licenses decides whether a package is stdlib by testing its source path
  # against build.Default.GOROOT, i.e. the GOROOT of its own process. Under a
  # downloaded toolchain the stdlib sources live in the module cache instead, so
  # without this every stdlib package is reported as "does not have module info"
  # and the run fails outright (google/go-licenses#244, still unfixed in v2.0.1).
  # Resolved from the repo so that a toolchain directive in go.mod is honored.
  goroot="$(cd "${REPO_ROOT}" && go env GOROOT)"

  : >"${WORK}/modules.raw"
  : >"${WORK}/licenses.raw"

  local platform goos goarch
  for platform in ${PLATFORMS}; do
    goos="${platform%/*}"
    goarch="${platform#*/}"
    [[ "${goos}" != "${platform}" && -n "${goarch}" ]] || fail "bad platform '${platform}', want GOOS/GOARCH"
    log "  analyzing ${platform}"

    # Module directories, for attributing each license file to its module.
    # `go list -deps .` walks the import graph of the provider's main package,
    # so test-only and tool dependencies are excluded by construction.
    (cd "${REPO_ROOT}" && GOOS="${goos}" GOARCH="${goarch}" \
      go list -deps -f '{{with .Module}}{{.Dir}}{{"\t"}}{{.Path}}{{"\t"}}{{.Version}}{{end}}' .) \
      >>"${WORK}/modules.raw"

    # go-licenses writes its diagnostics to stderr and says nothing on stdout
    # when it fails, so the log is the only account of what went wrong.
    if ! (cd "${REPO_ROOT}" && GOOS="${goos}" GOARCH="${goarch}" GOROOT="${goroot}" \
      "${GO_LICENSES}" report . --template "${template}") \
      >>"${WORK}/licenses.raw" 2>>"${WORK}/go-licenses.log"; then
      cat "${WORK}/go-licenses.log" >&2
      fail "go-licenses failed for ${platform}"
    fi
  done

  # Drop stdlib, which has no module and so yields a blank line.
  grep -v '^$' "${WORK}/modules.raw" | sort -u >"${WORK}/modules.all.tsv"
  grep -v '^$' "${WORK}/licenses.raw" | sort -u >"${WORK}/licenses.tsv"

  # The provider's own module has to stay in the attribution table -- its LICENSE
  # is one of the files go-licenses reports -- but it is not a bundled component.
  grep -v -F "	${main_module}	" "${WORK}/modules.all.tsv" >"${WORK}/modules.tsv"

  [[ -s "${WORK}/modules.tsv" ]] || fail "no dependency modules found; is the module cache populated?"

  # go-licenses reports a license file per package; attribute each one to the
  # module that owns it by longest matching directory prefix.
  awk -F'\t' -v OFS='\t' '
    NR == FNR { dir[FNR] = $1; mod[FNR] = $2; ver[FNR] = $3; n = FNR; next }
    {
      best = 0; bestlen = 0
      for (i = 1; i <= n; i++) {
        l = length(dir[i])
        if (l > bestlen && substr($2, 1, l) == dir[i] && substr($2, l + 1, 1) == "/") {
          best = i; bestlen = l
        }
      }
      if (best == 0) {
        print "error: cannot attribute license file to a module: " $2 > "/dev/stderr"
        bad = 1
        next
      }
      print mod[best], ver[best], substr($2, bestlen + 2), $1
    }
    END { if (bad) exit 1 }
  ' "${WORK}/modules.all.tsv" "${WORK}/licenses.tsv" \
    | awk -F'\t' -v main="${main_module}" '$1 != main' \
    | sort -u >"${WORK}/attributed.tsv"
}

# Rewrites attributed.tsv in place, applying dev/licenses/overrides.tsv.
#
# An entry that matches nothing is an error rather than a no-op: it is either a
# typo in the key or a dependency that has since been dropped, and silently
# ignoring it would let a correction the maintainer believes is in force quietly
# stop applying.
apply_overrides() {
  [[ -f "${OVERRIDES_FILE}" ]] || return 0
  local status=0
  awk -F'\t' -v OFS='\t' -v file="${OVERRIDES_FILE}" '
    NR == FNR {
      if ($0 ~ /^[[:space:]]*(#|$)/) next
      if (NF < 2 || $1 == "" || $2 == "") {
        print "error: " file ":" FNR ": expected <key><TAB><SPDX id | SKIP>" > "/dev/stderr"
        malformed = 1
        next
      }
      override[$1] = $2
      next
    }
    {
      # Key a root license file by its module and a nested one by the directory
      # it governs, so both read like an import path.
      key = $1
      if (index($3, "/") > 0) {
        sub_dir = $3
        sub(/\/[^\/]+$/, "", sub_dir)
        key = key "/" sub_dir
      }
      if (key in override) {
        used[key] = 1
        if (override[key] == "SKIP") next
        $4 = override[key]
      }
      print
    }
    END {
      # A malformed entry never made it into the table, so it would also report
      # as unmatched. Report the cause, not the consequence.
      if (malformed) exit 2
      for (key in override) {
        if (key in used) continue
        print "error: " file ": override for \"" key "\" matches no linked dependency" > "/dev/stderr"
        unmatched = 1
      }
      if (unmatched) exit 1
    }
  ' "${OVERRIDES_FILE}" "${WORK}/attributed.tsv" >"${WORK}/attributed.override" || status=$?

  if [[ ${status} -eq 2 ]]; then
    fail "dev/licenses/overrides.tsv is malformed; each entry is
<key><TAB><SPDX id | SKIP><TAB><reason>, and the separators must be tabs."
  elif [[ ${status} -ne 0 ]]; then
    fail "every override must name a dependency that is actually linked; drop the
entry if the dependency is gone, or correct the key. A module's own license file
is keyed by the module path, a nested one by the module path joined with the
directory it governs."
  fi
  mv "${WORK}/attributed.override" "${WORK}/attributed.tsv"
}

# Splits attributed.tsv into the module-level inventory that gets generated and
# the nested license files that stay hand-curated.
partition() {
  # Module-level, non-Apache: heading <TAB> module <TAB> version <TAB> license path
  : >"${WORK}/modlevel.tsv"
  local mod ver sub spdx heading
  while IFS=$'\t' read -r mod ver sub spdx; do
    [[ "${sub}" == */* ]] && continue
    [[ "${spdx}" == "Apache-2.0" ]] && continue
    if ! heading="$(heading_for "${spdx}")"; then
      fail "unmapped license '${spdx}' for ${mod}.
Add a heading for it in heading_for() (and confirm it is even acceptable for an
ASF distribution: https://www.apache.org/legal/resolved.html), or pin a correct
classification in dev/licenses/overrides.tsv."
    fi
    printf '%s\t%s\t%s\t%s\n' "${heading}" "${mod}" "${ver}" "${sub}" >>"${WORK}/modlevel.tsv"
  done <"${WORK}/attributed.tsv"
  sort -u -o "${WORK}/modlevel.tsv" "${WORK}/modlevel.tsv"

  # A module must not end up in two license groups; that means an override is
  # missing or wrong rather than that both are true. modlevel.tsv is sorted by
  # heading, so duplicate modules are not adjacent until sorted again.
  local dupes
  dupes="$(cut -f2 "${WORK}/modlevel.tsv" | sort | uniq -d)"
  [[ -z "${dupes}" ]] || fail "modules classified under more than one license: ${dupes}"

  # Nested, non-Apache license files: module <TAB> governed dir <TAB> spdx <TAB> path
  : >"${WORK}/nested.tsv"
  while IFS=$'\t' read -r mod ver sub spdx; do
    [[ "${sub}" == */* ]] || continue
    [[ "${spdx}" == "Apache-2.0" ]] && continue
    printf '%s\t%s\t%s\t%s\n' "${mod}" "${sub%/*}" "${spdx}" "${sub}" >>"${WORK}/nested.tsv"
  done <"${WORK}/attributed.tsv"
  sort -u -o "${WORK}/nested.tsv" "${WORK}/nested.tsv"
}

# Renders the grouped module list that goes between the markers.
render_section() {
  local heading mod prev="" first=1
  while IFS=$'\t' read -r heading mod _ _; do
    if [[ "${heading}" != "${prev}" ]]; then
      [[ ${first} -eq 1 ]] || echo
      first=0
      echo "${heading}"
      printf '%*s\n' "${#heading}" '' | tr ' ' '-'
      prev="${heading}"
    fi
    echo "${mod}"
  done <"${WORK}/modlevel.tsv"
}

# Module paths listed in the current generated section, i.e. the licenses-binary/
# files this script owns today. Anything else in that directory is hand-curated
# and never touched.
previous_modules() {
  awk -v b="${BEGIN_MARK}" -v e="${END_MARK}" '$0 == b { inside = 1; next } $0 == e { inside = 0 } inside' \
    "${LICENSE_BINARY}" | grep -E '^[a-z0-9.-]+\.[a-z]+/' || true
}

# Renders the updated LICENSE-binary into the work directory, and snapshots the
# module list the current one advertises -- the set of licenses-binary/ files
# this script owns -- before that list is overwritten.
build_license_binary() {
  local begins ends
  begins="$(grep -c -F -x -- "${BEGIN_MARK}" "${LICENSE_BINARY}" || true)"
  ends="$(grep -c -F -x -- "${END_MARK}" "${LICENSE_BINARY}" || true)"
  [[ "${begins}" == "1" && "${ends}" == "1" ]] || fail \
    "LICENSE-binary must contain exactly one generated-section marker pair (found ${begins} begin, ${ends} end)"

  render_section >"${WORK}/section.txt"
  awk -v b="${BEGIN_MARK}" -v e="${END_MARK}" -v section="${WORK}/section.txt" '
    $0 == b {
      print; print ""
      while ((getline line < section) > 0) print line
      print ""
      inside = 1
      next
    }
    $0 == e { inside = 0 }
    !inside { print }
  ' "${LICENSE_BINARY}" >"${WORK}/LICENSE-binary.new"
  previous_modules >"${WORK}/previous.txt"
}

report_notices() {
  local mod dir notices=()
  while IFS=$'\t' read -r dir mod _; do
    local found
    found="$(find "${dir}" -maxdepth 1 -type f \( -name 'NOTICE' -o -name 'NOTICE.txt' -o -name 'NOTICE.md' \) 2>/dev/null | head -n 1)"
    [[ -n "${found}" ]] && notices+=("${mod}")
  done <"${WORK}/modules.tsv"

  [[ ${#notices[@]} -gt 0 ]] || return 0
  log ""
  log "Linked modules that ship a NOTICE file -- confirm NOTICE-binary still"
  log "reproduces each of them (this is advisory; it is not checked):"
  local m
  for m in "${notices[@]}"; do
    log "  ${m}"
  done
}

# Escapes a string so it can be dropped into an ERE as a literal.
escape_re() {
  printf '%s' "$1" | sed -E 's/[^a-zA-Z0-9_-]/\\&/g'
}

# Nested license files are listed by hand in the trailing section of
# LICENSE-binary, because that section records file-level provenance that no
# tool can recover. All this can do is insist that each one is mentioned.
#
# The entry may narrow the directory the license file governs to the specific
# files it actually covers -- "simplelru/list.go, in <module>" satisfies a
# license file governing "simplelru" -- since that is strictly more accurate.
check_nested() {
  local mod dir spdx path pattern missing=()
  while IFS=$'\t' read -r mod dir spdx path; do
    pattern="^$(escape_re "${dir}")[^,]*, in $(escape_re "${mod}")\$"
    if ! grep -qE -- "${pattern}" "${LICENSE_BINARY}"; then
      missing+=("${dir}, in ${mod}	${spdx}	${path}")
    fi
  done <"${WORK}/nested.tsv"

  [[ ${#missing[@]} -gt 0 ]] || return 0
  drift ""
  drift "Bundled code under a license other than its own module's, not mentioned"
  drift "in LICENSE-binary. Add each to the trailing section and put its text in"
  drift "licenses-binary/, or record it as SKIP in dev/licenses/overrides.tsv:"
  local entry
  for entry in "${missing[@]}"; do
    IFS=$'\t' read -r line spdx path <<<"${entry}"
    drift "  ${line}"
    drift "      ${spdx}, text at <module>/${path}"
  done
}

sync_texts() {
  local heading mod ver sub dir target
  local -A wanted=()

  while IFS=$'\t' read -r heading mod ver sub; do
    dir="$(cd "${REPO_ROOT}" && go list -m -f '{{.Dir}}' "${mod}")"
    [[ -n "${dir}" && -f "${dir}/${sub}" ]] || fail "license file missing for ${mod}: ${dir}/${sub}"
    target="LICENSE-$(slug_for "${mod}").txt"
    wanted["${target}"]="${dir}/${sub}"
  done <"${WORK}/modlevel.tsv"

  # The module cache is read-only, so copy with an explicit mode rather than
  # preserving 0444 and making the next run unable to overwrite its own output.
  local name src
  for name in "${!wanted[@]}"; do
    src="${wanted[${name}]}"
    if [[ ! -f "${LICENSES_DIR}/${name}" ]]; then
      if [[ ${CHECK_ONLY} -eq 1 ]]; then
        drift "missing licenses-binary/${name} (copy of ${src})"
      else
        log "  + licenses-binary/${name}"
        install -m 644 "${src}" "${LICENSES_DIR}/${name}"
      fi
    elif ! cmp -s "${src}" "${LICENSES_DIR}/${name}"; then
      if [[ ${CHECK_ONLY} -eq 1 ]]; then
        drift "licenses-binary/${name} differs from ${src}"
      else
        log "  ~ licenses-binary/${name}"
        install -m 644 "${src}" "${LICENSES_DIR}/${name}"
      fi
    fi
  done

  # Prune texts for modules that were in the generated section but are no longer
  # linked. Files this script never generated are left alone.
  local prev
  while read -r prev; do
    [[ -n "${prev}" ]] || continue
    name="LICENSE-$(slug_for "${prev}").txt"
    [[ -n "${wanted[${name}]:-}" ]] && continue
    [[ -f "${LICENSES_DIR}/${name}" ]] || continue
    if [[ ${CHECK_ONLY} -eq 1 ]]; then
      drift "licenses-binary/${name} is stale: ${prev} is no longer linked"
    else
      log "  - licenses-binary/${name} (${prev} no longer linked)"
      rm "${LICENSES_DIR}/${name}"
    fi
  done <"${WORK}/previous.txt"
}

main() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --check) CHECK_ONLY=1 ;;
      -h|--help) usage; exit 0 ;;
      *) usage >&2; fail "unknown argument '$1'" ;;
    esac
    shift
  done

  command -v go >/dev/null 2>&1 || fail "go is required"
  ensure_go_licenses

  WORK="$(mktemp -d)"
  trap 'rm -rf "${WORK}"' EXIT

  log "Resolving the dependency tree of the convenience binaries..."
  (cd "${REPO_ROOT}" && go mod download)
  collect
  apply_overrides
  partition
  build_license_binary

  if [[ ${CHECK_ONLY} -eq 1 ]]; then
    if ! diff -u "${LICENSE_BINARY}" "${WORK}/LICENSE-binary.new" >"${WORK}/diff"; then
      drift "LICENSE-binary is out of date:"
      sed 's/^/  /' "${WORK}/diff" >&2
    fi
  elif ! cmp -s "${LICENSE_BINARY}" "${WORK}/LICENSE-binary.new"; then
    log "  ~ LICENSE-binary"
    cp "${WORK}/LICENSE-binary.new" "${LICENSE_BINARY}"
  fi

  sync_texts
  check_nested
  report_notices

  if [[ ${DRIFT} -eq 1 ]]; then
    log ""
    log "Run dev/update_licenses.sh to regenerate what can be regenerated."
    exit 1
  fi

  if [[ ${CHECK_ONLY} -eq 1 ]]; then
    log ""
    log "LICENSE-binary and licenses-binary/ match the linked dependencies."
  fi
}

main "$@"
