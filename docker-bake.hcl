target "default" {
    context = "."
    dockerfile = "bootstrap.Dockerfile"
    tags = [
        "ghcr.io/socheatsok78/buildkit-nix:dev"
    ]
}
