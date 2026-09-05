#!/usr/bin/env bash
# tools/demo-assets/fetch.sh - maintainer-only authoring script (ROADMAP.79 RA-2).
#
# Re-downloads and re-processes the curated demo photos/videos from Wikimedia
# Commons into internal/demo/assets/{photos,videos}/. Not part of the server
# build, not run in CI or `make generate` - run it by hand when refreshing the
# curated set, then hand-edit internal/demo/assets/manifest.yaml to match
# whatever content you swapped in (tags/fields/alt text must describe what's
# actually in the new file - see RA-1/RA-4 in ROADMAP.79.demo-real-assets.md).
#
# Requires: curl, ffmpeg/ffprobe, exiftool, ImageMagick (`convert`).
#
# Usage:
#   tools/demo-assets/fetch.sh
#
# Re-runs are safe: raw downloads are cached in .cache/ (gitignored) so a
# second run doesn't re-hit Wikimedia unless you delete the cache.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ASSETS_DIR="$SCRIPT_DIR/../../internal/demo/assets"
CACHE_DIR="$SCRIPT_DIR/.cache"
PHOTOS_DIR="$ASSETS_DIR/photos"
VIDEOS_DIR="$ASSETS_DIR/videos"

mkdir -p "$CACHE_DIR" "$PHOTOS_DIR" "$VIDEOS_DIR"

ua="damask-dam-demo-asset-fetch/1.0 (offline authoring tool)"

# download URL cache_name
download() {
  local url="$1" name="$2"
  local dest="$CACHE_DIR/$name"
  if [ -f "$dest" ]; then
    echo "cached: $name"
    return
  fi
  echo "downloading: $name"
  curl -sS --max-time 60 -A "$ua" -L "$url" -o "$dest"
}

# --- Source list: Wikimedia Commons file URLs -> cache name -----------------
# Picked by hand (see internal/demo/assets/ATTRIBUTION.md for full credits);
# re-picking new sources means editing this list and manifest.yaml together.

