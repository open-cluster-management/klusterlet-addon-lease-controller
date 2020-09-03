# klusterlet-addon-lease-controller

This controller runs on the managed-cluster as a side car of each addon controller and it creates a lease on the hub when the addon secret is created on the managed-cluster.

The controller binary has the following parameters 

```
          command: 
          - klusterlet-addon-lease-controller
          args:
          - -lease-name 
          - addon-lease
          - -lease-namespace 
          - open-cluster-management-self-import
          - -hub-kubeconfig-secret
          - brol
          - -lease-duration
          - "60"
          - -startup-delay
          - "10"
          env:
          - name: WATCH_NAMESPACE
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: metadata.namespace
          - name: POD_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
          - name: POD_NAMESPACE
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: metadata.namespace
```


