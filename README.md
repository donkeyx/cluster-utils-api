# cluster-utils-api

## description

Sample docker image to test docker deployments in your clusters or locally. Basically, a hello world with some useful info.

The container will run a simple node express service to show the environment variables available in the container namespace. Container port is 8080 but you can bind to 80 or whatever you like.

Default route is json response with environment variables, with more to come...

## Usage

### Build image:

You can build the image locally like this and then push to your own repo for testing

```bash
# pull image
docker pull donkeyx/cluster-utils-api

# tag
docker tag donkeyx/cluster-utils-api your-repo-url/container-name:latest

# push
docker push your-repo-url/container-name:latest
```

### Start container:

```bash
docker run -d -p 8080:8080 --name test-api donkeyx/cluster-utils-api:latest
```

### run image in k8 cluster:

You can run the pod in your cluster with the commands below. This will start a deployment and
service called ```cluster-utils-api``` but limited to cluster ip. If you want to expose with
type loadbalancer you can do it yourself, I don't want you to get a bill from this.

apply the manifest to create the pod and service
```bash
# apply pod config with default 30min timeout
kubectl -n default \
    apply -f https://raw.githubusercontent.com/donkeyx/docker_cluster-utils-api/master/k8s-cluster-util-apis.yml
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

# then curl the service -> pod
$ curl -sS  localhost:8080 | jq
{
  "version": "v1.1",
  "endpoints": [
    "/statsz",
    "/healthz",
    "/ping",
    "/envz",
    "/readyness_delay"
  ]
}
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
