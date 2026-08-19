#!/bin/sh
set -euo pipefail

BUILDKIT_NIX_STORE_CACHE_KEY=${BUILDKIT_NIX_STORE_CACHE_KEY:-}
BUILDKIT_NIX_USER_CONFIGS=${BUILDKIT_NIX_USER_CONFIGS:-}

nix-config-get() {
	nix --extra-experimental-features nix-command config show | grep "$1" | cut -d"=" -f2 | xargs
}

nix --version
{
	echo "binary-caches-parallel-connections = 15"
	echo "build-users-group = $(nix-config-get build-users-group)"
	echo "extra-experimental-features = configurable-impure-env"
	echo "extra-experimental-features = nix-command flakes"
	echo "filter-syscalls = false"
	echo "sandbox = relaxed"
	echo "substituters = $(nix-config-get substituters)"
	echo "trusted-public-keys = $(nix-config-get trusted-public-keys)"
	echo "trusted-users = $(whoami)"
	if [ -n "$BUILDKIT_NIX_USER_CONFIGS" ]; then echo "$BUILDKIT_NIX_USER_CONFIGS"; fi
} | tee /etc/nix/nix.conf | sort

# This is a fake config for debugging purposes,
# it will be printed in the build logs, but it will not be used by nix
echo "nix-store-cache-key = ${BUILDKIT_NIX_STORE_CACHE_KEY}"
echo -ne "\n"

# Check the nix config for errors
nix config check
