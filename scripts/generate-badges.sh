#!/usr/bin/env bash

set -euo pipefail

readonly goreportcard_revision="af15decf135bcd0a1bfa12180d027bdeb19f601c"
readonly gometalinter_version="v3.0.0+incompatible"
readonly kingpin_revision="63abe20a23e29e80bbef8089bd3dee3ac25e5306"
readonly shlex_revision="6f45313302b9c56850fc17f99e40caebce98c716"
readonly gocyclo_version="v0.6.0"
readonly misspell_version="v0.3.4"

readonly repository_root="$(git rev-parse --show-toplevel)"
readonly tools_dir="$(mktemp -d)"
trap 'rm -rf "${tools_dir}"' EXIT

mkdir -p "${tools_dir}/bin" "${tools_dir}/build" "${repository_root}/docs"

GOBIN="${tools_dir}/bin" go install "github.com/fzipp/gocyclo/cmd/gocyclo@${gocyclo_version}"
GOBIN="${tools_dir}/bin" go install "github.com/client9/misspell/cmd/misspell@${misspell_version}"
GOBIN="${tools_dir}/bin" go install "github.com/gojp/goreportcard/cmd/goreportcard-cli@${goreportcard_revision}"

pushd "${tools_dir}/build" >/dev/null
go mod init local/gometalinter-build >/dev/null
go get "github.com/alecthomas/gometalinter@${gometalinter_version}" >/dev/null
go get "gopkg.in/alecthomas/kingpin.v3-unstable@${kingpin_revision}" >/dev/null
go get "github.com/google/shlex@${shlex_revision}" >/dev/null
go build -o "${tools_dir}/bin/gometalinter" github.com/alecthomas/gometalinter
popd >/dev/null

report="$({ PATH="${tools_dir}/bin:${PATH}" "${tools_dir}/bin/goreportcard-cli" -d "${repository_root}"; } 2>/dev/null)"
grade="$(awk '/^Grade / { print $(NF - 1) }' <<<"${report}")"
score="$(awk '/^Grade / { print $NF }' <<<"${report}")"
if [[ -z "${grade}" || -z "${score}" ]]; then
  echo "could not parse goreportcard result" >&2
  exit 1
fi

case "${grade}" in
  A+) quality_color="#4c1" ;;
  A) quality_color="#97ca00" ;;
  B) quality_color="#dfb317" ;;
  C) quality_color="#fe7d37" ;;
  *) quality_color="#e05d44" ;;
esac

loc="$({
  git -C "${repository_root}" ls-files '*.go' |
    awk '!/(^|\/)internal\/gen\// && !/(^|\/)mock_.*\.go$/ && !/(^|\/)wire_gen\.go$/ && !/(^|\/)querier_metrics_gen\.go$/' |
    while IFS= read -r source_file; do
      wc -l < "${repository_root}/${source_file}"
    done
} | awk '{ total += $1 } END { print total + 0 }')"
loc_compact="$(awk -v lines="${loc}" 'BEGIN { if (lines >= 1000) printf "%.1fk", lines / 1000; else print lines }')"

render_badge() {
  local output_file="$1"
  local label="$2"
  local value="$3"
  local color="$4"
  local label_width="$5"
  local value_width="$6"
  local total_width=$((label_width + value_width))
  local label_center=$((label_width / 2))
  local value_center=$((label_width + value_width / 2))

  cat > "${output_file}" <<EOF
<svg xmlns="http://www.w3.org/2000/svg" role="img" aria-label="${label}: ${value}" width="${total_width}" height="20">
  <title>${label}: ${value}</title>
  <linearGradient id="s" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>
  <clipPath id="r"><rect width="${total_width}" height="20" rx="3" fill="#fff"/></clipPath>
  <g clip-path="url(#r)"><rect width="${label_width}" height="20" fill="#555"/><rect x="${label_width}" width="${value_width}" height="20" fill="${color}"/><rect width="${total_width}" height="20" fill="url(#s)"/></g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">
    <text x="${label_center}" y="15" fill="#010101" fill-opacity=".3">${label}</text><text x="${label_center}" y="14">${label}</text>
    <text x="${value_center}" y="15" fill="#010101" fill-opacity=".3">${value}</text><text x="${value_center}" y="14">${value}</text>
  </g>
</svg>
EOF
}

render_badge "${repository_root}/docs/goreportcard.svg" "Go Report Card" "${grade} ${score}" "${quality_color}" 92 64
render_badge "${repository_root}/docs/loc.svg" "handwritten LOC" "${loc_compact}" "#007ec6" 96 42

printf 'Go Report Card: %s %s\nHandwritten Go LOC: %s\n' "${grade}" "${score}" "${loc}"
