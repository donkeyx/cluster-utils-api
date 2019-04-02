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
❯ curl localhost:8080 | jq "."
{
  "npm_config_cache_lock_stale": "60000",
  "npm_config_ham_it_up": "",
  "npm_config_legacy_bundling": "",
  "npm_config_sign_git_tag": "",
  "LANGUAGE": "en_AU.UTF-8",
  "npm_config_user_agent": "npm/6.7.0 node/v11.13.0 linux x64",
  "npm_config_always_auth": "",
  "NODE_VERSION": "11.13.0",
  "npm_config_bin_links": "true",
  "npm_config_key": "",
  "HOSTNAME": "efa8d452f81e",
  "YARN_VERSION": "1.15.2"
}
```