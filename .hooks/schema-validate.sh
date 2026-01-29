#!/usr/bin/env bash
#
# Validate that the generated JSON schema is in sync with the Go structs.
# This hook prevents committing changes to pkg/builder/ without updating the schema.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Validating JSON schema is in sync with Go structs..."

# Check if schema-related files were modified
MODIFIED_FILES="${*:-}"
SCHEMA_NEEDS_REGEN=false

# Check if any Go files in pkg/builder/ were modified
if echo "$MODIFIED_FILES" | grep -q "pkg/builder/.*\.go"; then
	SCHEMA_NEEDS_REGEN=true
fi

# Check if schema generator was modified
if echo "$MODIFIED_FILES" | grep -q "cmd/schema-gen/"; then
	SCHEMA_NEEDS_REGEN=true
fi

# If no relevant files were modified, skip validation
if [ "$SCHEMA_NEEDS_REGEN" = false ]; then
	echo -e "${GREEN}✓${NC} No schema-related changes detected"
	exit 0
fi

# Regenerate schema to temporary location
TEMP_SCHEMA=$(mktemp)
trap 'rm -f "$TEMP_SCHEMA"' EXIT

echo "Regenerating schema to verify it's in sync..."
go run cmd/schema-gen/main.go -o "$TEMP_SCHEMA" >/dev/null

# Compare with committed schema
if ! diff -q "$TEMP_SCHEMA" schema/warpgate-template.json >/dev/null 2>&1; then
	echo -e "${RED}✗${NC} JSON schema is out of sync with Go structs!"
	echo ""
	echo -e "${YELLOW}The schema file needs to be regenerated.${NC}"
	echo "Run the following command to fix:"
	echo ""
	echo "  task schema:generate"
	echo ""
	echo "Then stage the updated schema file:"
	echo ""
	echo "  git add schema/warpgate-template.json"
	echo ""
	exit 1
fi

echo -e "${GREEN}✓${NC} JSON schema is in sync"
exit 0
