#!/bin/sh
cd /etc/swift

cp -t /etc/swift/ /var/lib/config-data/swiftconf/*
tar -xvzf /var/lib/config-data/rings/swiftrings.tar.gz -C /etc/swift/

for f in account.builder container.builder object.builder; do
    [ ! -e $f ] && swift-ring-builder $f create 8 ${SWIFT_REPLICAS} 1
done

for DEV in $(cat /var/lib/config-data/ring-devices/devices.csv); do
    HOST=$(echo $DEV | cut -f1 -d,)
    DEVICE_NAME=$(echo $DEV | cut -f2 -d,)
    WEIGHT=$(echo $DEV | cut -f3 -d,)

    swift-ring-builder account.builder add --region 1 --zone 1 --ip $HOST --port 6202 --device $DEVICE_NAME --weight $WEIGHT
    swift-ring-builder container.builder add --region 1 --zone 1 --ip $HOST --port 6201 --device $DEVICE_NAME --weight $WEIGHT
    swift-ring-builder object.builder add --region 1 --zone 1 --ip $HOST --port 6200 --device $DEVICE_NAME --weight $WEIGHT
done

for f in *.builder; do
    swift-ring-builder $f rebalance
done

TARFILE=`tar cvz *.builder *.ring.gz backups/*.builder | /usr/bin/base64 -w 0`

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
        "swiftrings.tar.gz": "'${TARFILE}'"
    }
}'

# Credentials to be used by curl
export CURL_CA_BUNDLE=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)

# https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/config-map-v1/#update-replace-the-specified-configmap
/usr/bin/curl \
    -H "Authorization: Bearer $TOKEN" \
    --data-binary "${CONFIGMAP_JSON}" \
    -H 'Content-Type: application/json' \
    -X PUT "https://kubernetes.default.svc/api/v1/namespaces/${NAMESPACE}/configmaps/${CM_NAME}"
