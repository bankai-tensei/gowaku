# Animation fixtures

Small files for `animation_test.go` that Go cannot encode itself (WebP), and
two animations made by real encoders, to check that `DecodeAnimation`
composites what they choose. The GIF and most APNG cases are built in the
tests instead.

The pictures come from `fixtureStill` and `fixtureMovie` in
`animation_fixture_test.go`. To make them again:

```bash
GOWAKU_ANIM_SOURCES=/tmp/anim go test -run TestAnimationFixtureSources ./paint/
cd /tmp/anim
# Lossless still WebP (VP8L).
for n in red4 blue4half green2; do ffmpeg -y -i $n.png -c:v libwebp -lossless 1 -pix_fmt bgra $n.webp; done
# Lossy still WebP with alpha (VP8X, ALPH, VP8).
ffmpeg -y -i whitehalf4.png -c:v libwebp -lossless 0 -q:v 90 -pix_fmt yuva420p whitehalf4.webp
# Animated WebP from libwebp's WebPAnimEncoder, lossless.
ffmpeg -y -framerate 10 -i movie%d.png -c:v libwebp_anim -lossless 1 -pix_fmt bgra -loop 0 movie.webp
# APNG from ffmpeg's encoder.
ffmpeg -y -framerate 10 -i movie%d.png -plays 0 -f apng movie.png
```

They were made with ffmpeg 2025-07-07-git-d2828ab284 (full build, Windows),
with libwebp. Both animations use partial frames: `movie.webp` uses dispose to
background, alpha blending and no blending; `movie.png` uses dispose to
background and the source blend, with the default image as the first frame.
`internal/golden/testdata/anim` holds copies for the `animated` golden.
