#!/bin/bash
# Starts a headless sway on $XDG_RUNTIME_DIR/wayland-1, from the debs that
# getdebs.sh extracted into ~/lx/wl. It needs no GPU, no seat and no root.
export LD_LIBRARY_PATH=$HOME/lx/wl/usr/lib/x86_64-linux-gnu:$LD_LIBRARY_PATH
export PATH=$HOME/lx/wl/usr/bin:$PATH
export WLR_BACKENDS=headless WLR_RENDERER=pixman WLR_LIBINPUT_NO_DEVICES=1
export XDG_RUNTIME_DIR=/run/user/$(id -u)
unset WAYLAND_DISPLAY DISPLAY
mkdir -p ~/lx/swaycfg
cat > ~/lx/swaycfg/config <<'EOF'
output HEADLESS-1 resolution 1280x800 scale 1
default_border normal 2
xwayland disable
for_window [app_id="gowaku"] floating enable
seat seat0 keyboard_grouping none
EOF
pkill -f "sway -c" 2>/dev/null
sleep 0.5
nohup sway -c ~/lx/swaycfg/config > ~/lx/sway.log 2>&1 &
for _ in $(seq 20); do [ -S $XDG_RUNTIME_DIR/wayland-1 ] && break; sleep 0.2; done
ls $XDG_RUNTIME_DIR/wayland-1
