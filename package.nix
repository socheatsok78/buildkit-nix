{
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
})
