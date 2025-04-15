let
  pins = import ../../npins;

  nilla = import pins.nilla;
in
  nilla.create ({config}: {
    includes = [
      ../../modules
    ];

    config = {
      inputs.nixpkgs.src = builtins.fetchTarball {
        url = "https://releases.nixos.org/nixos/24.11/nixos-24.11.716868.60e405b241ed/nixexprs.tar.xz";
        sha256 = "111zrdbnd2b7w64q07773ksf4gfbm4gq7riggxld1gmxpimprj0j";
      };

      systems.nixos.mysystem = {
        system = "x86_64-linux";

        modules = [
          {
            boot.loader.grub.devices = ["/dev/sda"];
            fileSystems = {
              "/" = {
                device = "/dev/sda1";
              };
            };
            system.stateVersion = "24.11";
          }
        ];
      };
    };
  })
