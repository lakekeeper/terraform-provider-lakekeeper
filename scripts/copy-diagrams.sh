#!/bin/sh
# Copy the newest Archify render of each diagram into architecture/.
#
# Usage: scripts/copy-diagrams.sh [archify-dir] [architecture-dir]
#
# Archify writes one folder per render, <archify-dir>/<type>-<slug>-<YYYYMMDD-HHMMSS>/,
# where <type> is an Archify diagram type. For each slug, this copies the newest
# folder's files under stable names:
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

# Archify's diagram types (see `archify --help`), longest first so the longest
# matching type wins. Keep in sync with ARCHIFY_TYPES in
# scripts/export-diagram-svg.mjs.
types="architecture waterfall lifecycle sequence dataflow workflow timeline class tree erd"

fail() {
	echo "make diagrams: $*" >&2
	exit 1
}

# render_files <slug> prints a render's files, one per line.
render_files() {
	printf '%s\n' candidate.json "$1.html" "$1-light.svg" "$1-dark.svg"
}

# parse <folder name> prints "<slug> <timestamp>": the name without its leading
# Archify type and trailing timestamp. It prints nothing for any other name.
parse() {
	stamp=$(printf '%s\n' "$1" | sed -n 's/^.*-\([0-9]\{8\}-[0-9]\{6\}\)$/\1/p')
	[ -n "$stamp" ] || return 0
	rest=${1%-"$stamp"}
	for type in $types; do
		slug=${rest#"$type"-}
		if [ "$slug" != "$rest" ] && [ -n "$slug" ]; then
			printf '%s %s\n' "$slug" "$stamp"
			return 0
		fi
	done
}

[ -d "$src" ] || fail "$src/ not found. It holds Archify renders, so this needs Archify; see architecture/README.md."

# One line per render folder: "<slug> <timestamp> <folder>". Sorting by slug,
# then timestamp, puts each slug's newest render last.
renders=$(
	for dir in "$src"/*/; do
		[ -d "$dir" ] || continue
		name=$(basename "$dir")
		parsed=$(parse "$name")
		if [ -n "$parsed" ]; then
			printf '%s %s\n' "$parsed" "$name"
		else
			echo "make diagrams: skipping $src/$name/: not <type>-<slug>-<YYYYMMDD-HHMMSS> with an Archify type" >&2
		fi
	done | sort -k1,1 -k2,2 | awk '{ newest[$1] = $3 } END { for (s in newest) print s, newest[s] }' | sort
)

[ -n "$renders" ] || fail "no <type>-<slug>-<YYYYMMDD-HHMMSS>/ render folders in $src/."

missing=$(
	printf '%s\n' "$renders" | while read -r slug name; do
		render_files "$slug" | while read -r f; do
			[ -f "$src/$name/$f" ] || printf '  %s\n' "$src/$name/$f"
		done
	done
)
[ -z "$missing" ] || fail "missing files (run Archify finalize, then node scripts/export-diagram-svg.mjs <folder>):
$missing"

mkdir -p "$dst"
printf '%s\n' "$renders" | while read -r slug name; do
	render_files "$slug" | while read -r f; do
		if [ "$f" = candidate.json ]; then target=$slug.json; else target=$f; fi
		cp "$src/$name/$f" "$dst/$target"
	done
	echo "$slug: copied from $src/$name"
done
