#!/bin/sh
# NOTE: this is more a POC with some basic functionality. It will be replaced
# by a Python script that directly uses swift.common.ring.builder and
# python-requests and compares existing rings with given devices, properly
# manages these and also checks replication status to decide if rebalancing is
# safe.

# TODO: this should be set properly according to the number of devices
PART_POWER=8
MIN_PART_HOURS=1
TARFILE="/tmp/swiftrings.tar.gz"
BASE_URL="https://kubernetes.default.svc/api/v1/namespaces/${NAMESPACE}/configmaps"
URL="${BASE_URL}/${CM_NAME}"
METHOD="PUT"

# Credentials to be used by curl
export CURL_CA_BUNDLE=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)

cp -t /etc/swift/ /var/lib/config-data/swiftconf/*

# Get the ConfigMap with the Swiftrings if it exists. If it exists, untar it
# and update the rings. If it does not exist, create a new ConfigMap using a
# POST request. If the response code is neither 200 or 404 fail early, b/c it
# is unclear if there might be an existing set of rings that should not be
# overwritten
HTTP_CODE=$(/usr/bin/curl \
    -H "Authorization: Bearer $TOKEN" \
    -o /tmp/configmap \
    -w "%{http_code}" \
    -X GET "${URL}" 2>/dev/null)

case $HTTP_CODE in
    "200")
        # Configmap was found
        # Get JSON keyvalue without jq
        grep -e '"swiftrings.tar.gz": ".*"' /tmp/configmap  | cut -f 4 -d '"' | base64 -d > $TARFILE
        [ -e $TARFILE ] && tar -xvzf $TARFILE -C /etc/swift/
    ;;

    "404")
        METHOD="POST"
        URL="${BASE_URL}"
    ;;

    *)
        exit 1
    ;;
esac

cd /etc/swift

# Create new rings if not existing
for f in account.builder container.builder object.builder; do
    [ ! -e $f ] && swift-ring-builder $f create 8 ${SWIFT_REPLICAS} 1
done

# Iterate over all devices from the list created by the SwiftStorage CR.
# This does not check for existing ones, which is OK for smaller rings but will
# be replaced in the improved version. It's basically a dumb brute-force
# approach to add devices and set their weights
for DEV in $(cat /var/lib/config-data/ring-devices/devices.csv); do
    REGION=$(echo $DEV | cut -f1 -d,)
    ZONE=$(echo $DEV | cut -f2 -d,)
    HOST=$(echo $DEV | cut -f3 -d,)
    DEVICE_NAME=$(echo $DEV | cut -f4 -d,)
    WEIGHT=$(echo $DEV | cut -f5 -d,)

    swift-ring-builder account.builder add --region $REGION --zone $ZONE --ip $HOST --port 6202 --device $DEVICE_NAME --weight $WEIGHT
    swift-ring-builder container.builder add --region $REGION --zone $ZONE --ip $HOST --port 6201 --device $DEVICE_NAME --weight $WEIGHT
    swift-ring-builder object.builder add --region $REGION --zone $ZONE --ip $HOST --port 6200 --device $DEVICE_NAME --weight $WEIGHT

    # This will change the weights, eg. after bootstrapping and correct PVC
    # sizes are known.
    swift-ring-builder account.builder set_weight --region $REGION --zone $ZONE --ip $HOST --port 6202 --device $DEVICE_NAME $WEIGHT
    swift-ring-builder container.builder set_weight --region $REGION --zone $ZONE --ip $HOST --port 6201 --device $DEVICE_NAME $WEIGHT
    swift-ring-builder object.builder set_weight --region $REGION --zone $ZONE --ip $HOST --port 6200 --device $DEVICE_NAME $WEIGHT
done

# TODO: needs a check if it is safe to rebalance individual rings
for f in *.builder; do
    swift-ring-builder $f rebalance
done

# Tar up all the ring data and either create or update the SwiftRing ConfigMap
BINARY_DATA=`tar cvz *.builder *.ring.gz backups/*.builder | /usr/bin/base64 -w 0`
CONFIGMAP_JSON='{
    "apiVersion":"v1",
    "kind":"ConfigMap",
    "metadata":{
        "name":"'${CM_NAME}'",
        "namespace":"'${NAMESPACE}'",
        "ownerReferences": [
            {
                "apiVersion": "'${OWNER_APIVERSION}'",
                "kind": "'${OWNER_KIND}'",
                "name": "'${OWNER_NAME}'",
                "uid": "'${OWNER_UID}'"
            }
        ]
    },
    "binaryData":{
        "swiftrings.tar.gz": "'${BINARY_DATA}'"
    }
}'

# https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/config-map-v1/#update-replace-the-specified-configmap
/usr/bin/curl \
    -H "Authorization: Bearer $TOKEN" \
    --data-binary "${CONFIGMAP_JSON}" \
    -H 'Content-Type: application/json' \
    -X "${METHOD}" "${URL}"
