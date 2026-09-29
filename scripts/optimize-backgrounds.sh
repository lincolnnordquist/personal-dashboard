#!/usr/bin/env bash
# Optimizes background theme videos so they're cheap to play all day.
#
# For every video in backgrounds/ (or just the themes/files you name), it:
#   - converts it to H.264 MP4 (the format GPUs decode in hardware), fast-start for streaming
#   - caps the frame rate at 30 fps
#   - removes the audio track (backgrounds are always muted)
#   - keeps <theme>/posters/ in step: creates a still for each video (or refreshes it when the
#     video was replaced), and removes stills whose video is gone
# Resolution is never changed: add clips at the resolution you want them shown at.
# A video that only needs its audio removed is copied without re-encoding, so it loses no
# quality. Videos that already meet all of the above are left untouched, so it's safe to
# re-run whenever you add clips. Originals are replaced; committed ones are still in git.
#
# Usage: scripts/optimize-backgrounds.sh [--dry-run] [theme-folder-or-video ...]
#   scripts/optimize-backgrounds.sh                  # everything in backgrounds/
#   scripts/optimize-backgrounds.sh mario            # one theme
#   scripts/optimize-backgrounds.sh --dry-run        # just report what would change
set -euo pipefail

MAX_FPS=30
CRF=20          # x264 quality when re-encoding: lower is better and bigger; 18-24 is the useful range
DRY_RUN=false
root="$(cd "$(dirname "$0")/.." && pwd)/backgrounds"

targets=()
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    -h | --help) sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
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
  codec="" rate="" pix_fmt=""
  while IFS== read -r key value; do
    case "$key" in
      codec_name) codec=$value ;;
      avg_frame_rate) rate=$value ;;
      pix_fmt) pix_fmt=$value ;;
    esac
  done < <(ffprobe -v error -select_streams v:0 \
    -show_entries stream=codec_name,avg_frame_rate,pix_fmt -of default=noprint_wrappers=1 "$video")
  fps=$(awk -F/ '{ printf "%.2f", ($2 > 0 ? $1 / $2 : $1) }' <<<"$rate")
  has_audio=$(ffprobe -v error -select_streams a -show_entries stream=index -of csv=p=0 "$video" | head -1)

  # Changes that need the video re-encoded, and ones that don't.
  reencode=()
  [[ "$codec" != "h264" ]] && reencode+=("$codec → h264")
  [[ "$pix_fmt" != "yuv420p" ]] && reencode+=("$pix_fmt → yuv420p")
  awk -v f="$fps" -v m="$MAX_FPS" 'BEGIN { exit !(f > m + 0.5) }' && reencode+=("${fps%.*} → ${MAX_FPS} fps")
  reasons=("${reencode[@]}")
  [[ "${video##*.}" != "mp4" ]] && reasons+=("${video##*.} → mp4")
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
    if [[ ${#reencode[@]} -gt 0 ]]; then
      ffmpeg -v error -y -i "$video" -map 0:v:0 -an -vf "fps=fps='min(source_fps,$MAX_FPS)'" \
        -c:v libx264 -preset slow -crf "$CRF" -profile:v high -pix_fmt yuv420p \
        -movflags +faststart "$tmp"
    else
      # The video stream is already fine: copy it as-is, just without the audio.
      ffmpeg -v error -y -i "$video" -map 0:v:0 -an -c:v copy -movflags +faststart "$tmp"
    fi
    mv "$tmp" "$out"
    [[ "$video" != "$out" ]] && rm "$video"
    after=$(stat -c %s "$out")
    total_after=$((total_after + after))
    changed=$((changed + 1))
    echo "fixed     $rel ($(IFS=,; echo "${reasons[*]}" | sed 's/,/, /g')): $(human "$before") → $(human "$after")"
  fi

  # A poster still (used for reduced motion and while a video loads), from 2s in. It is
  # (re)made when missing or older than the video, e.g. after replacing a clip.
  poster="$dir/posters/$stem.jpg"
  src="$out"; [[ -e "$src" ]] || src="$video"
  if [[ ! -e "$poster" || "$src" -nt "$poster" ]]; then
    if $DRY_RUN; then
      echo "          would make poster ${poster#"$root"/}"
    else
      mkdir -p "$dir/posters"
      ffmpeg -v error -y -ss 2 -i "$src" -frames:v 1 -q:v 3 "$poster"
      echo "          + poster ${poster#"$root"/}"
    fi
  fi
done

# Remove posters left behind by deleted or renamed videos.
while IFS= read -r poster; do
  posters_dir=$(dirname "$poster")
  stem=$(basename "${poster%.*}")
  has_video=false
  for ext in mp4 webm mov mkv m4v MP4 WEBM MOV MKV M4V; do
    [[ -e "$(dirname "$posters_dir")/$stem.$ext" ]] && has_video=true
  done
  if ! $has_video; then
    if $DRY_RUN; then
      echo "would remove orphaned poster ${poster#"$root"/}"
    else
      rm "$poster"
      echo "removed orphaned poster ${poster#"$root"/}"
    fi
  fi
done < <(find "${targets[@]}" -path '*/posters/*' -type f \( -iname '*.jpg' -o -iname '*.jpeg' -o -iname '*.png' -o -iname '*.webp' \) | sort)

echo
if $DRY_RUN; then
  echo "Dry run: nothing was changed."
else
  echo "$changed of ${#videos[@]} videos updated. Total: $(human "$total_before") → $(human "$total_after")."
fi
