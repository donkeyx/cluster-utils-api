# cluster-utils-api

## description

Simple docker image which will stand up a flexibile api that handles most entrypoints and has all the health variations. This allows me to deploy to a cluster, ecs/eks with any entrypoint or params and it will still run and respont to health checks. Great for testing cluster setup and has endpoints for debugging routing and headers.

Default route is swagger doc for the endpoints

| dockerhub: https://hub.docker.com/repository/docker/donkeyx/cluster-utils-api

| github: https://github.com/donkeyx/cluster-utils-api

## Usage

The the endpoints are generally readable, but /a/ will be authenticated and you can find the bearer token in the logs. This will rotate on every startup to keep the sensitive endpoints a bit more secure.

### Start container:

```bash
docker run -d -p 8080:8080 --name test-api donkeyx/cluster-utils-api:latest
```
### use the swagger docs by opening this in your browser
http://localhost:8080


```bash
# view basic info
curl -sS localhost:8080/help|jq
{
  "/": "This can be used to redirect to the swagger docs",
  "/a/env": "GET",
  "/debug": "GET",
  "/headers": "GET",
  "/health": "GET",
  "/healthz": "GET",
  "/ping": "GET",
  "/ready": "GET",
  "/readyz": "GET"
}
```

### run image in k8 cluster:

You can run the pod in your cluster with the commands below. This will start a deployment and
service called ```cluster-utils-api``` but limited to cluster ip. If you want to expose with
type loadbalancer you can do it yourself, I don't want you to get a bill from this.

apply the manifest to create the pod and service
```bash
# apply pod config with default 30min timeout
kubectl -n default \
    apply -f https://raw.githubusercontent.com/donkeyx/cluster-utils-api/master/k8s-cluster-util-apis.yml
```

list the created pod and service.
```bash
# list the pod and service which shows
$ kubectl get pods,svc -n default
NAME                                     READY   STATUS    RESTARTS   AGE
pod/cluster-utils-api-6c5999df88-wssg6   1/1     Running   0          14m

NAME                        TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)   AGE
service/cluster-utils-api   ClusterIP   10.114.3.0   <none>        80/TCP    14m
service/kubernetes          ClusterIP   10.114.0.1   <none>        443/TCP   99d
...
```

### Now you can use port forwarding to curl your apis inside the cluster

```bash
# in one windows forward the ports to the service
$ kubectl -n default port-forward svc/cluster-utils-api 8080:80

# then curl the service -> pod
curl -sS localhost:8080/debug|jq
{
  "Headers": {
    "Accept": [
      "*/*"
    ],
    "User-Agent": [
      "curl/7.68.0"
    ]
  },
  "Hostname": "DESKTOP-V9N2U1D",
  "RequestURI": "/debug",
  "SourceIP": "127.0.0.1",
  "UserAgent": "curl/7.68.0"
}
```
