#!/bin/sh
printf '\033c\033]0;%s\a' hh
base_path="$(dirname "$(realpath "$0")")"
"$base_path/hh.x86_64" "$@"
