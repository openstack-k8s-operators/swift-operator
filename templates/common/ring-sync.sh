#!/bin/sh
TARFILE="/var/lib/config-data/rings/swiftrings.tar.gz"
MTIME="0"

while true; do
    if [ -e $TARFILE ] ; then
        _MTIME=$(stat -L --printf "%Y" $TARFILE)
        if [ $MTIME != $_MTIME ]; then
            tar -xvzf $TARFILE -C etc/swift/
        fi
        MTIME=$_MTIME
    fi
    sleep 60
done
