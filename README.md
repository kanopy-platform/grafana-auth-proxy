# grafana-auth-proxy

Reverse proxy server, written in Go, to be used with the [Auth Proxy Authentication](https://grafana.com/docs/grafana/latest/auth/auth-proxy/) feature in Grafana.

The proxy will receive a jwt token saved on a cookie, will get claims out of it and will send to Grafana the appropriate headers to authenticate the user.

## Assumptions

The jwt token provided in a cookie should be minted by a known (most likely private) auth provider. The idea is that the proxy will sit behind the auth provider so the cookie is already present when receiving a request.

The proxy doesn't handle any validation or custom actions on the jwt token, it only extracts claims, so it "trust" that the jwt token is already validated and contains safe content.

## Kubernetes deployment considerations

In a Kubernetes environment, the proxy can be deployed as a sidecar to the Grafana Deployment or as a separate one.

An Ingress will route the `/login` path on the Grafana url to the proxy instead of the main app, after that any request to login will be solely answered by the auth proxy. The login page in Grafana won't be accessible, except when bypassing the Ingress rule.

## Local testing

Build and run the application in a local docker container

    make docker-run
