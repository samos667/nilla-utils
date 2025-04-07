{
  lib,
  buildGoApplication,
  nvd,
  makeWrapper,
}: let
  version = "0.0.0";
in
  buildGoApplication {
    inherit version;
    pname = "nilla-utils-plugins";

    src = lib.cleanSource ./.;

    modules = ./gomod2nix.toml;

    subPackages = ["cmd/nilla-os" "cmd/nilla-home"];
    ldflags = ["-X main.version=${version}"];

    nativeBuildInputs = [makeWrapper];

    postInstall = ''
      wrapProgram $out/bin/nilla-os --prefix PATH : ${lib.makeBinPath [nvd]}
      wrapProgram $out/bin/nilla-home --prefix PATH : ${lib.makeBinPath [nvd]}
    '';
  }
