#!/bin/sh
# Copy the newest Archify render of each diagram into architecture/.
#
# Usage: scripts/copy-diagrams.sh [archify-dir] [architecture-dir]
#
# Archify writes one folder per render, <archify-dir>/<type>-<slug>-<YYYYMMDD-HHMMSS>/.
# For each slug, this copies the newest folder's files under stable names:
#
#   candidate.json     -> <architecture-dir>/<slug>.json
#   <slug>.html        -> <architecture-dir>/<slug>.html
#   <slug>-light.svg   -> <architecture-dir>/<slug>-light.svg
#   <slug>-dark.svg    -> <architecture-dir>/<slug>-dark.svg
#
# The SVGs come from scripts/export-diagram-svg.mjs. This script never runs
# Archify. It checks every newest render first and copies nothing if one is
# incomplete.
set -eu

src=${1:-.archify}
dst=${2:-architecture}

fail() {
	echo "make diagrams: $*" >&2
	exit 1
}

[ -d "$src" ] || fail "$src/ not found. It holds Archify renders, so this needs Archify; see architecture/README.md."

# One line per render folder: "<slug> <timestamp> <folder>". Sorting by slug,
# then timestamp, puts each slug's newest render last.
renders=$(
	for dir in "$src"/*/; do
		[ -d "$dir" ] || continue
		name=$(basename "$dir")
		slug=$(printf '%s\n' "$name" | sed -n 's/^[a-z][a-z]*-\(.*\)-\([0-9]\{8\}-[0-9]\{6\}\)$/\1/p')
		stamp=$(printf '%s\n' "$name" | sed -n 's/^[a-z][a-z]*-\(.*\)-\([0-9]\{8\}-[0-9]\{6\}\)$/\2/p')
		[ -n "$slug" ] && printf '%s %s %s\n' "$slug" "$stamp" "$name"
	done | sort -k1,1 -k2,2 | awk '{ newest[$1] = $3 } END { for (s in newest) print s, newest[s] }' | sort
)

[ -n "$renders" ] || fail "no <type>-<slug>-<YYYYMMDD-HHMMSS>/ render folders in $src/."

missing=$(
	printf '%s\n' "$renders" | while read -r slug name; do
		for f in candidate.json "$slug.html" "$slug-light.svg" "$slug-dark.svg"; do
			[ -f "$src/$name/$f" ] || printf '  %s\n' "$src/$name/$f"
		done
	done
)
[ -z "$missing" ] || fail "missing files (run Archify finalize, then node scripts/export-diagram-svg.mjs <folder>):
$missing"

mkdir -p "$dst"
printf '%s\n' "$renders" | while read -r slug name; do
	cp "$src/$name/candidate.json" "$dst/$slug.json"
	cp "$src/$name/$slug.html" "$src/$name/$slug-light.svg" "$src/$name/$slug-dark.svg" "$dst/"
	echo "$slug: copied from $src/$name"
done
