#!/bin/bash
set -euo pipefail

root="$(cd "$(dirname "$0")" && pwd)"
rm -rf "$root/gitroot"

docker run --rm --platform linux/arm64 \
  -v "$root:/out" \
  public.ecr.aws/amazonlinux/amazonlinux:2023 \
  bash -c 'set -euo pipefail
    dnf install -y git findutils gawk
    dest=/out/gitroot
    mkdir -p "$dest/bin" "$dest/libexec" "$dest/lib"
    cp -a /usr/bin/git "$dest/bin/git"
    cp -a /usr/libexec/git-core "$dest/libexec/git-core"
    find "$dest/bin" "$dest/libexec" -type f -print0 |
      while IFS= read -r -d "" file; do
        ldd "$file" 2>/dev/null || true
      done |
      awk "/=>/ { print \$3 }" |
      sort -u |
      while read -r lib; do
        if [ -f "$lib" ]; then
          cp -a "$lib" "$dest/lib/"
        fi
      done
  '
