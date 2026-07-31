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
ENTRYPOINT ["/nix-frontend"]
