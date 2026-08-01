variable "GITHUB_REPOSITORY" {
    default = "socheatsok78/buildkit-nix"
}
variable "GITHUB_REPOSITORY_OWNER" {
    default = "socheatsok78"
}


target "docker-metadata-action" {}
target "github-metadata-action" {}

target "dockerfile" {
    inherits = [ 
        "docker-metadata-action",
        "github-metadata-action",
    ]
    context = "."
    dockerfile = "bootstrap.Dockerfile"
    platforms = [
        "linux/amd64",
        "linux/arm64",
    ]
    tags = [
        "ghcr.io/${GITHUB_REPOSITORY_OWNER}/buildkit-nix:experimental",
    ]
}

target "flakes" {
    inherits = [ 
        "docker-metadata-action",
        "github-metadata-action",
    ]
    context = "."
    dockerfile = "flake.nix"
    target = "buildkit-nix-image"
    platforms = [
        "linux/amd64",
        "linux/arm64",
    ]
    tags = [
        "ghcr.io/${GITHUB_REPOSITORY_OWNER}/buildkit-nix:experimental",
    ]
}

target "default" {
    inherits = [ "flakes" ]
}
