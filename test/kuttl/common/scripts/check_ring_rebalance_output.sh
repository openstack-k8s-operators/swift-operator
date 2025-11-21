#!/bin/sh
# Check that all storage pods were included in the rebalance job
oc wait --for=condition=complete -n $NAMESPACE job swift-ring-rebalance
LOGS=$(oc logs -n $NAMESPACE job/swift-ring-rebalance)
PODS=$(oc get pods -n $NAMESPACE -l component=swift-storage -o name | cut -f 2 -d "/")

for pod in $PODS; do
    echo $LOGS | grep -q $pod || exit 1
done
