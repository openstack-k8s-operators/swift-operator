#!/bin/sh
TARFILE="/var/lib/config-data/rings/swiftrings.tar.gz"
MTIME="0"

# This is done only initially, will be done by Kolla eventually
mkdir /etc/swift/account-server.conf.d /etc/swift/container-server.conf.d /etc/swift/object-server.conf.d /etc/swift/object-expirer.conf.d /etc/swift/proxy-server.conf.d

cp -t /etc/swift/ /var/lib/config-data/swiftconf/swift.conf /var/lib/config-data/default/internal-client.conf /var/lib/config-data/default/rsyncd.conf /var/lib/config-data/default/dispersion.conf /var/lib/config-data/default/keymaster.conf

cp -t /etc/swift/account-server.conf.d/ /var/lib/config-data/default/*account-server*.conf
cp -t /etc/swift/container-server.conf.d/ /var/lib/config-data/default/*container-server*.conf
cp -t /etc/swift/object-server.conf.d/ /var/lib/config-data/default/*object-server*.conf
cp -t /etc/swift/object-expirer.conf.d/ /var/lib/config-data/default/*object-expirer*.conf
cp -t /etc/swift/proxy-server.conf.d/ /var/lib/config-data/default/*proxy-server*.conf

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
