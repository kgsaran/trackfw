#!/usr/bin/env bash
# capture-barrier-baseline.sh — capture `trackfw barrier --json` output for every
# (roadmap, wave) pair in done/ and wip/, masking timestamps.
#
# Usage:
#   bash scripts/capture-barrier-baseline.sh <binary> <output-file>
#
# <binary>      path to the trackfw binary (e.g. /tmp/trackfw-baseline)
# <output-file> where to write the baseline (e.g. internal/roadmapdoc/testdata/barrier-baseline.txt)
#
# Record format:
#   PATH=<path> WAVE=<label> RC=<code>
#   STDOUT: <json with timestamps masked>
#   STDERR: <stderr content>
#   ---
#
# Timestamps (started_at, finished_at) are masked to "MASKED" for stable comparison.

BINARY="${1:?first argument: path to binary}"
OUTFILE="${2:?second argument: output file path}"

ROADMAP_DIRS=(
    "docs/roadmaps/done"
    "docs/roadmaps/wip"
)

# Collect all roadmap files, sorted deterministically
mapfile -t ROADMAPS < <(
    for d in "${ROADMAP_DIRS[@]}"; do
        find "$d" -maxdepth 1 -name "*.md" -type f 2>/dev/null
    done | LC_ALL=C sort
)

> "$OUTFILE"

STDOUT_TMP="/tmp/barrier-baseline-stdout-$$"
STDERR_TMP="/tmp/barrier-baseline-stderr-$$"

for roadmap in "${ROADMAPS[@]}"; do
    # Extract wave labels from headings matching "## Wave <label> "
    mapfile -t WAVES < <(
        /usr/bin/grep -E '^## Wave [0-9]' "$roadmap" \
            | sed 's/^## Wave \([^ ]*\) .*/\1/' \
            | LC_ALL=C sort -u
    )

    if [ ${#WAVES[@]} -eq 0 ]; then
        printf 'PATH=%s WAVE=__NO_WAVE__ RC=SKIP\nSTDOUT: \nSTDERR: no wave headings\n---\n' \
            "$roadmap" >> "$OUTFILE"
        continue
    fi

    for wave in "${WAVES[@]}"; do
        # Run, capturing stdout and stderr separately; capture exit code without failing
        "$BINARY" barrier "$roadmap" --wave "$wave" --json --trust-local-gates \
            > "$STDOUT_TMP" 2> "$STDERR_TMP" || true
        rc=$?

        stdout=$(cat "$STDOUT_TMP")
        stderr=$(cat "$STDERR_TMP")

        # Mask timestamps for stable comparison
        masked=$(printf '%s' "$stdout" | sed \
            -e 's/"started_at":"[^"]*"/"started_at":"MASKED"/g' \
            -e 's/"finished_at":"[^"]*"/"finished_at":"MASKED"/g')

        printf 'PATH=%s WAVE=%s RC=%d\nSTDOUT: %s\nSTDERR: %s\n---\n' \
            "$roadmap" "$wave" "$rc" "$masked" "$stderr" >> "$OUTFILE"
    done
done

rm -f "$STDOUT_TMP" "$STDERR_TMP"

line_count=$(wc -l < "$OUTFILE" | tr -d ' ')
echo "Baseline captured: $OUTFILE (${line_count} lines)"
