{config}: let
  inherit (config) lib;
in {
  options.overlays = lib.options.create {
    description = "Overlays which are available from this Nilla project.";
    type = lib.types.attrs.of lib.types.raw;
    default.value = {};
  };
}
