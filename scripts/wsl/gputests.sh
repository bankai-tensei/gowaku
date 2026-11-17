#!/bin/bash
# Runs the GPU-tagged tests of Linux under each window system of WSL: sway
# (sway.sh, wayland-1), WSLg's Weston (wayland-0) and WSLg's XWayland (:0).
# $1 is a directory with the test binaries, built on any system with
#   GOOS=linux CGO_ENABLED=0 go test -c -tags gpu -o DIR/<pkg>.test ./<pkg>
# where <pkg> is platform/wayland, platform/x11, platform/dbus, gpu/vulkan or internal/golden
# with / replaced by _.
BIN=$1
# Vulkan comes from Mesa's lavapipe: install mesa-vulkan-drivers, or extract its
# deb and its dependencies into ~/lx/root as getdebs.sh does for sway.
REPO=$(cd "$(dirname "$0")/../.." && pwd)
export LD_LIBRARY_PATH=$HOME/lx/wl/usr/lib/x86_64-linux-gnu:$HOME/lx/root/usr/lib/x86_64-linux-gnu
export XDG_RUNTIME_DIR=/run/user/$(id -u)
run() { # name, package dir, binary, env...
  local name=$1 dir=$2 bin=$3; shift 3
  cp "$BIN/$bin" /tmp/$bin; chmod +x /tmp/$bin
  if [[ " $* " == *" DBUS_RUN=1 "* ]]; then
    # A session bus of its own, which the test may take names on.
    out=$(cd "$REPO/$dir" && env "$@" dbus-run-session -- /tmp/$bin -test.count=1 -test.v 2>&1)
  else
    out=$(cd "$REPO/$dir" && env "$@" /tmp/$bin -test.count=1 -test.v 2>&1)
  fi
  printf '%-28s %s pass, %s skip, %s fail\n' "$name" "$(grep -c '^--- PASS' <<<"$out")" \
    "$(grep -c '^--- SKIP' <<<"$out")" "$(grep -c '^--- FAIL' <<<"$out")"
  grep -E -A3 '^(--- FAIL|panic)' <<<"$out" | head -30
}
# A headless sway has no keyboard, and without one no window gets the focus
# that the clipboard needs.
WAYLAND_DISPLAY=wayland-1 $HOME/lx/wl/usr/bin/wtype -s 600000 & KPID=$!
sleep 0.3
run "wayland on sway"   platform/wayland platform_wayland.test WAYLAND_DISPLAY=wayland-1
kill $KPID
run "wayland on weston" platform/wayland platform_wayland.test WAYLAND_DISPLAY=wayland-0
run "x11 on xwayland"   platform/x11     platform_x11.test     DISPLAY=:0
run "vulkan on sway"    gpu/vulkan       gpu_vulkan.test       WAYLAND_DISPLAY=wayland-1 GOWAKU_BACKEND=wayland
run "vulkan on weston"  gpu/vulkan       gpu_vulkan.test       WAYLAND_DISPLAY=wayland-0 GOWAKU_BACKEND=wayland
run "vulkan on x11"     gpu/vulkan       gpu_vulkan.test       DISPLAY=:0 GOWAKU_BACKEND=x11
run "golden on sway"    internal/golden  internal_golden.test  WAYLAND_DISPLAY=wayland-1 GOWAKU_BACKEND=wayland
run "dbus with a fake portal" platform/dbus platform_dbus.test GOWAKU_DBUS_TEST=1 DBUS_RUN=1
run "golden on x11"     internal/golden  internal_golden.test  DISPLAY=:0 GOWAKU_BACKEND=x11
