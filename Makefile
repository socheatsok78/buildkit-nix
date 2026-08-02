it:
	docker buildx bake flakes --set="*.platform="
	docker tag socheatsok78/nixfile-frontend:experimental nixfile-frontend:experimental
bootstrap:
	docker buildx bake dockerfile --set="*.platform="
	docker tag socheatsok78/nixfile-frontend:experimental nixfile-frontend:experimental
