{
  description = "shrt, the URL shortener for the hypr.sh homelab";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = f: lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAllSystems (pkgs: {
        default = pkgs.buildGoModule {
          pname = "shrt";
          version = self.shortRev or self.dirtyShortRev or "dev";
          # Only the Go sources, so a docs change doesn't rebuild the package.
          src = lib.fileset.toSource {
            root = ./.;
            fileset = lib.fileset.unions [
              ./go.mod
              ./go.sum
              (lib.fileset.fileFilter (file: file.hasExt "go") ./.)
            ];
          };
          # Update when go.mod or go.sum change; a wrong hash fails the build
          # and prints the right one.
          vendorHash = "sha256-7IC/p5GlD2EZkDXQzkaZ7E19S/ABKEBsg68vt8pykis=";
          env.CGO_ENABLED = "0";
          ldflags = [
            "-s"
            "-w"
          ];
          meta = {
            description = "URL shortener for the hypr.sh homelab";
            mainProgram = "shrt";
          };
        };
      });
    };
}
