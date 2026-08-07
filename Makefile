buildxflags :=  --set="*.platform=" --set="*.tags=nixfile-frontend:experimental"
it: flakes
bootstrap: dockerfile
flakes:
	docker buildx bake flakes --load $(buildxflags)
dockerfile:
	docker buildx bake dockerfile --load $(buildxflags)
