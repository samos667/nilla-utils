let
  inherit (builtins) readDir filter attrNames concatMap hasAttr;
in {
  config.lib.utils.loadHostsFromDir = dir: file: let
    hosts' = let
      contents = readDir dir;
    in
      filter
      (n: contents."${n}" == "directory")
      (attrNames contents);
  in
    concatMap
    (
      n: let
        contents = readDir "${dir}/${n}";
        hasConfig =
          (hasAttr file contents)
          && (contents.${file} == "regular");
      in
        if hasConfig
        then [
          {
            hostname = n;
            configuration = import "${dir}/${n}/${file}";
          }
        ]
        else []
    )
    hosts';
}