download "https://upload.wikimedia.org/wikipedia/commons/e/ef/Ocean_beach_at_low_tide_against_the_sun.jpg" beach.jpg
download "https://upload.wikimedia.org/wikipedia/commons/7/75/Studio_portrait_photography_2.jpg" studio.jpg
download "https://upload.wikimedia.org/wikipedia/commons/1/19/California_surfer_inside_wave.jpg" surfer.jpg
download "https://upload.wikimedia.org/wikipedia/commons/c/c4/Person_hiking_on_trail_-_Lula_Lake_Land_Trust_%28Unsplash%29.jpg" hike.jpg
download "https://upload.wikimedia.org/wikipedia/commons/5/50/Yoga_mat_and_water_bottle_in_a_living_room.jpg" yoga.jpg
download "https://upload.wikimedia.org/wikipedia/commons/4/45/A_Specimen_by_William_Caslon.jpg" fontspecimen.jpg
download "https://upload.wikimedia.org/wikipedia/commons/2/25/Standard_Color_Card_of_America_-_DPLA_-_19cddb4f9f1a842113dc847b3bc1266f_%28page_1%29.jpg" moodboard.jpg
download "https://upload.wikimedia.org/wikipedia/commons/9/9b/Ocean_waves_at_L%C3%A6kjavik_beach%2C_Iceland.webm" teaser.webm
download "https://upload.wikimedia.org/wikipedia/commons/1/17/Stevie_Ray_Vaughan_soundcheck_in_the_studio_in_1989.webm" bts.webm
download "https://upload.wikimedia.org/wikipedia/commons/c/cd/Baspa_River%2C_Chitkul%2C_Himachal_Pradesh.jpg" river.jpg
download "https://upload.wikimedia.org/wikipedia/commons/2/27/Mount_Shasta_as_seen_from_Bunny_Flat.jpg" mountain.jpg
download "https://upload.wikimedia.org/wikipedia/commons/1/14/Victoria_Falls_-_VicFalls3452.jpg" gorge.jpg
download "https://upload.wikimedia.org/wikipedia/commons/7/7b/Cenote_Dolomiti.jpg" cenote.jpg
download "https://upload.wikimedia.org/wikipedia/commons/a/a2/Teide_von_Nordosten_%28Zuschnitt_1%29.jpg" volcano.jpg
download "https://upload.wikimedia.org/wikipedia/commons/a/af/Grand_Canyon_view_from_Pima_Point_2010.jpg" canyon.jpg
download "https://upload.wikimedia.org/wikipedia/commons/0/00/Antelope_Canyon_Mittags.jpg" antelope-canyon.jpg
download "https://upload.wikimedia.org/wikipedia/commons/f/f7/Bryce_Canyon_Amphitheater_Hoodoos_Panorama.jpg" bryce-canyon.jpg
download "https://upload.wikimedia.org/wikipedia/commons/e/e9/Gorges_du_Verdon_from_bridge_over_Lac_de_Sainte-Croix%2C_Provence.jpg" verdon-gorge.jpg
download "https://upload.wikimedia.org/wikipedia/commons/b/ba/Durmitor%2C_canyon_Tara_-_Montenegro.JPG" tara-canyon.jpg
download "https://upload.wikimedia.org/wikipedia/commons/6/69/Arenal_volcano_%2870785p%29.jpg" arenal-volcano.jpg
download "https://upload.wikimedia.org/wikipedia/commons/7/7d/Mount_Bromo_at_sunrise%2C_showing_its_volcanoes_and_Mount_Semeru_%28background%29.jpg" bromo-volcano.jpg
download "https://upload.wikimedia.org/wikipedia/commons/c/c3/Taal_volcano_aerial.jpg" taal-volcano.jpg
download "https://upload.wikimedia.org/wikipedia/commons/f/fb/Cenote_Ik-Kil.jpg" ikkil-cenote.jpg
download "https://upload.wikimedia.org/wikipedia/commons/6/64/Halocline_cenote_Chac_Mool.jpg" chacmool-cenote.jpg
download "https://upload.wikimedia.org/wikipedia/commons/8/8c/Cenote_Angelita.jpg" angelita-cenote.jpg
download "https://upload.wikimedia.org/wikipedia/commons/f/f3/View_to_Geiranger_from_Flydalsjuvet%2C_2013_June.jpg" geirangerfjord.jpg
download "https://upload.wikimedia.org/wikipedia/commons/8/8c/A_Late_evening_view_to_Austnesfjorden_at_Sildpollnes_Church%2C_Austv%C3%A5g%C3%B8ya%2C_Lofoten%2C_Norway%2C_2015_April.jpg" austnesfjorden.jpg
download "https://upload.wikimedia.org/wikipedia/commons/8/82/Torres_del_Paine_-_Janvier_2010.jpg" torresdelpaine.jpg
download "https://upload.wikimedia.org/wikipedia/commons/1/17/Zermatt_and_Matterhorn.jpg" matterhorn.jpg
download "https://upload.wikimedia.org/wikipedia/commons/7/7a/Cirque_de_Gavarnie%2C_Haute-Pyr%C3%A9n%C3%A9es%2C_France.jpg" gavarnie.jpg
download "https://upload.wikimedia.org/wikipedia/commons/7/77/1_tianzishan_wulingyuan_zhangjiajie_2012.jpg" zhangjiajie.jpg
download "https://upload.wikimedia.org/wikipedia/commons/5/5a/Cataratas_Iguacu_Iguazu_Falls.jpg" iguazu-falls.jpg
download "https://upload.wikimedia.org/wikipedia/commons/6/61/Kuang_Si_Falls_and_its_turquoise_tiered_water_pools_in_Luang_Prabang_province_Laos.jpg" kuangsi-falls.jpg
download "https://upload.wikimedia.org/wikipedia/commons/0/0b/Dead_Vlei_-_Sossusvlei_-_Namib-Desert_-_Namibia_%2823394319372%29.jpg" deadvlei.jpg
download "https://upload.wikimedia.org/wikipedia/commons/3/32/Ha_Long_Bay_panorama.jpg" halongbay.jpg

cd "$CACHE_DIR"

