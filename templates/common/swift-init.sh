#!/bin/sh
cp -t /etc/swift/ /var/lib/config-data/default/* /var/lib/config-data/swiftconf/* /var/lib/config-data/rings/*

cd /etc/swift

for f in account.builder container.builder object.builder; do
	if [ ! -e $f ]; then
		swift-ring-builder $f create 1 1 1
		swift-ring-builder $f add --region 1 --zone 1 --ip 127.0.0.1 --port 0 --device dummy --weight 1
		swift-ring-builder $f rebalance
	fi
done
