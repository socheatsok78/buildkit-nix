#!/bin/sh

BUILDKIT_NIX_SHELTER_DIR=${BUILDKIT_NIX_SHELTER_DIR:-"/nix-store-closure"}

installable="$1"
nixopts=( --show-trace --print-build-logs --log-format raw )

echo "Prepare build environment..."

# Pass any secrets from /run/secrets to the nix build as options
for f in /run/secrets/*; do
	if [ -f "$f" ]; then
		echo "- Reading nix option '$(basename "$f")' from secret..."
		nixopts+=("--option" "$(basename "$f")" "$(cat "$f")")
	fi
done

# If GITHUB_TOKEN is not empty, then add it to the nix options as a secret
{
	GITHUB_SERVER_URL=${GITHUB_SERVER_URL:-"https://github.com"}
	__GITHUB_SERVER_HOST=${GITHUB_SERVER_URL#https://}
	__GITHUB_SERVER_HOST=${__GITHUB_SERVER_HOST%%/}
	if [ ! -f "/run/access-tokens/${__GITHUB_SERVER_HOST}" ] && [ -n "${GITHUB_TOKEN:-}" ]; then
		echo "- Reading nix option 'access-tokens' for ${__GITHUB_SERVER_HOST} from environment variable GITHUB_TOKEN..."
		nixopts+=("--option" "access-tokens" "${__GITHUB_SERVER_HOST}=${GITHUB_TOKEN}")
	fi
}

# Pass any access tokens from /run/access-tokens to the nix build as options
for f in /run/access-tokens/*; do
	if [ -f "$f" ]; then
		echo "- Reading nix option 'access-tokens' for $(basename "$f") from secret..."
		nixopts+=("--option" "access-tokens" "$(basename "$f")=$(cat "$f")")
	fi
done

echo -e "\nBuild log data will stream in below:"
nix "${nixopts[@]}" build "$installable"
errcode=$?
if [ $errcode -ne 0 ]; then
	exit $errcode
fi
echo -e "\nBuild finished!"

# store the derivation in the shelter for later use
nix derivation show --quiet "$installable" 2>/dev/null > "${BUILDKIT_NIX_SHELTER_DIR}/derivation.json"

# evaluate the result
if [ -d "$(readlink -f result)" ]; then
	echo -n "derivation" > "${BUILDKIT_NIX_SHELTER_DIR}/type"
	mkdir -p "${BUILDKIT_NIX_SHELTER_DIR}/result/nix/store"
	cp -af result/* "${BUILDKIT_NIX_SHELTER_DIR}/result"
	cp -af $(nix-store -qR result/) "${BUILDKIT_NIX_SHELTER_DIR}/result/nix/store"
else
	if tar -tf result | grep -q manifest.json; then
		echo -n "oci" > "${BUILDKIT_NIX_SHELTER_DIR}/type"
		cp $(nix-store -qR result/) "${BUILDKIT_NIX_SHELTER_DIR}/result"
	else
		echo -n "shelter" > "${BUILDKIT_NIX_SHELTER_DIR}/type"
		cp $(nix-store -qR result/) "${BUILDKIT_NIX_SHELTER_DIR}/result"
	fi
fi

exit 0
