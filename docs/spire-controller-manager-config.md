# SPIRE Controller Manager Configuration

The SPIRE Controller Manager configuration is defined [here](../api/v1alpha1/controllermanagerconfig_types.go).

Beyond the standard [controller manager configuration](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/config/v1alpha1#ControllerConfigurationSpec), the following fields are defined:

| Field                                | Required | Description                                                                                                             |
| ------------------------------------ | -------- | ----------------------------------------------------------------------------------------------------------------------- |
| `clusterName`                        | REQUIRED | The name of the cluster |
| `trustDomain`                        | REQUIRED | The trust domain name for the cluster |
| `ignoreNamespaces`                   | OPTIONAL | Namespaces that the controllers should ignore. Defaults to `kube-system`, `kube-public`, and `spire-system` |
| `validatingWebhookConfigurationName` | OPTIONAL | The name of the validating admission controller webhook to manage. Defaults to `spire-controller-manager-webhook`. |
| `gcInterval`                         | OPTIONAL | How often the SPIRE state is reconciled when the controller is otherwise idle. This impacts how quickly SPIRE state will converge after CRDs are removed or SPIRE state is mutated underneath the controller. Defaults to 10 seconds. |
