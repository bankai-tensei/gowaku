#!/bin/bash
# Reference numbers of gowaku in WSL: headless sway (Wayland, pixman) with
# Vulkan on lavapipe, the CPU driver of Mesa. Not comparable with real GPUs.
D=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$D/../../.." && pwd)
bash "$REPO/scripts/wsl/sway.sh" > /dev/null 2>&1
export LD_LIBRARY_PATH=$HOME/lx/wl/usr/lib/x86_64-linux-gnu:$HOME/lx/root/usr/lib/x86_64-linux-gnu
export XDG_RUNTIME_DIR=/run/user/$(id -u)
export WAYLAND_DISPLAY=wayland-1 GOWAKU_BACKEND=wayland
B=/tmp/benchg; mkdir -p $B; cp "$D/gowaku" $B/gowaku; chmod +x $B/gowaku
CLK=$(getconf CLK_TCK)

echo "binary gowaku=$(stat -c %s "$D/gowaku") hello=$(stat -c %s "$D/hello-gowaku")"

# startup: process start to READY, 7 runs (the first warms the caches).
for i in 1 2 3 4 5 6 7; do
  s=$(date +%s%N)
  $B/gowaku -scenario startup -duration 20s > /dev/null
  e=$(date +%s%N)
  echo "startup $(( (e - s) / 1000000 ))"
done

# cpu prints the CPU time of pid in milliseconds.
cpu() { awk -v clk=$CLK '{print ($14+$15)*1000/clk}' /proc/$1/stat; }

# run scenario seconds: frames per second and CPU % after the first 2 seconds.
run() {
  $B/gowaku -scenario $1 -duration ${2}s -out $B/$1.json > /dev/null &
  pid=$!
  sleep 2; c0=$(cpu $pid); t0=$(date +%s%N)
  sleep $(( $2 - 3 )); c1=$(cpu $pid); t1=$(date +%s%N)
  wait $pid
  frames=$(python3 -c "import json,sys;t=json.load(open('$B/$1.json'));s=t[0]+2e9;e=t[0]+($2-1)*1e9;n=[x for x in t if s<=x<e];print(len(n)/(($2-3)))" 2>/dev/null)
  awk -v c0=$c0 -v c1=$c1 -v t0=$t0 -v t1=$t1 -v f="$frames" -v n=$1 'BEGIN{printf "%s fps=%s cpu=%.1f%%\n", n, f, (c1-c0)/((t1-t0)/1e6)*100}'
}
run animate 12
run dense 12
run dense-flat 12
run idle 32
