ARG GOLANG_IMAGE=golang:1.26.5-alpine

FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} AS build
WORKDIR /src
ENV CGO_ENABLED=0
ARG TARGETOS
ARG TARGETARCH
RUN --mount=target=. --mount=target=/root/.cache,type=cache --mount=target=/go/pkg,type=cache \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w" -o /out/nix-frontend ./cmd/nix-frontend

FROM scratch
COPY --from=build /out/ /
LABEL moby.buildkit.frontend.network.none="true"
# nix-frontend isn't technically support these capabilities,
# This is a workaround for the following error:
# - buildx bake failed with: ERROR: current frontend does not support defining additional contexts for targets.
#   Named contexts are supported since Dockerfile v1.4. Use #syntax directive in Dockerfile or update to latest BuildKit.
LABEL moby.buildkit.frontend.caps="moby.buildkit.frontend.inputs,moby.buildkit.frontend.contexts,moby.buildkit.frontend.gitquerystring"
ENTRYPOINT ["/nix-frontend"]
