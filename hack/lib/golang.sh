#!/usr/bin/env bash

# Copyright 2014 The ISTIOrnetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# shellcheck disable=SC2034 # Variables sourced in other scripts.

# The golang package that we are building.

readonly ISTIO_GO_PACKAGE=istio.io/istio
readonly ISTIO_GOPATH="${ISTIO_OUTPUT}/go"

# The server platform we are building on.
readonly ISTIO_SUPPORTED_SERVER_PLATFORMS=(
  linux/amd64
  linux/arm
  linux/arm64
  linux/s390x
  linux/ppc64le
)

# The node platforms we build for
readonly ISTIO_SUPPORTED_NODE_PLATFORMS=(
  linux/amd64
  linux/arm
  linux/arm64
  linux/s390x
  linux/ppc64le
)

# If we update this we should also update the set of platforms whose standard
# library is precompiled for in build/build-image/cross/Dockerfile
readonly ISTIO_SUPPORTED_CLIENT_PLATFORMS=(
  linux/amd64
  linux/386
  linux/arm
  linux/arm64
  linux/s390x
  linux/ppc64le
  darwin/amd64
  darwin/arm64
)

# Which platforms we should compile test targets for.
# Not all client platforms need these tests
readonly ISTIO_SUPPORTED_TEST_PLATFORMS=(
  linux/amd64
  linux/arm
  linux/arm64
  linux/s390x
  linux/ppc64le
  darwin/amd64
  darwin/arm64
)

# ------------
# NOTE: All functions that return lists should use newlines.
# bash functions can't return arrays, and spaces are tricky, so newline
# separators are the preferred pattern.
# To transform a string of newline-separated items to an array, use ISTIO::util::read-array:
# ISTIO::util::read-array FOO < <(ISTIO::golang::dups a b c a)
#
# ALWAYS remember to quote your subshells. Not doing so will break in
# bash 4.3, and potentially cause other issues.
# ------------

# Returns a sorted newline-separated list containing only duplicated items.
ISTIO::golang::dups() {
  # We use printf to insert newlines, which are required by sort.
  printf "%s\n" "$@" | sort | uniq -d
}

# Returns a sorted newline-separated list with duplicated items removed.
ISTIO::golang::dedup() {
  # We use printf to insert newlines, which are required by sort.
  printf "%s\n" "$@" | sort -u
}

# Depends on values of user-facing ISTIO_BUILD_PLATFORMS, ISTIO_FASTBUILD,
# and ISTIO_BUILDER_OS.
# Configures ISTIO_SERVER_PLATFORMS, ISTIO_NODE_PLATFOMRS,
# ISTIO_TEST_PLATFORMS, and ISTIO_CLIENT_PLATFORMS, then sets them
# to readonly.
# The configured vars will only contain platforms allowed by the
# ISTIO_SUPPORTED* vars at the top of this file.
declare -a ISTIO_SERVER_PLATFORMS
declare -a ISTIO_CLIENT_PLATFORMS
declare -a ISTIO_NODE_PLATFORMS
declare -a ISTIO_TEST_PLATFORMS
ISTIO::golang::setup_platforms() {
  if [[ -n "${ISTIO_BUILD_PLATFORMS:-}" ]]; then
    # ISTIO_BUILD_PLATFORMS needs to be read into an array before the next
    # step, or quoting treats it all as one element.
    local -a platforms
    IFS=" " read -ra platforms <<< "${ISTIO_BUILD_PLATFORMS}"

    # Deduplicate to ensure the intersection trick with ISTIO::golang::dups
    # is not defeated by duplicates in user input.
    ISTIO::util::read-array platforms < <(ISTIO::golang::dedup "${platforms[@]}")

    # Use ISTIO::golang::dups to restrict the builds to the platforms in
    # ISTIO_SUPPORTED_*_PLATFORMS. Items should only appear at most once in each
    # set, so if they appear twice after the merge they are in the intersection.
    ISTIO::util::read-array ISTIO_SERVER_PLATFORMS < <(ISTIO::golang::dups \
        "${platforms[@]}" \
        "${ISTIO_SUPPORTED_SERVER_PLATFORMS[@]}" \
      )
    readonly ISTIO_SERVER_PLATFORMS

    ISTIO::util::read-array ISTIO_NODE_PLATFORMS < <(ISTIO::golang::dups \
        "${platforms[@]}" \
        "${ISTIO_SUPPORTED_NODE_PLATFORMS[@]}" \
      )
    readonly ISTIO_NODE_PLATFORMS

    ISTIO::util::read-array ISTIO_TEST_PLATFORMS < <(ISTIO::golang::dups \
        "${platforms[@]}" \
        "${ISTIO_SUPPORTED_TEST_PLATFORMS[@]}" \
      )
    readonly ISTIO_TEST_PLATFORMS

    ISTIO::util::read-array ISTIO_CLIENT_PLATFORMS < <(ISTIO::golang::dups \
        "${platforms[@]}" \
        "${ISTIO_SUPPORTED_CLIENT_PLATFORMS[@]}" \
      )
    readonly ISTIO_CLIENT_PLATFORMS

  elif [[ "${ISTIO_FASTBUILD:-}" == "true" ]]; then
    host_arch=$(ISTIO::util::host_arch)
    if [[ "${host_arch}" != "amd64" && "${host_arch}" != "arm64" ]]; then
      # on any platform other than amd64 and arm64, we just default to amd64
      host_arch="amd64"
    fi
    ISTIO_SERVER_PLATFORMS=("linux/${host_arch}")
    readonly ISTIO_SERVER_PLATFORMS
    ISTIO_NODE_PLATFORMS=("linux/${host_arch}")
    readonly ISTIO_NODE_PLATFORMS
    if [[ "${ISTIO_BUILDER_OS:-}" == "darwin"* ]]; then
      ISTIO_TEST_PLATFORMS=(
        "darwin/${host_arch}"
        "linux/${host_arch}"
      )
      readonly ISTIO_TEST_PLATFORMS
      ISTIO_CLIENT_PLATFORMS=(
        "darwin/${host_arch}"
        "linux/${host_arch}"
      )
      readonly ISTIO_CLIENT_PLATFORMS
    else
      ISTIO_TEST_PLATFORMS=("linux/${host_arch}")
      readonly ISTIO_TEST_PLATFORMS
      ISTIO_CLIENT_PLATFORMS=("linux/${host_arch}")
      readonly ISTIO_CLIENT_PLATFORMS
    fi
  else
    ISTIO_SERVER_PLATFORMS=("${ISTIO_SUPPORTED_SERVER_PLATFORMS[@]}")
    readonly ISTIO_SERVER_PLATFORMS

    ISTIO_NODE_PLATFORMS=("${ISTIO_SUPPORTED_NODE_PLATFORMS[@]}")
    readonly ISTIO_NODE_PLATFORMS

    ISTIO_CLIENT_PLATFORMS=("${ISTIO_SUPPORTED_CLIENT_PLATFORMS[@]}")
    readonly ISTIO_CLIENT_PLATFORMS

    ISTIO_TEST_PLATFORMS=("${ISTIO_SUPPORTED_TEST_PLATFORMS[@]}")
    readonly ISTIO_TEST_PLATFORMS
  fi
}

