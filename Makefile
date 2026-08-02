it:
	docker buildx bake flakes --set="*.platform="
	docker tag ghcr.io/socheatsok78/nixfile-frontend:experimental nixfile-frontend:experimental
