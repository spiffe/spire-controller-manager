# Kubernetes Workload Registrar Migration

## Introduction

This guide will walk you through how to migrate an existing Kubernetes Workload Registrar deployment to SPIRE Controller Manager. Existing entries created by the Kubernetes Workload Registrar aren't compatible with SPIRE Controller Manager so they'll be deleted and replaced with new entries. Workloads will continue to function with the old entries until their certificates expire, after which they'll get new certificates based on the new entries.

> **Note**
> As we'll be deleting and creating entries, it's important to do this migration during a downtime window.

## Clean up Kubernetes Workload Registrar Resources

First we need to clean up the Kubernetes Workload Registrar and its resources.

1. (CRD mode only) Delete the SpiffeId Custom Resource Definition (CRD). This will delete all entries created by the k8s-workload-registrar. 
   ```shell
   kubectl delete crd spiffeids.spiffeid.spiffe.io
   ```

1. Delete the `ValidatingWebhookConfiguration`, `Service`, `Roles`, and other k8s-workload-registrar config. Not all of the resources below are applicable for all k8s-workload-registrar modes, so if there's a "not found" message it's safe to ignore. In general make sure to clean up any Kubernetes Workload Registrar resources aside from the SPIRE Server and Kubernetes Workload Registrar itself. Those will be removed below.
   ```shell
   kubectl delete validatingwebhookconfigurations k8s-workload-registrar k8s-workload-registrar-webhook
   kubectl delete service k8s-workload-registrar -n spire
   kubectl delete clusterrolebindings k8s-workload-registrar-role-binding spire-k8s-registrar-cluster-role-binding
   kubectl delete clusterroles k8s-workload-registrar-role spire-k8s-registrar-cluster-role
   kubectl delete rolebinding spire-k8s-registrar-role-binding -n spire
   kubectl delete role spire-k8s-registrar-role -n spire
   kubectl delete configmaps k8s-workload-registrar k8s-workload-registrar-certs -n spire
   kubectl delete secret k8s-workload-registrar-secret
   ```

## Deploy Spire Controller Manager

Next we deploy the new SPIRE Controller Manager.

1. Create the `ClusterSPIFFEID` CRD, `ValidatingWebhookConfiguration`, `Service`, `Roles`, and other SPIRE Controller Manager config.
   ```shell
   kubectl apply -f config/spire.spiffe.io_clusterspiffeids.yaml \
                 -f config/spire.spiffe.io_clusterfederatedtrustdomains.yaml \
                 -f config/spire-controller-manager-webhook.yaml \
                 -f config/leader_election_role.yaml \
                 -f config/leader_election_role_binding.yaml \
                 -f config/role.yaml \
                 -f config/role_binding.yaml \
                 -f config/spire-controller-manager-config.yaml \
                 -f config/spire-server.yaml
   ```

