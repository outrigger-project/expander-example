# Sample expander

Sample expander with example deployment and test job for Openshift clusters.

Not intended for production use.

## Pre-requisites

- Setup the KUBECONFIG
- Ensure you have arm64 and amd64 MachineSets in the OCP cluster
- Ensure you have the `oc` CLI installed and configured to access your OCP cluster

## Deploy

```bash
bash hack/deploy-ocp.sh
```

## Test

```bash
oc apply -f deploy/test/job.yaml
```