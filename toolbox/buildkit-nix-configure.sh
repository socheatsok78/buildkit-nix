#!/bin/sh
set -euo pipefail

BUILDKIT_NIX_OPTION_SUBSTITUTERS=${BUILDKIT_NIX_OPTION_SUBSTITUTERS:-}
BUILDKIT_NIX_OPTION_TRUSTED_PUBLIC_KEYS=${BUILDKIT_NIX_OPTION_TRUSTED_PUBLIC_KEYS:-}
BUILDKIT_NIX_OPTION_TRUSTED_SUBSTITUTERS=${BUILDKIT_NIX_OPTION_TRUSTED_SUBSTITUTERS:-}
BUILDKIT_NIX_STORE_CACHE_KEY=${BUILDKIT_NIX_STORE_CACHE_KEY:-}

nix-config-get() {
    nix --extra-experimental-features nix-command config show | grep "$1" | cut -d"=" -f2 | xargs
}

echo "Setup build environment"
nix --version
{
    echo "binary-caches-parallel-connections = 15"
    echo "build-users-group = $(nix-config-get build-users-group)"
    echo "extra-experimental-features = configurable-impure-env"
    echo "extra-experimental-features = nix-command flakes"
    echo "filter-syscalls = false"
    echo "sandbox = relaxed"
    if [ -n "${BUILDKIT_NIX_OPTION_SUBSTITUTERS:-}" ]; then
        echo "substituters = ${BUILDKIT_NIX_OPTION_SUBSTITUTERS} $(nix-config-get substituters)"
    else
        echo "substituters = $(nix-config-get substituters)"
    fi
    if [ -n "${BUILDKIT_NIX_OPTION_TRUSTED_PUBLIC_KEYS:-}" ]; then
        echo "trusted-public-keys = ${BUILDKIT_NIX_OPTION_TRUSTED_PUBLIC_KEYS} $(nix-config-get trusted-public-keys)"
    else
        echo "trusted-public-keys = $(nix-config-get trusted-public-keys)"
    fi
    if [ -n "${BUILDKIT_NIX_OPTION_TRUSTED_SUBSTITUTERS:-}" ]; then
        echo "trusted-substituters = ${BUILDKIT_NIX_OPTION_TRUSTED_SUBSTITUTERS}"
    fi
    echo "trusted-users = $(whoami)"
    echo "post-build-hook = /etc/nix/buildkit-nix-post-build-hook.sh"
} | tee /etc/nix/nix.conf

# This is a fake config for debugging purposes,
# it will be printed in the build logs, but it will not be used by nix
echo "nix-store-cache-key = ${BUILDKIT_NIX_STORE_CACHE_KEY}"
echo ""
exit 0
