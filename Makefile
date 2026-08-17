buildxflags :=  --set="*.platform=" --set="*.tags=nixfile-frontend:experimental"
no-cache := false
ifeq ($(no-cache), true)
	buildxflags += --no-cache
endif
it: flakes
bootstrap: dockerfile
flakes:
	docker buildx bake flakes --load $(buildxflags)
dockerfile:
	docker buildx bake dockerfile --load $(buildxflags)
