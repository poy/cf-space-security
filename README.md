# cf-space-security
Microservice Security for CloudFoundry applications

```
Request -> Go-Router -> Reverse Proxy -> Application -> Proxy -> Go-Router
```

## Proxy

### Authorization

The proxy is a HTTP proxy that will add the `Authorization` header to any
request that goes to a configured domain. This enables an application to not
worry about refresh and access tokens and just focus on its business logic.

### `GET` Caching
The proxy caches results for `GET` requests to enable the application to more
freely make requests without concerns of DDOSing peers or the system. This
gives the application a more performat feel with no extra work.

## Reverse Proxy

The reveres proxy sits ahead of the application to ensure that any request
made to the application has the `Authorization` header set with a `JWT` that
has access to the application. Each reqeust is taken to CAPI's
`/v3/apps/<GUID` endpoint. If the JWT does not result in a 200, then the
request is returned with a 401 (it is recommended to use the reverse proxy
with the proxy for the `GET` caching).
