# klusterlet-addon-lease-controller

This controller runs on the managed-cluster as a side car of each addon controller and it creates a lease on the hub when the addon secret is created on the managed-cluster.

The controller binary has the following parameters 

```
          command: 
          - klusterlet-addon-lease-controller
          args:
          - -lease-name # The lease name
          - addon-lease
          - -lease-namespace # The namespace where the lease must be created on the hub 
          - open-cluster-management-self-import
          - -hub-kubeconfig-secret # the secret on the managed-cluster containing the hub kubeconfig for the specific addon. The namespace is defined by the env var $WATCH_NAMESPACE
          - my-addon-hub-kubeconfig-secret
          - -lease-duration # The lease duration in secondes, default 60 sec
          - "60"
          - -startup-delay # The delay to start the controller, default 10 sec.
          - "10"
          env:
          - name: WATCH_NAMESPACE # The namespace to monitor the hub the hub-kubeconfig-secret
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: metadata.namespace
          - name: POD_NAME # The pod name to check for readyness, usually the pod name where the lease-controller runs.
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
          - name: POD_NAMESPACE # The pod namespace
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: metadata.namespace
```

## ServiceAccount and Role

The serviceaccount used on the hub (which is identify by the token in the provided secret `-hub-kubeconfig-secret` parameter) must have at least the verbs: get, update, create for the `leases.coordination.k8s.io`

```
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - get
  - update
  - create
```

# Build

`make build`

# run locally

1. Set the environment variables, POD_NAME, POD_NAMESPACE and WATCH_NAMESPACE.
2. Update the make file `run` target with the parameters like 
`-lease-name addon-lease -lease-namespace open-cluster-management-self-import -hub-kubeconfig-secret appmgr-hub-kubeconfig`
or launch
`go run ./main.go -lease-name addon-lease -lease-namespace open-cluster-management-self-import -hub-kubeconfig-secret appmgr-hub-kubeconfig`


# Unit tests

`make test`

# functional tests

The functional tests will create 2 kind clusters, one for the hub and one for the managedcluster.
`make functional-test-full`