1. Create the `ClusterSpiffeId` custom resource. The below example will create SPIFFE IDs with this shape: `spiffe://<trust domain>/ns/<namespace>/sa/<serviceaccount>`. Only Pods with the label `spiffe.io/spiffe-id: true` will have entries auto-created. This corresponds to the `identity_template` and `identity_template_label` configurables from CRD mode Kubernetes Workload Registrar. 
   ```shell
   kubectl apply -f config/clusterspiffeid.yaml
   ```

   > **Note**
   > See [FAQs](#faqs) or instructions on how to translate [label](#how-do-i-do-label-based-workload-registration), [annotation](#how-do-i-do-annotation-based-workload-registration), and [service account](#how-do-i-do-service-account-based-workload-registration) based workload registration. Also see [ClusterSPIFFEID defintion](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md) for more information on how to create the shape most suitable to your environment.

## Verify Spire Controller Manager Deployment

First verify the Pods came up correctly. The `spire-server-0` Pod should have two containers running in it.

```shell
$ kubectl get pods -n spire
NAME                READY   STATUS    RESTARTS      AGE
spire-agent-5jkzg   1/1     Running   0             46m
spire-server-0      2/2     Running   1 (11m ago)   11m
```
> **Note**
> It's ok to see a restart in the `spire-server-0` Pod. SPIRE Controller Manager relies on the SPIRE Server to get a certificate for it's Webhook, and when SPIRE Controller Manager comes up first it can't get that certificate and restarts. See [#39](https://github.com/spiffe/spire-controller-manager/issues/39).

Next try to deploy this example NGINX Deployment:

```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes/website/master/content/en/examples/application/simple_deployment.yaml

```

And add the label to the Deployment Template. This will reroll the Deployment

```shell
kubectl patch deployment nginx-deployment -p '{"spec":{"template":{"metadata":{"labels":{"spiffe.io/spiffe-id": "true"}}}}}'
```

From the SPIRE Server you should see a single entry with SPIFFE ID `spiffe://example.org/ns/default/sa/default`:

```shell
$ kubectl exec spire-server-0  -n spire -c spire-server -- ./bin/spire-server entry show
Found 1 entry
Entry ID         : c93a53bd-c313-4239-a13b-75ebf292db8f
SPIFFE ID        : spiffe://example.org/ns/default/sa/default
Parent ID        : spiffe://example.org/spire/agent/k8s_psat/demo-cluster/85ad58a6-64ae-4cc7-a126-f60dfa5b8139
Revision         : 0
X509-SVID TTL    : default
JWT-SVID TTL     : default
Selector         : k8s:pod-uid:dca56e85-142e-4de2-b04a-257ac8d7e3c8
```

When done you can delete the NGINX deployment, this will automatically delete the SPIFFE ID:

```shell
kubectl delete -f https://raw.githubusercontent.com/kubernetes/website/master/content/en/examples/application/simple_deployment.yaml
```

## FAQs

### How do I do label based workload registration?

With this configuration Kubernetes Workload Registrar took a specified Label off of a Pod and used that to form the SPIFFE ID. For example:

```
pod_label = "spiffe.io/spiffe-id"
```

This can be done with the SPIRE Controller Manager with a config like the below:

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  name: label-based
spec:
  spiffeIDTemplate: "spiffe://{{ .TrustDomain }}/{{ index .PodMeta.Labels \"spiffe.io/spiffe-id\" }}"
  podSelector:
    matchExpressions:
      - key: "spiffe.io/spiffe-id"
        operator: "Exists"
```

The `matchExpressions` statement will select only Pods with the `spiffe.io/spiffe-id` label. For Pods with this label, the `spiffeIDTemplate` will extract the value of this label and use it to form the SPIFFE ID.

> **Note**
> Allowing the value of labels to directly populate a SPIFFE ID gives the power to create arbitrary SPIFFE IDs to anyone that can deploy a Pod in your cluster. It's better to define a SPIFFE ID using a template that doesn't depend on a label. See [ClusterSPIFFEID defintion](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md) for more information. 

### How do I do annotation based workload registration?

There is no equivalent to this configuration in SPIRE Controller Manager. Annotations specifically don't allow for selecting Pods with a specific annotation, which SPIRE Controller Manager relies on. The easiest path forward is to convert the annotations to labels and use [label based workload registration]((#how-do-i-do-label-based-workload-registration)).

### How do I do service account based workload registration?

This can be done with the SPIRE Controller Manager with a config like the below:

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  name: service-account-based
spec:
  spiffeIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
```

> **Note**
> This will create entries for every Pod in the system. Its better to restrict it with a label like in the main example in `config/clusterspiffeid.yaml`. Also see [ClusterSPIFFEID defintion](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md) for more information.

### How do I federate trust domains?

With Kubernetes Workload Regisrar the Pod annotation `spiffe.io/federatesWith` is used to create SPIFFE ID's that federate with other trust domains. For example:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    spiffe.io/federatesWith: example.io,example.ai
  name: test
  ...
```

The equivalent with SPIRE Controller Manager is accomplished with the `federatesWith` field of the [ClusterSPIFFEID CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md).

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  name: federation
spec:
  spiffeIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
  podSelector:
    matchLabels:
      spiffe.io/spiffe-id: "true"
  federatesWith: ["example.io", "example.ai"]

```

### How do I add DNS names to my certificates?

You can add multiple DNS names with the `dnsNameTemplates` field of the [ClusterSPIFFEID CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md).

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  name: federation
spec:
  spiffeIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
  podSelector:
    matchLabels:
      spiffe.io/spiffe-id: "true"
  dnsNameTemplates: ["{{ .PodMeta.Name }}", "my-custom-dns-name"]

```

### Does SPIRE Controller Manager automatically populate DNS Names of Services a Pod is attached to?

SPIRE Controller Manager doesn't monitor Endpoints like Kubernetes Workload Registrar did, so it won't do this automatically. A workaround is to use the `app` label to populate DNS Names using `dnsNameTemplates` field of the [ClusterSPIFFEID CRD](https://github.com/spiffe/spire-controller-manager/blob/main/docs/clusterspiffeid-crd.md), assuming you are using `app` as your selector and it matches the name of the `Service`.

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  name: federation
spec:
  spiffeIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
  podSelector:
    matchLabels:
      spiffe.io/spiffe-id: "true"
  dnsNameTemplates: ["{{ index .PodMeta.Labels \"app\" }}.{{ .PodMeta.Namespace }}.svc.cluster.local"]

```

If you require these DNS Names to be automatically populated, please update [#48](https://github.com/spiffe/spire-controller-manager/issues/48) with your use case.

### Can SPIRE Controller Manager be deployed in a different Pod from SPIRE Server?

This is not supported with SPIRE Controller Manager, they must by in the same Pod. If you require them to be in seperate Pods, please open a [new issue](https://github.com/spiffe/spire-controller-manager/issues/new) with your use case.

### How do i see SPIRE Controller Manager logs?

```shell
$ kubectl logs spire-server-0 -n spire -c spire-controller-manager
2022-12-13T00:41:21.362Z	INFO	setup	Config loaded	{"cluster name": "demo-cluster", "trust domain": "example.org", "ignore namespaces": ["kube-system", "kube-public", "istio-system", "spire", "local-path-storage"], "gc interval": "10s", "spire server socket path": "/spire-server/api.sock"}
2022-12-13T00:41:21.764Z	INFO	controller-runtime.metrics	metrics server is starting to listen	{"addr": "127.0.0.1:8082"}
2022-12-13T00:41:21.807Z	INFO	webhook-manager	Minting webhook certificate	{"reason": "initializing", "dnsNames": ["spire-controller-manager-webhook-service.spire.svc"]}
2022-12-13T00:41:21.828Z	INFO	webhook-manager	Minted webhook certificate
2022-12-13T00:41:21.844Z	INFO	webhook-manager	Webhook configuration patched with CABundle
```

### I'm using CRD mode Kubernetes Workload Registrar and it gets stuck deleting the SpiffeId CRD. What do I do?

This can happen if the Kubernetes Workload Registrar is deleted before all the SpiffeId custom resources are removed. To get around this, manually remove the finalizers with the below script and try deleting the CRD again.

```shell
for ns in $(kubectl get ns | awk '{print $1}' | tail -n +2)
do
  if [ $(kubectl get spiffeids -n $ns 2>/dev/null | wc -l) -ne 0 ]
  then
    kubectl patch spiffeid $(kubectl get spiffeids -n $ns | awk '{print $1}' | tail -n +2) --type='merge' -p '{"metadata":{"finalizers":null}}' -n $ns
  fi
done
```

### Why can't Kubernetes Workload Registrar entries be reused with SPIRE Controller Manager?

SPIRE Controller Manager uses a different scheme for parenting SPIFFE IDs. Though it is technically possible to modify all the entries, its a lot easier to just allow SPIRE Controller Maanger to automatically replace the entries.

### What happens if a Pod is deployed while I'm in the middle of this cutover?

SPIRE Controller Manager will reconcile the state of the system when it starts up. Any new Pods deployed after Kubernetes Workload Registrar is deleted and before SPIRE Controller Manager is up will have entries created when SPIRE Controller Manager is up.
