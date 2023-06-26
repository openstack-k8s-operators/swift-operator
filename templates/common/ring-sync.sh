#!/bin/sh
TARFILE="/var/lib/config-data/rings/swiftrings.tar.gz"
MTIME="0"

# This is done only initially, should be done by Kolla probably
cp -t /etc/swift/ /var/lib/config-data/default/* /var/lib/config-data/swiftconf/*

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
