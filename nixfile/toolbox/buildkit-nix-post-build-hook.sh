#!/bin/sh

set -eu
set -f # disable globbing
export IFS=' '

echo "Running buildkit-nix post-build hook:"
echo $OUT_PATHS

echo "printenv:"
printenv

# if [ -d "$(readlink -f result)" ]; then
#     echo -n "derivation" > "${BUILDKIT_NIX_BUILD_SHELTER}/type"
#     mkdir -p "${BUILDKIT_NIX_BUILD_SHELTER}/result/nix/store"
#     cp -af result/* "${BUILDKIT_NIX_BUILD_SHELTER}/result"
#     cp -af $(nix-store -qR result/) "${BUILDKIT_NIX_BUILD_SHELTER}/result/nix/store"
# else
#     if tar -tf result | grep -q manifest.json; then
#         echo -n "ocispec" > "${BUILDKIT_NIX_BUILD_SHELTER}/type"
#         cp $(nix-store -qR result/) "${BUILDKIT_NIX_BUILD_SHELTER}/result"
#     else
#         echo "ERROR: nix build did not produce a valid result"
#         exit 1
#     fi
# fi
