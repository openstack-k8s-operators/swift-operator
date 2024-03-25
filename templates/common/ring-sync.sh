#!/bin/sh
TARFILE="/var/lib/config-data/rings/swiftrings.tar.gz"
MTIME="0"

for f in /var/lib/config-data/swiftconf/swift.conf \
    /var/lib/config-data/default/internal-client.conf \
    /var/lib/config-data/default/rsyncd.conf \
    /var/lib/config-data/default/dispersion.conf \
    /var/lib/config-data/default/keymaster.conf; do
    [ -e $f ] && cp -t /etc/swift/ $f
done

for s in account-server \
    container-server \
    object-server \
    object-expirer \
    proxy-server; do
    if $(ls -1 /var/lib/config-data/default/ | grep -q "${s}"); then
        [ ! -d  /etc/swift/${s}.conf.d ] && mkdir /etc/swift/${s}.conf.d
        cp -t /etc/swift/${s}.conf.d/ /var/lib/config-data/default/*${s}*.conf
    fi
done

# This loop checks every 60 seconds if the ring files from the configmap got
# updated and if so, extracts them /etc/swift. Swift services notice the change
# and reload the ring files, thus no pod restart required
while true; do
    if [ -e $TARFILE ] ; then
        _MTIME=$(stat -L --printf "%Y" $TARFILE)
        if [ $MTIME != $_MTIME ]; then
            tar -xzf $TARFILE -C etc/swift/
            find /etc/swift/ -type f -ls
            echo
        fi
        MTIME=$_MTIME
    fi
    sleep 60
done
