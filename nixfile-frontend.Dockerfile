# Nix builder
FROM nixos/nix:latest AS builder

# Copy our source and setup our working dir.
COPY . /tmp/build
WORKDIR /tmp/build

# Build our Nix environment
RUN nix \
	--extra-experimental-features "nix-command flakes" \
	--option filter-syscalls false \
	build .#nixfile-frontend

# Copy the Nix store closure into a directory. The Nix store closure is the
# entire set of Nix store values that we need for our build.
RUN mkdir /tmp/nix-store-closure
RUN cp -R $(nix-store -qR result/) /tmp/nix-store-closure

# Final image is based on scratch. We copy a bunch of Nix dependencies
# but they're fully self-contained so we don't need Nix anymore.
FROM scratch

# Copy /nix/store
COPY --from=builder /tmp/nix-store-closure /nix/store
COPY --from=builder /tmp/build/result /

LABEL moby.buildkit.frontend.network.none="true"
# nixfile-frontend isn't technically support these capabilities,
# This is a workaround for the following error:
# - buildx bake failed with: ERROR: current frontend does not support defining additional contexts for targets.
#   Named contexts are supported since Dockerfile v1.4. Use #syntax directive in Dockerfile or update to latest BuildKit.
LABEL moby.buildkit.frontend.caps="moby.buildkit.frontend.inputs,moby.buildkit.frontend.contexts,moby.buildkit.frontend.gitquerystring"
ENTRYPOINT ["/bin/nixfile-frontend"]
