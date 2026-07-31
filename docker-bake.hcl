variable "GITHUB_REPOSITORY" {
    default = "socheatsok78/buildkit-nix"
}
variable "GITHUB_REPOSITORY_OWNER" {
    default = "socheatsok78"
}


target "docker-metadata-action" {}
target "github-metadata-action" {}

target "buildkit-nix-dockerfile" {
    inherits = [ 
        "docker-metadata-action",
        "github-metadata-action",
    ]
    context = "."
    dockerfile = "bootstrap.Dockerfile"
}

target "buildkit-nix" {
    inherits = [ 
        "docker-metadata-action",
        "github-metadata-action",
    ]
    context = "."
    dockerfile = "flake.nix"
    target = "buildkit-nix-image"
}


target "dockerfile" {
    inherits = [ 
        "buildkit-nix-dockerfile",
    ]
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
        "buildkit-nix",
    ]
    platforms = [
        "linux/amd64",
        "linux/arm64",
    ]
    tags = [
        "ghcr.io/${GITHUB_REPOSITORY_OWNER}/buildkit-nix:experimental",
    ]
}


target "default" {
    inherits = [ "dockerfile" ]
}