# --- Re-encode photos to web-reasonable sizes --------------------------------
ffmpeg -y -v error -i beach.jpg  -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/hero-shot-beach.jpg"
ffmpeg -y -v error -i studio.jpg -vf "scale=-2:1060:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2:black" -q:v 3 "$PHOTOS_DIR/hero-shot-studio.jpg"
ffmpeg -y -v error -i surfer.jpg -vf "scale='min(1600,iw)':'min(1600,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/lifestyle-01.jpg"
ffmpeg -y -v error -i hike.jpg   -vf "scale='min(1600,iw)':'min(1600,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/lifestyle-02.jpg"
ffmpeg -y -v error -i yoga.jpg   -vf "scale='min(1600,iw)':'min(1600,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/lifestyle-03.jpg"
ffmpeg -y -v error -i moodboard.jpg -vf "scale='min(1600,iw)':'min(1600,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/mood-board-final.jpg"
ffmpeg -y -v error -i fontspecimen.jpg -vf "scale='min(1400,iw)':'min(1400,ih)':force_original_aspect_ratio=decrease" "$PHOTOS_DIR/font-specimen.png"
ffmpeg -y -v error -i river.jpg    -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-river.jpg"
ffmpeg -y -v error -i mountain.jpg -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-mountain.jpg"
ffmpeg -y -v error -i gorge.jpg    -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-gorge.jpg"
ffmpeg -y -v error -i cenote.jpg   -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-cenote.jpg"
ffmpeg -y -v error -i volcano.jpg  -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-volcano.jpg"
ffmpeg -y -v error -i canyon.jpg   -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-canyon.jpg"
ffmpeg -y -v error -i antelope-canyon.jpg -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-antelope-canyon.jpg"
ffmpeg -y -v error -i bryce-canyon.jpg    -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-bryce-canyon.jpg"
ffmpeg -y -v error -i verdon-gorge.jpg    -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-verdon-gorge.jpg"
ffmpeg -y -v error -i tara-canyon.jpg     -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-tara-canyon.jpg"
ffmpeg -y -v error -i arenal-volcano.jpg  -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-arenal-volcano.jpg"
ffmpeg -y -v error -i bromo-volcano.jpg   -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-bromo-volcano.jpg"
ffmpeg -y -v error -i taal-volcano.jpg    -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-taal-volcano.jpg"
ffmpeg -y -v error -i ikkil-cenote.jpg    -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-ikkil-cenote.jpg"
ffmpeg -y -v error -i chacmool-cenote.jpg -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-chacmool-cenote.jpg"
ffmpeg -y -v error -i angelita-cenote.jpg -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-angelita-cenote.jpg"
ffmpeg -y -v error -i geirangerfjord.jpg  -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-geirangerfjord.jpg"
ffmpeg -y -v error -i austnesfjorden.jpg  -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-austnesfjorden.jpg"
ffmpeg -y -v error -i torresdelpaine.jpg  -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-torresdelpaine.jpg"
ffmpeg -y -v error -i matterhorn.jpg      -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-matterhorn.jpg"
ffmpeg -y -v error -i gavarnie.jpg        -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-gavarnie.jpg"
ffmpeg -y -v error -i zhangjiajie.jpg     -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-zhangjiajie.jpg"
ffmpeg -y -v error -i iguazu-falls.jpg    -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-iguazu-falls.jpg"
ffmpeg -y -v error -i kuangsi-falls.jpg   -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-kuangsi-falls.jpg"
ffmpeg -y -v error -i deadvlei.jpg        -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-deadvlei.jpg"
ffmpeg -y -v error -i halongbay.jpg       -vf "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease" -q:v 3 "$PHOTOS_DIR/landscape-halongbay.jpg"

# --- Platform-format social crops -------------------------------------------
ffmpeg -y -v error -i beach.jpg  -vf "crop=ih:ih,scale=1080:1080" -q:v 3 "$PHOTOS_DIR/instagram-square.jpg"
ffmpeg -y -v error -i surfer.jpg -vf "crop=ih*9/16:ih,scale=1080:1920" -q:v 3 "$PHOTOS_DIR/instagram-story.jpg"
ffmpeg -y -v error -i beach.jpg  -vf "crop=iw:iw/3,scale=1500:500" -q:v 3 "$PHOTOS_DIR/twitter-banner.jpg"
ffmpeg -y -v error -i teaser.webm -vf "select=eq(n\,50)" -vframes 1 teaser_frame.jpg
ffmpeg -y -v error -i teaser_frame.jpg -vf "crop=iw:iw*312/820,scale=820:312" -q:v 3 "$PHOTOS_DIR/facebook-cover.jpg"

# --- Videos: mute (avoid any embedded-audio rights question), scale, h264 ---
ffmpeg -y -v error -i teaser.webm -an -vf "scale=1280:720" -c:v libx264 -preset veryslow -crf 28 -pix_fmt yuv420p "$VIDEOS_DIR/teaser-15s.mp4"
ffmpeg -y -v error -ss 00:00:05 -i bts.webm -t 10 -an -vf "scale=1280:720" -c:v libx264 -preset veryslow -crf 28 -pix_fmt yuv420p "$VIDEOS_DIR/behind-the-scenes.mp4"

