#!/bin/bash
set -euo pipefail

# get the current machine sets
readarray -t MACHINE_SETS < <($OC get machinesets -o name)
if [ "${#MACHINE_SETS[@]}" -eq 0 ]; then
  echo "No machine sets found."
  exit 1
fi

for machine_set in "${MACHINE_SETS[@]}"; do
  machine_set_name=$(echo "$machine_set" | cut -d'/' -f2)
  $OC create -f - <<EOF
apiVersion: autoscaling.openshift.io/v1beta1
kind: MachineAutoscaler
metadata:
  name: $machine_set_name
  namespace: openshift-machine-api
spec:
  minReplicas: 0
  maxReplicas: 10
  scaleTargetRef:
    apiVersion: machine.openshift.io/v1beta1
    kind: MachineSet
    name: $machine_set_name

EOF

done

oc scale --replicas=0 -n openshift-cluster-version deployment/cluster-version-operator
oc apply -n openshift-machine-api -k ../
oc scale --replicas=0 -n openshift-machine-api deployment/cluster-autoscaler-operator
oc patch -n openshift-machine-api deployment/cluster-autoscaler-default --type='json' \
    -p='[
      {"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value":"--grpc-expander-url=autoscaler-expander.openshift-machine-api.svc.cluster.local:7000"},
      {"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value":"--grpc-expander-cert=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"},
      {"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value":"--expander=grpc"}
    ]'
