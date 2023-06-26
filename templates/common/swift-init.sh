#!/bin/sh
# NOTE: a few more functions will be added to this script, thus keeping it for now
cp -t /etc/swift/ /var/lib/config-data/default/* /var/lib/config-data/swiftconf/*

tar xvzf /var/lib/config-data/rings/swiftrings.tar.gz -C /etc/swift