# --- Logos: generated marks, not stock content ------------------------------
convert -size 512x512 xc:white \
  -fill '#f59e0b' -draw "polygon 256,90 400,230 350,230 256,140 162,230 112,230" \
  -fill '#6366f1' -draw "polygon 256,422 112,282 162,282 256,372 350,282 400,282" \
  "$PHOTOS_DIR/logo-primary-light.png"

convert -size 512x512 xc:'#1e1e1e' \
  -fill '#f59e0b' -draw "polygon 256,90 400,230 350,230 256,140 162,230 112,230" \
  -fill '#818cf8' -draw "polygon 256,422 112,282 162,282 256,372 350,282 400,282" \
  "$PHOTOS_DIR/logo-primary-dark.png"

convert -size 800x200 xc:'#f0f0ff' \
  -fill '#f59e0b' -draw "polygon 100,50 160,105 140,105 100,75 60,105 40,105" \
  -fill '#6366f1' -draw "polygon 100,158 40,120 60,120 100,140 140,120 160,120" \
  -fill '#6366f1' -draw "roundrectangle 200,80 720,120 10,10" \
  -fill '#f0f0ff' -draw "roundrectangle 220,90 260,110 4,4" \
  -fill '#f0f0ff' -draw "roundrectangle 280,90 400,110 4,4" \
  -fill '#f0f0ff' -draw "roundrectangle 420,90 540,110 4,4" \
  -fill '#f0f0ff' -draw "roundrectangle 560,90 700,110 4,4" \
  "$PHOTOS_DIR/logo-wordmark.png"

# --- Synthetic EXIF on real photos (RA-5) -----------------------------------
d10=$(date -d '-10 days' '+%Y:%m:%d %H:%M:%S')
d9=$(date -d '-9 days' '+%Y:%m:%d %H:%M:%S')
d8=$(date -d '-8 days' '+%Y:%m:%d %H:%M:%S')
d7=$(date -d '-7 days' '+%Y:%m:%d %H:%M:%S')
d6=$(date -d '-6 days' '+%Y:%m:%d %H:%M:%S')

exiftool -overwrite_original -q \
  -Make="Canon" -Model="Canon EOS R5" -LensModel="RF 50mm F1.2L USM" \
  -FocalLength="50.0 mm" -FNumber=4.0 -ISO=200 -ExposureTime="1/1000" \
  -DateTimeOriginal="$d10" -CreateDate="$d10" \
  -GPSLatitude=34.0259 -GPSLatitudeRef=N -GPSLongitude=118.7798 -GPSLongitudeRef=W \
  "$PHOTOS_DIR/hero-shot-beach.jpg"

exiftool -overwrite_original -q \
  -Make="Canon" -Model="Canon EOS R5" -LensModel="RF 85mm F1.2L USM" \
  -FocalLength="85.0 mm" -FNumber=2.8 -ISO=400 -ExposureTime="1/200" \
  -DateTimeOriginal="$d9" -CreateDate="$d9" \
  "$PHOTOS_DIR/hero-shot-studio.jpg"

exiftool -overwrite_original -q \
  -Make="Sony" -Model="Sony A7 IV" -LensModel="FE 24-70mm F2.8 GM II" \
  -FocalLength="24.0 mm" -FNumber=5.6 -ISO=320 -ExposureTime="1/2000" \
  -DateTimeOriginal="$d8" -CreateDate="$d8" \
  -GPSLatitude=36.9741 -GPSLatitudeRef=N -GPSLongitude=122.0308 -GPSLongitudeRef=W \
  "$PHOTOS_DIR/lifestyle-01.jpg"

exiftool -overwrite_original -q \
  -Make="Sony" -Model="Sony A7 IV" -LensModel="FE 35mm F1.4 GM" \
  -FocalLength="35.0 mm" -FNumber=4.0 -ISO=250 -ExposureTime="1/500" \
  -DateTimeOriginal="$d7" -CreateDate="$d7" \
  "$PHOTOS_DIR/lifestyle-02.jpg"

exiftool -overwrite_original -q \
  -Make="Sony" -Model="Sony A7 IV" -LensModel="FE 50mm F1.2 GM" \
  -FocalLength="50.0 mm" -FNumber=2.0 -ISO=800 -ExposureTime="1/125" \
  -DateTimeOriginal="$d6" -CreateDate="$d6" \
  "$PHOTOS_DIR/lifestyle-03.jpg"

echo
echo "Done. internal/demo/assets/{photos,videos}/ updated:"
du -sh "$PHOTOS_DIR" "$VIDEOS_DIR"
echo
echo "Next: review the files, then update internal/demo/assets/manifest.yaml"
echo "and ATTRIBUTION.md if you changed what any slot contains."
