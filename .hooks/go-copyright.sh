#!/bin/bash
set -ex

copyright_line='// Copyright 2026 National Technology & Engineering Solutions of Sandia, LLC (NTESS).'

echo "Starting copyright check..."

update_copyright() {
	local file="$1"
	local temp_file
	local first_nonempty

	temp_file=$(mktemp)
	first_nonempty=$(awk 'NF {print NR; exit}' "$file")

	if [[ -n "$first_nonempty" ]]; then
		head -n "$((first_nonempty - 1))" "$file" >"$temp_file"
	fi

	echo "${copyright_line}" >>"$temp_file"
	echo "" >>"$temp_file"

	if [[ -n "$first_nonempty" ]]; then
		tail -n "+${first_nonempty}" "$file" | sed '1{/^\/\/ Copyright /d;}' >>"$temp_file"
	fi

	mv "${temp_file}" "${file}"
}

# Get the list of staged .go files
staged_files=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)

# Check if there are any staged .go files
if [[ -z "$staged_files" ]]; then
	echo "No .go files staged for commit. Exiting."
	exit 0
fi

for file in $staged_files; do
	echo "Checking file: $file"
	if grep -qF "${copyright_line}" "$file"; then
		echo "Current copyright header is up-to-date in $file"
	else
		echo "Updating copyright header in $file"
		update_copyright "$file"
		echo "Copyright header updated in $file"
	fi
done

echo "Copyright check completed."
