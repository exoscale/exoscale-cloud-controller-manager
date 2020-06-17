# Exoscale LoadBalancer Service

This example will show you how to use the `exoscale-cloud-controller-manager` to create a service of type: LoadBalancer.

First, you need a exoscale instancepool running a `exoscale-cloud-controller-manager`, please follow this guideline to Deploy [Exoscale Cloud Controller Manager](../../README.md).

For this example, we will deploy a simple web application that will be accessible from the outside using a LoadBalancer.

``` yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-app
spec:
  selector:
    matchLabels:
      app: nginx
  replicas: 2
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginxdemos/hello:latest
        ports:
        - containerPort: 80
```

And we will deploy a simple tcp loadbalancer to expose the web application.

>Note: you will need to specify the zone in which to deploy your loadbalancer and the id of your instancepool running a `exoscale-cloud-controller-manager`.

``` yaml
kind: Service
apiVersion: v1
metadata:
  name: nginx-service
  annotations:
    service.beta.kubernetes.io/exo-lb-zone: "ch-gva-2"
    service.beta.kubernetes.io/exo-lb-service-instancepoolid: "abcdefgh-1234-ijkl-5678-mnopqrstuvwx"
spec:
  selector:
    app: nginx
  type: LoadBalancer
  ports:
  - name: service
    protocol: TCP
    port: 80
    targetPort: 80
  - name: health-check
    protocol: TCP
    port: 55
    targetPort: 80
```

execute this command:
```
kubectl create -f demo-lb.yaml
```

To access your service you will have to wait for your loadbalancer to be correctly deployed:

```
kubectl get service
NAME            TYPE           CLUSTER-IP       EXTERNAL-IP   PORT(S)        AGE
nginx-service   LoadBalancer   10.105.252.146   x.x.x.x       80:30432/TCP   127m

exo nlb list
┼──────────────────────────────────────┼──────────────────────────────────────────┼──────────┼─────────────────┼
│                  ID                  │                   NAME                   │   ZONE   │   IP ADDRESS    │
┼──────────────────────────────────────┼──────────────────────────────────────────┼──────────┼─────────────────┼
│ dkeifucx-e286-4bc6-ac36-qpwod48fjrye │ nlb-bd7c92e2-0d1f-4988-8cb3-88ecdebb6649 │ ch-gva-2 │      x.x.x.x    │
┼──────────────────────────────────────┼──────────────────────────────────────────┼──────────┼─────────────────┼
```

Now, you can now access your service

```
curl -i http://EXTERNAL_IP
```
