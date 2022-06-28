#!/usr/bin/env bash
# 2021-07-08 WATERMARK, DO NOT REMOVE - This script was generated from the Kurtosis Bash script template

set -euo pipefail   # Bash "strict mode"
script_dirpath="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
kudet_module_dirpath="$(dirname "${script_dirpath}")"
root_dirpath="$(dirname "${script_dirpath}")"

# ==================================================================================================
#                                             Constants
# ==================================================================================================
DEFAULT_SHOULD_PUBLISH_ARG="false"
GORELEASER_OUTPUT_DIRNAME="dist"
GORELEASER_KUDET_BUILD_ID="kudet"
KUDET_BINARY_FILENAME="kudet"
GO_ARCH_ENV_AMD64_VALUE="amd64"
GO_DEFAULT_AMD64_ENV="v1"
GIT_REF_SANITIZING_SED_REGEX="s,[/:],_,g"
# ==================================================================================================
#                                       Arg Parsing & Validation
# ==================================================================================================
show_helptext_and_exit() {
  echo "Usage: $(basename "${0}") [should_publish_arg]"
  echo ""
  echo "  should_publish_arg  Whether the build artifacts should be published (default: ${DEFAULT_SHOULD_PUBLISH_ARG})"
  echo ""
  exit 1  # Exit with an error so that if this is accidentally called by CI, the script will fail
}

should_publish_arg="${1:-"${DEFAULT_SHOULD_PUBLISH_ARG}"}"
if [ "${should_publish_arg}" != "true" ] && [ "${should_publish_arg}" != "false" ]; then
  echo "Error: Invalid should-publish arg '${should_publish_arg}'" >&2
  show_helptext_and_exit
fi

# ==================================================================================================
#                                             Main Logic
# ==================================================================================================

# This logic is actually just `kudet get-docker-tag` but we can't use kudet to build kudet
# in every other Kurtosis repo, this logic is done using `kudet get-docker-tag`
# Captures the first of tag > commit
if ! git_ref="$(git describe --tags --exact-match 2> /dev/null || git rev-parse --short HEAD)"; then
    echo "Error: Couldn't get a Git ref to use for a Docker tag" >&2
    exit 1
fi
# Sanitize git ref to be acceptable Docker tag format
if ! version="$(echo "${git_ref}" | sed "${GIT_REF_SANITIZING_SED_REGEX}")"; then
    echo "Error: Couldn't sanitize Git ref to acceptable Docker tag format" >&2
    exit 1
fi

(
    if ! cd "${kudet_module_dirpath}"; then
        echo "Error: Couldn't cd to the kudet module directory in preparation for running Go generate & tests" >&2
        exit 1
    fi
    if ! go generate "./..."; then
        echo "Error: Go generate failed" >&2
        exit 1
    fi
    if ! CGO_ENABLED=0 go test "./..."; then
        echo "Error: Go tests failed" >&2
        exit 1
    fi
)

# vvvvvvvv Goreleaser variables vvvvvvvvvvvvvvvvvvv
export KUDET_BINARY_FILENAME \
export VERSION="${version}"
if "${should_publish_arg}"; then
#     These environment variables will be set ONLY when publishing, in the CI environment
#     See the CI config for details on how these get set
      export FURY_TOKEN="${GEMFURY_PUBLISH_TOKEN}"
      export GITHUB_TOKEN="${KURTOSISBOT_GITHUB_TOKEN}"
fi
# ^^^^^^^^ Goreleaser variables ^^^^^^^^^^^^^^^^^^^

# Build a Kudet binary (compatible with the current OS & arch) so that we can run interactive & testing locally via the launch-cli.sh script
(
    if ! cd "${kudet_module_dirpath}"; then
        echo "Error: Couldn't cd to Kudet module dirpath '${kudet_module_dirpath}'" >&2
        exit 1
    fi
    if "${should_publish_arg}"; then
        goreleaser_verb_and_flags="release --rm-dist"
    else
        goreleaser_verb_and_flags="build --rm-dist --snapshot --id ${GORELEASER_KUDET_BUILD_ID}"
    fi
    if ! goreleaser ${goreleaser_verb_and_flags}; then
        echo "Error: Couldn't build the Kudet binary for the current OS/arch" >&2
        exit 1
    fi
)

# Final verification
goarch="$(go env GOARCH)"
goos="$(go env GOOS)"
architecture_dirname="${GORELEASER_KUDET_BUILD_ID}_${goos}_${goarch}"

if [ "${goarch}" == "${GO_ARCH_ENV_AMD64_VALUE}" ]; then
  goamd64="$(go env GOAMD64)"
  if [ "${goamd64}" == "" ]; then
    goamd64="${GO_DEFAULT_AMD64_ENV}"
  fi
  architecture_dirname="${architecture_dirname}_${goamd64}"
fi

kudet_binary_filepath="${kudet_module_dirpath}/${GORELEASER_OUTPUT_DIRNAME}/${architecture_dirname}/${KUDET_BINARY_FILENAME}"
if ! [ -f "${kudet_binary_filepath}" ]; then
    echo "Error: Expected a Kudet binary to have been built by Goreleaser at '${kudet_binary_filepath}' but none exists" >&2
    exit 1
fi