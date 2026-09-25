#!/usr/bin/env bash
# Optimizes background theme videos so they're cheap to play all day.
#
# For every video in backgrounds/ (or just the themes/files you name), it:
#   - re-encodes to H.264 MP4 (the format GPUs decode in hardware), fast-start for streaming
#   - caps the frame rate at 30 fps and the height at 720p (or 1080p with --1080p)
#   - removes the audio track (backgrounds are always muted)
#   - creates a poster still in <theme>/posters/ if there isn't one
# Videos that already meet all of that are left untouched, so it's safe to re-run whenever
# you add clips. Originals are replaced; committed ones can be recovered from git.
#
# Usage: scripts/optimize-backgrounds.sh [--1080p] [--dry-run] [theme-folder-or-video ...]
#   scripts/optimize-backgrounds.sh                  # everything in backgrounds/
#   scripts/optimize-backgrounds.sh mario            # one theme
#   scripts/optimize-backgrounds.sh --dry-run        # just report what would change
set -euo pipefail

MAX_HEIGHT=720
MAX_FPS=30
CRF=22          # x264 quality: lower is better and bigger; 18-24 is the useful range
DRY_RUN=false
root="$(cd "$(dirname "$0")/.." && pwd)/backgrounds"

targets=()
for arg in "$@"; do
  case "$arg" in
    --1080p) MAX_HEIGHT=1080 ;;
    --dry-run) DRY_RUN=true ;;
    -h | --help) sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)
      if [[ -e "$arg" ]]; then targets+=("$arg")
      elif [[ -e "$root/$arg" ]]; then targets+=("$root/$arg")
      else echo "Not found: $arg" >&2; exit 1
      fi ;;
  esac
done
[[ ${#targets[@]} -eq 0 ]] && targets=("$root")

for tool in ffmpeg ffprobe; do
  command -v "$tool" >/dev/null || { echo "$tool is required (e.g. pacman -S ffmpeg)" >&2; exit 1; }
done

human() { numfmt --to=iec --suffix=B "$1"; }

# Lists videos under the targets, skipping the posters folders.
mapfile -t videos < <(find "${targets[@]}" -type f \
  \( -iname '*.mp4' -o -iname '*.webm' -o -iname '*.mov' -o -iname '*.mkv' -o -iname '*.m4v' \) \
  -not -path '*/posters/*' -not -name '.*' | sort)
if [[ ${#videos[@]} -eq 0 ]]; then
  echo "No videos found in ${targets[*]}"
  exit 0
fi

total_before=0 total_after=0 changed=0
for video in "${videos[@]}"; do
  dir=$(dirname "$video")
  stem=$(basename "${video%.*}")
  out="$dir/$stem.mp4"
  rel=${video#"$root"/}

  # Current properties of the first video stream. ffprobe prints fields in its own order,
  # so read them by name.
  codec="" height=0 rate="" pix_fmt=""
  while IFS== read -r key value; do
    case "$key" in
      codec_name) codec=$value ;;
      height) height=$value ;;
      avg_frame_rate) rate=$value ;;
      pix_fmt) pix_fmt=$value ;;
    esac
  done < <(ffprobe -v error -select_streams v:0 \
    -show_entries stream=codec_name,height,avg_frame_rate,pix_fmt -of default=noprint_wrappers=1 "$video")
  fps=$(awk -F/ '{ printf "%.2f", ($2 > 0 ? $1 / $2 : $1) }' <<<"$rate")
  has_audio=$(ffprobe -v error -select_streams a -show_entries stream=index -of csv=p=0 "$video" | head -1)

  reasons=()
  [[ "${video##*.}" != "mp4" ]] && reasons+=("${video##*.} → mp4")
  [[ "$codec" != "h264" ]] && reasons+=("$codec → h264")
  [[ "$pix_fmt" != "yuv420p" ]] && reasons+=("$pix_fmt → yuv420p")
  (( height > MAX_HEIGHT )) && reasons+=("${height}p → ${MAX_HEIGHT}p")
  awk -v f="$fps" -v m="$MAX_FPS" 'BEGIN { exit !(f > m + 0.5) }' && reasons+=("${fps%.*} → ${MAX_FPS} fps")
  [[ -n "$has_audio" ]] && reasons+=("drop audio")

  before=$(stat -c %s "$video")
  total_before=$((total_before + before))

  if [[ ${#reasons[@]} -eq 0 ]]; then
    echo "ok        $rel"
    total_after=$((total_after + before))
  elif $DRY_RUN; then
    echo "would fix $rel ($(IFS=,; echo "${reasons[*]}" | sed 's/,/, /g'))"
    total_after=$((total_after + before))
  else
    tmp="$dir/.$stem.optimizing.mp4"
    filters="fps=fps='min(source_fps,$MAX_FPS)'"
    (( height > MAX_HEIGHT )) && filters="scale=-2:$MAX_HEIGHT:flags=lanczos,$filters"
    ffmpeg -v error -y -i "$video" -map 0:v:0 -an -vf "$filters" \
      -c:v libx264 -preset slow -crf "$CRF" -profile:v high -pix_fmt yuv420p \
      -movflags +faststart "$tmp"
    mv "$tmp" "$out"
    [[ "$video" != "$out" ]] && rm "$video"
    after=$(stat -c %s "$out")
    total_after=$((total_after + after))
    changed=$((changed + 1))
    echo "fixed     $rel ($(IFS=,; echo "${reasons[*]}" | sed 's/,/, /g')): $(human "$before") → $(human "$after")"
  fi

  # A poster still (used for reduced motion and while a video loads), from 2s in.
  poster="$dir/posters/$stem.jpg"
  if [[ ! -e "$poster" ]] && ! $DRY_RUN; then
    mkdir -p "$dir/posters"
    src="$out"; [[ -e "$src" ]] || src="$video"
    ffmpeg -v error -y -ss 2 -i "$src" -frames:v 1 -vf "scale=-2:$MAX_HEIGHT" -q:v 3 "$poster"
    echo "          + poster ${poster#"$root"/}"
  fi
done

echo
if $DRY_RUN; then
  echo "Dry run: nothing was changed."
else
  echo "$changed of ${#videos[@]} videos re-encoded. Total: $(human "$total_before") → $(human "$total_after")."
fi
