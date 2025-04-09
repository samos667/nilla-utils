{config}: let
  inherit (config) lib;
  inherit (builtins) mapAttrs;
in {
  options = {
    overlays = lib.options.create {
      description = "Overlays which are available from this Nilla project.";
      type = lib.types.attrs.of lib.types.raw;
      default.value = {};
    };

    generators.overlays = lib.options.create {
      description = "Overlays that should be generated from a directory structure.";
      default.value = {};
      type = lib.types.attrs.lazy (lib.types.submodule ({config}: {
        options.folder = lib.options.create {
          type = lib.types.path;
          description = "The folder to auto discover packages.";
        };
      }));
    };
  };

  config = {
    overlays = let
      mkOverlayFromDir = dir: (f: p: (
        mapAttrs
        (_: d: f.callPackage d {})
        (lib.utils.loadDirsWithFile "default.nix" dir)
      ));
    in
      mapAttrs
      (_: opts: mkOverlayFromDir opts.folder)
      config.generators.overlays;
  };
}
