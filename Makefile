buildxflags :=  --set="*.platform=" --set="*.tags=nixfile-frontend:experimental"
no-cache := false
ifeq ($(no-cache), true)
	buildxflags += --no-cache
endif
it:
	docker buildx bake '*' --print
bootstrap:
	@echo "Building bootstrap image..."
	docker buildx bake bootstrap --load $(buildxflags)
flakes:
	@echo "Building image..."
	docker buildx bake flakes --load $(buildxflags)
dockerfile:
	@echo "Building bootstrap image..."
	docker buildx bake dockerfile --load $(buildxflags)
