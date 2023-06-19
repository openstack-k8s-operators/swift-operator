#!/bin/sh
cd /etc/swift

cp -t /etc/swift/ /var/lib/config-data/swiftconf/*

swift-ring-builder account.builder create 8 ${SWIFT_REPLICAS} 1
swift-ring-builder container.builder create 8 ${SWIFT_REPLICAS} 1
swift-ring-builder object.builder create 8 ${SWIFT_REPLICAS} 1

for DEV in $(cat /var/lib/config-data/ring-devices/devices.csv); do
	HOST=$(echo $DEV | cut -f1 -d,)
	DEVICE_NAME=$(echo $DEV | cut -f2 -d,)
	WEIGHT=$(echo $DEV | cut -f3 -d,)

	swift-ring-builder account.builder add --region 1 --zone 1 --ip $HOST --port 6202 --device $DEVICE_NAME --weight $WEIGHT
	swift-ring-builder container.builder add --region 1 --zone 1 --ip $HOST --port 6201 --device $DEVICE_NAME --weight $WEIGHT
	swift-ring-builder object.builder add --region 1 --zone 1 --ip $HOST --port 6200 --device $DEVICE_NAME --weight $WEIGHT
done

swift-ring-builder account.builder rebalance
swift-ring-builder container.builder rebalance
swift-ring-builder object.builder rebalance

ACC_BUILDER=`/usr/bin/base64 -w 0 account.builder`
CNT_BUILDER=`/usr/bin/base64 -w 0 container.builder`
OBJ_BUILDER=`/usr/bin/base64 -w 0 object.builder`
ACC_RING=`/usr/bin/base64 -w 0 account.ring.gz`
CNT_RING=`/usr/bin/base64 -w 0 container.ring.gz`
OBJ_RING=`/usr/bin/base64 -w 0 object.ring.gz`

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
		"account.builder": "'${ACC_BUILDER}'",
		"container.builder": "'${CNT_BUILDER}'",
		"object.builder": "'${OBJ_BUILDER}'",
		"account.ring.gz": "'${ACC_RING}'",
		"container.ring.gz": "'${CNT_RING}'",
		"object.ring.gz": "'${OBJ_RING}'"
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
