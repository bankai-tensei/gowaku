#!/bin/bash
# Downloads sway, grim, wtype, wl-clipboard and wayland-protocols from the Ubuntu
# archive with the packages they need that the system lacks, and extracts them
# into ~/lx/wl. It needs no root: apt-get download only fetches.
set -e
mkdir -p ~/lx/debs10 ~/lx/wl
cd ~/lx/debs10
pkgs="sway grim wtype wl-clipboard wayland-protocols"
all=$(apt-cache depends --recurse --no-recommends --no-suggests --no-conflicts --no-breaks \
  --no-replaces --no-enhances $pkgs | grep -E '^[a-zA-Z0-9]' | sort -u)
need=""
for p in $all; do
  if ! dpkg-query -W -f='${Status}' "$p" 2>/dev/null | grep -q "install ok installed"; then
    need="$need $p"
  fi
done
apt-get download $need
for f in *.deb; do dpkg-deb -x "$f" ~/lx/wl; done
ls ~/lx/wl/usr/bin
