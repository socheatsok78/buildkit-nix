variable "GITHUB_REPOSITORY_OWNER" {
	default = "nixfile"
}
variable "GITHUB_REPOSITORY" {
	default = "${GITHUB_REPOSITORY_OWNER}/buildkit-nix"
}

function "ownerprefix" {
	params = [ owner ]
	result = owner == "nixfile" ? "" : "nixfile-"
}

function "tags" {
	params = [ name, tag ]
	result = [
		"${GITHUB_REPOSITORY_OWNER}/${ownerprefix(GITHUB_REPOSITORY_OWNER)}${name}:${tag}",
		"ghcr.io/${GITHUB_REPOSITORY_OWNER}/${ownerprefix(GITHUB_REPOSITORY_OWNER)}${name}:${tag}",
	]
}

target "default" { inherits = ["flakes"] }

target "docker-metadata-action" {}
target "github-metadata-action" {}

target "nixfile-frontend" {
	platforms = [
		"linux/amd64",
		"linux/arm64",
	]
	tags = tags("frontend", "experimental")
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
