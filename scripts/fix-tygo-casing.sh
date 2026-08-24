#!/bin/bash
# Converts PascalCase TypeScript interface fields to camelCase in Tygo-generated files
# This matches the runtime behavior of StructToMap() in Go
#
# Pattern: "  FieldName:" or "  FieldName?:" -> "  fieldName:" or "  fieldName?:"
# Special handling for:
#   - ID -> id (not iD)
#   - Single letter fields like X, Y, Z -> x, y, z

set -e

TARGET_FILES=(
    "src/net/generated/models.ts"
    "src/net/generated/world.ts"
    "src/net/generated/world_api.ts"
    "src/net/generated/world_db.ts"
    "src/net/generated/world_interface.ts"
)

COUNT=0
for TARGET_FILE in "${TARGET_FILES[@]}"; do
    if [ ! -f "$TARGET_FILE" ]; then
        # Silent skip or warning if file doesn't exist
        continue
    fi

    # Use perl for in-place editing with proper regex
    # Step 1: Convert first letter to lowercase for multi-char fields
    # Step 2: Fix "iD" -> "id" (ID special case)
    perl -i -pe '
        # Match interface fields: leading whitespace, capital letter(s), rest of name, optional ?, colon
        s/^(\s+)([A-Z])([a-zA-Z0-9_]*)(\??:)/\1\L$2\E$3$4/g;
        # Fix iD at start -> id
        s/^(\s+)iD(\??:)/\1id$2/g;
        # Fix ID suffix -> Id (so spellID becomes spellId)
        s/ID(\??:)/Id$1/g;
    ' "$TARGET_FILE"

    # Tygo emits a trailing blank line for source sections with declarations
    # that do not map to TypeScript. Keep generated output byte-stable so
    # starting the development server does not dirty a clean checkout.
    perl -0pi -e 's/[ \t\r\n]+\z/\n/' "$TARGET_FILE"

    COUNT=$((COUNT + 1))
done

if [ $COUNT -gt 0 ]; then
    echo "✓ Fixed PascalCase -> camelCase in $COUNT generated files"
fi
