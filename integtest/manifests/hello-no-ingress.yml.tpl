apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
  labels:
    app: hello
spec:
  selector:
    matchLabels:
      app: hello
  replicas: 2
  template:
    metadata:
      labels:
        app: hello
    spec:
      containers:
      - name: nginx
        image: nginxdemos/hello:plain-text
        ports:
        - containerPort: 80
---
kind: Service
apiVersion: v1
metadata:
  name: hello
  annotations:
    service.beta.kubernetes.io/exoscale-loadbalancer-zone: "%%EXOSCALE_ZONE%%"
    service.beta.kubernetes.io/exoscale-loadbalancer-id: "%%EXTERNAL_NLB_ID%%"
    service.beta.kubernetes.io/exoscale-loadbalancer-keep: "true"
    service.beta.kubernetes.io/exoscale-loadbalancer-service-instancepool-id: "%%NODEPOOL_ID%%"
    service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-interval: "5s"
    service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-timeout: "2s"
    service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-retries: "1"
spec:
  selector:
    app: hello
  type: LoadBalancer
  ports:
    - port: 80
