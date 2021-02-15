---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: exoscale-cloud-controller-manager
  namespace: kube-system
  labels:
    app: exoscale-cloud-controller-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: exoscale-cloud-controller-manager
  template:
    metadata:
      labels:
        app: exoscale-cloud-controller-manager
    spec:
      dnsPolicy: Default
      hostNetwork: true
      serviceAccountName: cloud-controller-manager
      tolerations:
        - key: node.cloudprovider.kubernetes.io/uninitialized
          value: "true"
          effect: NoSchedule
        - key: "CriticalAddonsOnly"
          operator: "Exists"
        - key: node-role.kubernetes.io/master
          effect: NoSchedule
      containers:
      - image: exoscale/cloud-controller-manager:test
        imagePullPolicy: Never
        name: exoscale-cloud-controller-manager
        command:
        - sh
        - -c
        - |
          echo '{"name":"test","api_key":"xxx","api_secret":"xxx"}' > /tmp/iam-keys && \
          exec /exoscale-cloud-controller-manager \
          --cloud-provider=exoscale \
          --leader-elect=true \
          --allow-untagged-cloud  \
          --v=3
        env:
          - name: EXOSCALE_ZONE
            value: %%EXOSCALE_ZONE%%
          - name: EXOSCALE_API_CREDENTIALS_FILE
            value: /tmp/iam-keys

---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cloud-controller-manager
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  name: system:cloud-controller-manager
rules:
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - get
  - create
  - update
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
  - update
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - '*'
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - 'get'
  - 'watch'
  - 'list'
- apiGroups:
  - ""
  resources:
  - nodes/status
  verbs:
  - patch
- apiGroups:
  - ""
  resources:
  - services
  verbs:
  - list
  - patch
  - update
  - watch
- apiGroups:
  - ""
  resources:
  - services/status
  verbs:
  - list
  - patch
  - update
  - watch
- apiGroups:
  - ""
  resources:
  - serviceaccounts
  verbs:
  - create
- apiGroups:
  - ""
  resources:
  - persistentvolumes
  verbs:
  - get
  - list
  - update
  - watch
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - create
  - get
  - list
  - watch
  - update
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: system:cloud-controller-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:cloud-controller-manager
subjects:
- kind: ServiceAccount
  name: cloud-controller-manager
  namespace: kube-system
