# docker_cluster-utils-api

Sample docker image to test docker deployments

## description
Simple container for testing deployments, which returns env vars as json over port 8080

## Usage

### build and push

docker build -t donkeyx/cluster-utils-api .

You can now push your new image to your own repo or wherever you like

### Start your image:
docker run -d -p 8080:8080 --name test-api donkeyx/cluster-utils-api:latest

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