it: flakes retag
bootstrap: dockerfile retag
flakes:
	docker buildx bake flakes --set="*.platform="
dockerfile:
	docker buildx bake dockerfile --set="*.platform="
retag:
	docker tag socheatsok78/nixfile-frontend:experimental nixfile-frontend:experimental
