# Kubernetes ElasticSearch Operator

A simple operator to provision and deprovision ElasitcSearch indexes in a namespace in Kubernetes.

**NOTE:** This repo is intended for demo purposes only and should not be used in Production since not all the states are currently implemented. Contact me if you wish to contribute.

## Description

This repo shows how to build an **operator** in Kubernetes using [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).

It can be used to create and secure ElasticSearch indexes which are namespace scoped. This can be used when you have multiple applications or tenants sharing an ElasticSearch cluster. 

This Operator creates a custom resource definition (CRD) called **Index** that is used to manage the ES inside a given Kubernetes namespace.

You can find an example [here](config/samples/es-provisioner_v1_index.yaml).

Applications will include the Index CRD in their Helm charts and deploy it alongside the application. 

When deploying an application in a Kubernetes namespace, this operator will create the following:

- ElasticSearch Index for the particular namespace. You can specify a name which is optional, otherwise one will be generated based on the application and namespace.
- ElasticSearch Alias for the Index
- ElasticSearch Role with only read/write permission to the created index only
- ElasticSearch User and Password with that given role


After the Index is provisioned it will create a secret `es-provisioner-index-secret` in the namespace with the following keys:

- `username`: To connect to ES
- `password`: User password
- `index`: ES Index

Applications should mount the secret as Env Vars and use it to talk to ElasticSearch.

Since there will be an Operator per cluster all Operations must be done asynchronously to avoid blocking call and performance issues.

## Usage

Follow the *getting started* section for instruction on how to build an deploy the operator.

Once the CRDs and the controller are deployed, you can use the CRD to deploy indexes into an ElasticSearch cluster:

```
apiVersion: es-provisioner.com.ramos/v1
kind: Index
metadata:
  labels:
    app.kubernetes.io/name: index
    app.kubernetes.io/created-by: es-provisioner-operator
  name: index-sample
spec:
  application: test
  sourceEnabled: true
  numberOfShards: 2
  numberOfReplicas: 0
  properties: |-
    "field1": {
        "type": "text",
        "fields": {
          "keyword": {
            "type": "keyword"
          }
        }
      },
      "internal_id": {
        "type": "keyword"
      },
      "name": {
        "analyzer": "standard",
        "type": "text",
        "fields": {
          "keyword": {
            "type": "keyword"
          }
        }
      }
      
```
In this example we create an index for an application `test` in a given namespace.

Alternatively, you can pass the index configuration as a **ConfigMap**:

```
apiVersion: es-provisioner.com.ramos/v1
kind: Index
metadata:
  labels:
    app.kubernetes.io/name: index
    app.kubernetes.io/created-by: es-provisioner-operator
  name: index-sample
spec:
  application: test
  configMap: "myconfigmap"

```


### Future Functionality

This Operator can be extended to support any index management tasks such changing the schema, number of shards or any other operation.

This operator should also support index management to perform operations such as:
- Change Index settings such as number of shards, analyzer, etc.
- Disaster Recovery
- Schema migration

## TODO

- Add handlers for all possible states. For example, currently if the index creation fails, the operator goes out of sync and we need manual intervention.
- Add test cases
- Support full YAML configuration instead of passing the schema in JSON.


## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:
	
```sh
make docker-build docker-push IMG=<some-registry>/es-provisioner-operator:tag
```
	
3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/es-provisioner-operator:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing

Contact me if you are interested in prodictionizing this operator.

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster 

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

