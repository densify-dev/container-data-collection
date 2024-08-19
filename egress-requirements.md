# Egress Requirements

A Kubernetes or OpenShift cluster may run in an environment where egress (outgoing) web traffic is subject to network security devices or services. Such devices or services may perform "web filtering" by:

- Replacing the target certificate with their own (often self-signed) certificate for the purpose of traffic inspection;
- Manipulating the HTTP request body and/or headers;
- Manipulating the HTTP response body and/or headers.

If the Data Forwarder is deployed in such a cluster, this may impact:

- Collecting the data from an external observability platform, if you are using one;
- Uploading the data to Densify.

## Troubleshooting

### Self-signed certificate failure

If you are collecting data from an **external observability platform** or uploading the data to Densify and encounter failure and the Data Forwarder logs indicate the following:

```shell
failed to verify certificate: x509: certificate signed by unknown authority
```

or similar text, the certificate should be examined as follows:

1. In the same cluster (and same namespace), follow the instructions [here](https://github.com/densify-dev/cert-output/tree/main/examples) and examine the logs.

2. If at the tail of the log you see text similar to:

```shell
--- openssl log:
...
verify error:num=...:self-signed certificate in certificate chain
...

```

Then it is likely that the genuine certificate has been replaced by a network security device/service self-signed certificate.

Note: this issue can also occur when you are using in-cluster, Authenticated Prometheus where the CA certificate is misconfigured (this case should be resolved by fixing the configuration).

### Request / Response Manipulation

If you are collecting data from an external observability platform, this issue may result in failure to collect your container data.

When uploading data to Densify, this issue may result in failure to upload any data due to authentication failure. Please verify that the username and (encrypted) password are correct, and then contact support@densify.com for assistance.

If the Data Forwarder logs include text similar to:

```json
{"level":"fatal","pkg":"default","error":"HTTP status code: 400, Message: message: Unauthorized, status: 400",...,"message":"failed to initialize Densify client"}
```

And the HTTP status code for `Unauthorized` is `400` (Bad Request), not `401`, it's likely that the request has been manipulated. An incorrect username/(encrypted) password combination will return a `401` status code.

## Resolution

Turn off web filtering, either globally or for this target.
