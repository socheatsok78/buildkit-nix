it: flakes
bootstrap: dockerfile
flakes:
	docker buildx bake flakes --set="*.platform=" --set="*.tags=nixfile-frontend:experimental"
dockerfile:
	docker buildx bake dockerfile --set="*.platform=" --set="*.tags=nixfile-frontend:experimental"
