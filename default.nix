{ writeShellScriptBin }:

writeShellScriptBin "workspaced" ''
  dotfilesFolder=
  if [ -d ~/.internal/dotfiles ]; then
    dotfilesFolder=~/.internal/dotfiles
  elif [ -d "$HOME/.internal/dotfiles" ]; then
    dotfilesFolder="$HOME/.internal/dotfiles"
  elif [ -d /etc/.internal/dotfiles ]; then
    dotfilesFolder=/etc/.internal/dotfiles
  fi
  if [ -z "$dotfilesFolder" ]; then
    echo "can't find internal/dotfiles folder" >&2
    exit 1
  fi
  exec "$dotfilesFolder/bin/shim/workspaced" "$@"
''
