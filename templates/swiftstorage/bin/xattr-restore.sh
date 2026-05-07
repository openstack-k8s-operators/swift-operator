#!/bin/sh
set -e
DUMP="/srv/node/pv/.swift-xattrs.dump"
if [ -f "$DUMP" ]; then
    echo "xattr dump found, restoring extended attributes..."
    cd /
    setfattr --restore="$DUMP"
    echo "xattr restore complete, validating..."
    MISSING="/srv/node/pv/.swift-xattrs.missing"
    find /srv/node/pv/objects/ -name "*.data" 2>/dev/null | \
        while read f; do
            getfattr -n user.swift.metadata "$f" >/dev/null 2>&1 || echo "$f"
        done > "$MISSING"
    COUNT=$(wc -l < "$MISSING")
    if [ "$COUNT" -gt 0 ]; then
        echo "WARNING: $COUNT files without xattrs after restore"
        echo "Details written to $MISSING"
        echo "These objects may be quarantined and recovered by the replicator"
    else
        rm -f "$MISSING"
        echo "validation passed - all files have xattrs"
    fi
    mv "$DUMP" "${DUMP}.applied"
    echo "dump file renamed to ${DUMP}.applied"
else
    echo "No xattr dump found, skipping restore"
fi
