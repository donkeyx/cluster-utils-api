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