#!/bin/sh
set -uo pipefail

BUILDKIT_NIX_BUILD_SHELTER=${BUILDKIT_NIX_BUILD_SHELTER:-/shelter}
BUILDKIT_NIX_BUILD_TARGET=${BUILDKIT_NIX_BUILD_TARGET:-default}

installable="${1:-${BUILDKIT_NIX_BUILD_TARGET}}"
nixopts=()

# If GITHUB_TOKEN is not empty, then add it to the nix options as a secret
if [ -n "${GITHUB_TOKEN:-}" ]; then
    echo "- Detected GITHUB_TOKEN secret, adding to nix options"
    nixopts+=("--option" "access-tokens" "github.com=${GITHUB_TOKEN}")
fi

# If there are any secrets in /run/secrets, then add them to nix options,
# the secret name is the nix option name and the secret value is the nix option value
for f in /run/secrets/*; do
    if [ -f "$f" ]; then
        echo "- Detected secret for nix option: $(basename "$f")"
        nixopts+=("--option" "$(basename "$f")" "$(cat "$f")")
    fi
done

echo -e "\nBuild log data will stream in below:"
nix "${nixopts[@]}" --show-trace --log-format raw build "$installable"
if [ $? -ne 0 ]; then
    errcode=$?
    echo -e "\nBuild failed, dumping log data:"
    nix log $installable
    exit $errcode
fi
echo -e "\nBuild finished!"

# store the derivation in the shelter for later use
nix derivation show --quiet "$installable" 2>/dev/null > "${BUILDKIT_NIX_BUILD_SHELTER}/derivation.json"

# evaluate the result
if [ -d "$(readlink -f result)" ]; then
    echo -n "derivation" > "${BUILDKIT_NIX_BUILD_SHELTER}/type"
    mkdir -p "${BUILDKIT_NIX_BUILD_SHELTER}/result/nix/store"
    cp -af result/* "${BUILDKIT_NIX_BUILD_SHELTER}/result"
    cp -af $(nix-store -qR result/) "${BUILDKIT_NIX_BUILD_SHELTER}/result/nix/store"
else
    if tar -tf result | grep -q manifest.json; then
        echo -n "ocispec" > "${BUILDKIT_NIX_BUILD_SHELTER}/type"
        cp $(nix-store -qR result/) "${BUILDKIT_NIX_BUILD_SHELTER}/result"
    else
        echo "ERROR: nix build did not produce a valid result"
        exit 1
    fi
fi

exit 0