ISTIO::golang::setup_platforms

# Create the GOPATH tree under $ISTIO_OUTPUT
ISTIO::golang::create_gopath_tree() {
  local go_pkg_dir="${ISTIO_GOPATH}/src/${ISTIO_GO_PACKAGE}"
  local go_pkg_basedir
  go_pkg_basedir=$(dirname "${go_pkg_dir}")

  mkdir -p "${go_pkg_basedir}"

  # TODO: This symlink should be relative.
  if [[ ! -e "${go_pkg_dir}" || "$(readlink "${go_pkg_dir}")" != "${ISTIO_ROOT}" ]]; then
    ln -snf "${ISTIO_ROOT}" "${go_pkg_dir}"
  fi
}

# Ensure the go tool exists and is a viable version.
ISTIO::golang::verify_go_version() {
  if [[ -z "$(command -v go)" ]]; then
    ISTIO::log::usage_from_stdin <<EOF
Can't find 'go' in PATH, please fix and retry.
See http://golang.org/doc/install for installation instructions.
EOF
    return 2
  fi

  local go_version
  IFS=" " read -ra go_version <<< "$(GOFLAGS='' go version)"
  local minimum_go_version
  minimum_go_version=go1.16.0
  if [[ "${minimum_go_version}" != $(echo -e "${minimum_go_version}\n${go_version[2]}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1) && "${go_version[2]}" != "devel" ]]; then
    ISTIO::log::usage_from_stdin <<EOF
Detected go version: ${go_version[*]}.
ISTIOrnetes requires ${minimum_go_version} or greater.
Please install ${minimum_go_version} or later.
EOF
    return 2
  fi
}

# ISTIO::golang::setup_env will check that the `go` commands is available in
# ${PATH}. It will also check that the Go version is good enough for the
# ISTIOrnetes build.
#
# Inputs:
#   ISTIO_EXTRA_GOPATH - If set, this is included in created GOPATH
#
# Outputs:
#   env-var GOPATH points to our local output dir
#   env-var GOBIN is unset (we want binaries in a predictable place)
#   env-var GO15VENDOREXPERIMENT=1
#   current directory is within GOPATH
ISTIO::golang::setup_env() {
  ISTIO::golang::verify_go_version

  ISTIO::golang::create_gopath_tree

  export GOPATH="${ISTIO_GOPATH}"
  export GOCACHE="${ISTIO_GOPATH}/cache"

  # Append ISTIO_EXTRA_GOPATH to the GOPATH if it is defined.
  if [[ -n ${ISTIO_EXTRA_GOPATH:-} ]]; then
    GOPATH="${GOPATH}:${ISTIO_EXTRA_GOPATH}"
  fi

  # Make sure our own Go binaries are in PATH.
  export PATH="${ISTIO_GOPATH}/bin:${PATH}"

  # Change directories so that we are within the GOPATH.  Some tools get really
  # upset if this is not true.  We use a whole fake GOPATH here to collect the
  # resultant binaries.  Go will not let us use GOBIN with `go install` and
  # cross-compiling, and `go install -o <file>` only works for a single pkg.
  local subdir
  subdir=$(ISTIO::realpath . | sed "s|${ISTIO_ROOT}||")
  cd "${ISTIO_GOPATH}/src/${ISTIO_GO_PACKAGE}/${subdir}" || return 1

  # Set GOROOT so binaries that parse code can work properly.
  GOROOT=$(go env GOROOT)
  export GOROOT

  # Unset GOBIN in case it already exists in the current session.
  unset GOBIN

  # This seems to matter to some tools
  export GO15VENDOREXPERIMENT=1
}