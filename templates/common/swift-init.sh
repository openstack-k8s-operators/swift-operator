#!/bin/sh
TARFILE=/var/lib/config-data/rings/swiftrings.tar.gz

cp -t /etc/swift/ /var/lib/config-data/default/* /var/lib/config-data/swiftconf/*

cd /etc/swift

if [ ! -f $TARFILE ]; then
	echo "$TARFILE not found - creating dummy Swift rings"
	for f in account.builder container.builder object.builder; do
		swift-ring-builder $f create 1 1 1
		swift-ring-builder $f add --region 1 --zone 1 --ip 127.0.0.1 --port 0 --device dummy --weight 1
		swift-ring-builder $f rebalance
	done
else
	tar xvzf $TARFILE
fi
