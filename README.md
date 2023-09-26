# cluster-utils-api

## description

Simple docker image to allow testing within clusters or locally. It provides me an api as the base which will run with an entrypoint of node/npm/cu-api and still run the base expected binary. This is great for testing that "your" api might be routable with istio or other meshes, without having to worry about issues with the api its self. It also allows me to deploy and verify routing, check headers and other things like verifying env parameters are exposed to the container.

Default route is json response with environment variables, with more to come...

dockerhub: https://hub.docker.com/repository/docker/donkeyx/cluster-utils-api
github: https://github.com/donkeyx/cluster-utils-api

## Usage

The the endpoints are generally readable, but /a/ will be authenticated and you can find the bearer token in the logs. This will rotate on every startup to keep the sensitive endpoints a bit more secure.

### Start container:

```bash
docker run -d -p 8080:8080 --name test-api donkeyx/cluster-utils-api:latest

# then curl the container
$ curl -sS  localhost:8080 | jq
{
  "version": "v1.1",
  "endpoints": [
    "/health",
    "/healthz",
    "/ready",
    "/readyz",
    "/headers",
    "/readyness_delay",
    "/a/env"
  ]
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

Now you can use port forwarding to curl your apis inside the cluster
```bash
# in one windows forward the ports to the service
$ kubectl -n default port-forward svc/cluster-utils-api 8080:80

```


### curling apis in this container:

curling your running container

```bash
❯ http localhost:8080
HTTP/1.1 200 OK
Connection: keep-alive
Content-Length: 4790
Content-Type: application/json; charset=utf-8
Date: Tue, 02 Apr 2019 08:49:22 GMT
ETag: W/"12b6-OdCM/Gv6+/5YnNIz25ceGper7Zc"
X-Powered-By: Express

{
    "HOME": "/root",
    "HOSTNAME": "377ad6708651",
    "INIT_CWD": "/usr/src/app",
    "LANG": "en_AU.UTF-8",
    "LANGUAGE": "en_AU.UTF-8",
    "LC_ALL": "en_AU.UTF-8",
    "LC_CTYPE": "en_AU.UTF-8",
    "NODE": "/usr/local/bin/node",
    "NODE_VERSION": "11.13.0",
    "PATH": "/usr/local/lib/node_modules/npm/node_modules/npm-lifecycle/node-gyp-bin:/usr/src/app/node_modules/.bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
    "PWD": "/usr/src/app"
}
```
