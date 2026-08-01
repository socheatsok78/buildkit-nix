it:
	docker buildx bake dockerfile --set="*.platform="
	docker tag ghcr.io/socheatsok78/buildkit-nix:experimental ghcr.io/socheatsok78/buildkit-nix:local
