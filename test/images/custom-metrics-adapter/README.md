# k8s-e2e-adapter

Forked from https://github.com/kubernetes-incubator/custom-metrics-apiserver

## Running:

1. Make sure you've built the included Dockerfile with `make docker-build`.
   
2. *(Optional, if modifying deployment to run securely):*
   Create a secret called `cm-adapter-serving-certs` with two values:
   `serving.crt` and `serving.key`. For more information on how to
   generate these certificates, see the [auth concepts
   documentation](https://github.com/kubernetes-incubator/apiserver-builder/blob/master/docs/concepts/auth.md)
   in the apiserver-builder repository.

4. `kubectl create -f k8s-e2e-adapter.yaml`

5. Submit a POST request to one of the following endpoints:

**Namespaced resources:**

`/write-metrics/namespaces/{namespace}/{resourceType}/{name}/{metric}`

**Rootscoped resources:**

`/write-metrics/{resourceType}/{name}/{metric}`

**Namespaces:**

`/write-metrics/namespaces/{namespace}/metrics/{name}/{metric}`

Where

`{namespace}` is the namespace of the resource whose metric you wish to set

`name` is the name of the resource

`resourceType` is the type of resource

`metric` is the name of the metric

With a body of `{"Value":xxx}` (where `xxx` is an int) to set a metric value

ex: `curl -X POST -H 'Content-Type: application/json' -d '{"Value":4200000000000}' 172.17.0.4:8080/write-metrics/namespaces/custom-metrics/services/custom-metrics-apiserver/packets-per-second`

6. Get the request via the metrics API

ex: `kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/namespaces/custom-metrics/services/custom-metrics-apiserver/packets-per-second" | jq`