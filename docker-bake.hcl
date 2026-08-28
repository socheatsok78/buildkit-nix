variable "GITHUB_REPOSITORY" {
    default = "socheatsok78/buildkit-nix"
}
variable "GITHUB_REPOSITORY_OWNER" {
    default = "socheatsok78"
}


target "docker-metadata-action" {}
target "github-metadata-action" {}

target "nixfile-frontend" {
    platforms = [
        "linux/amd64",
        "linux/arm64",
    ]
    tags = [
        "${GITHUB_REPOSITORY_OWNER}/nixfile-frontend:experimental",
        "ghcr.io/${GITHUB_REPOSITORY_OWNER}/nixfile-frontend:experimental",
    ]
}

target "bootstrap" {
    inherits = [ 
        "docker-metadata-action",
        "github-metadata-action",
    ]
    context = "."
    dockerfile = "bootstrap.Dockerfile"
	tags = [
        "${GITHUB_REPOSITORY_OWNER}/nixfile-frontend:bootstrap",
        "ghcr.io/${GITHUB_REPOSITORY_OWNER}/nixfile-frontend:bootstrap",
    ]
}


target "dockerfile" {
    inherits = [ 
        "docker-metadata-action",
        "github-metadata-action",
        "nixfile-frontend",
    ]
    context = "."
    dockerfile = "nixfile-frontend.Dockerfile"
}

target "flakes" {
    inherits = [ 
        "docker-metadata-action",
        "github-metadata-action",
        "nixfile-frontend",
    ]
    context = "."
    dockerfile = "flake.nix"
    target = "nixfile-frontend-image"
}

target "default" {
    inherits = [ "flakes" ]
}
