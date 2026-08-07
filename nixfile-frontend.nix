{
  lib,
  maintainers,
  buildGoModule,
}:
buildGoModule (finalAttrs: {
  pname = "nixfile-frontend";
  version = "experimental";

  src = ./.;

  subPackages = [
    "cmd/nixfile-frontend"
  ];

  vendorHash = "sha256-4doFp5UonyTpQlpMewEbaJXy9m94NAlQDVTH7YnBpvA=";

  ldflags = [
    "-s"
    "-w"
  ];

  env = {
    CGO_ENABLED = 0; # Structured environment block
  };

  meta = with lib; {
    homepage = "https://github.com/socheatsok78/buildkit-nix";
    description = "An experimental BuildKit frontend for building Nix Flakes as Dockerfile";
    license = licenses.asl20;
    mainProgram = "nixfile-frontend";
    maintainers = [
      maintainers.socheatsok78
    ];
  };
})
