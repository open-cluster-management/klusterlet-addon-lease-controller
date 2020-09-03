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
          - brol
          - -lease-duration # The lease duration in secondes, default 60 sec
          - "60"
          - -startup-delay # The delay to start the controller.
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


