Quay OpenShift Registry Operator
=============================

Operator responsible for facilitating the utilization of Quay as Red Hat's default image registry

## Overview

The OpenShift Container Platform contains an in cluster image registry for pushing and pulling images that either produced within the cluster or are references from external sources. Quay is a highly available, enterprise grade image registry with enhanced role based access control (RBAC) features. The goal of this operator is to support replicating the features found within OpenShift intgrated image registry with Quay.

## Functionality

<TODO>

## Prerequisites

This section illustrates the steps that must be completed prior to making use of the Operator:

### Quay Application

The operator communicates with Quay using its provided RESTful API. Since Quay features a fully functional RBAC system, OpenShift must be granted access to query Quay. Access to the API makes use of OAuth associated to a Quay Application. Utilize the following steps to create an application within an organization.

1. When logged into Quay as a Superuser account, select the desired organization
2. Select **Applications** on the lefthand navigation bar
3. Select **Create New Application** and enter a name for the Application
4. Select the newly created Application
5. Select **Generate Token** on the lefthand navigation bar
6. Select all available options to grant the account full access to Quay
7. Select **Generate OAuth Token**
8. Review the permissions that will be applied and select **Authorize Application** and then confirm the action in the corresponding dialog
9. Copy the Access Token that is provided on the following page

### Create a Secret

The Operator will use the previously obtained Access Token to communicate with Quay. Store this token within OpenShift as a secret. Create or reuse an existing OpenShift project that can be used to store the secret. 

Execute the following command to create a secret called `quay-integration` with a key called `token` containing the access token:

```
$ oc create secret generic --from-literal=token=<access_token>
```

### TLS Considerations

Best practices dictate that all communications between a client and an image registry be facilitated through secure means. Communications should all leverage HTTPS/TLS with a certificate trust between the parties. While Quay can be configured to serve in an insecure configuration, proper certificates should be utilized on the server and configured on the client. Follow the [OpenShift documentation](https://docs.openshift.com/container-platform/3.11/day_two_guide/docker_tasks.html#day-two-guide-managing-docker-certs) for adding and managing certificates at the container runtime level. 

## Setup and Configuration

Utilize the following steps to complete the setup and configuration. There is an assumption that the user executing the following commands has `cluster-admin` rights.

### Apply Custom Resource Definition

The connection between OpenShift and Quay is made possible through the use of a [custom resources](https://docs.openshift.com/container-platform/3.11/dev_guide/creating_crd_objects.html) and specifically a Custom Resource Definition (CRD) called a `QuayIntegration`. 

To enable the `QuayIntegration` custom resource within the cluster, execute the following command: 

```
$ oc create -f deploy/crds/redhatcop_v1alpha1_quayintegration_crd.yaml
```

### Generate a certificate for the application

<TODO>

```
./utils/webhook-create-signed-cert.sh --namespace <operator_namespace> --service quay-openshift-registry-operator --secret quay-openshift-registry-operator
```

### Operator Deployment

The operator can be deployed in any namespace. Create or utilize an existing namespace (can be the same namespace that was previously created).

Locate the _ClusterRoleBinding_ in the _deploy/clusterrole_binding.yaml_ file and update the `namespace` field.

Finally, loacte the _deploy/operator.yaml_ file and update the value of the _image_ field.

Now, execute the following command to add the permissions to the cluster and deploy the operator:

```
$ oc create -f deploy/service_account.yaml
$ oc create -f deploy/clusterrole.yaml
$ oc create -f deploy/clusterrole_binding.yaml
$ oc create -f deploy/operator.yaml
```

### Setup the MutatingWebhookConfiguration


A [Mutating Webhook Configuration]() is an Admission Controller that....


#### Enable the Admission Plugin

On each OpenShift master, add the following to the `/etc/origin/master/master-config.yaml` file

```
admissionConfig:
  pluginConfig:
    MutatingAdmissionWebhook:
      configuration:
        apiVersion: apiserver.config.k8s.io/v1alpha1
        kubeConfigFile: /dev/null
        kind: WebhookAdmission
    ValidatingAdmissionWebhook:
      configuration:
        apiVersion: apiserver.config.k8s.io/v1alpha1
        kubeConfigFile: /dev/null
        kind: WebhookAdmission
```

Restart the OpenShift API and Controller

```
/usr/local/bin/master-restart api && /usr/local/bin/master-restart controllers
```


Get the CA bundle for the cluster

```
oc get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 | tr -d '\n'
```

Update the `<CA_BUNDLE>` value within the [deploy/mutatingwebhookconfiguration.yaml](deploy/mutatingwebhookconfiguration.yaml) file 

Add the Mutating Webhook Configuration to the cluster

```
oc create -f deploy/mutatingwebhookconfiguration.yaml
```



### Create the QuayIntegration Custom Resource

Finally, to complete the integration between OpenShift and Quay, a `QuayIntegration` custom resource needs to be created.

The following is an example of a basic definition of a `QuayIntegration` resource associated from the associated CRD.

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayIntegration
metadata:
  name: example-quayintegration
spec:
  clusterID: openshift
  credentialsSecretName: cat<NAMESPACE>/<SECRET>
  quayHostname: https://<QUAY_URL>
```

The _clusterID_ is a value which should be unique across the entire ecosystem while the _credentialsSecretName_ is a NamespacedName value of the secret previously created.

A baseline `QuayIntegration` Custom Resource can be found in _deploy/crds/redhatcop_v1alpha1_quayintegration_cr.yaml_. Update the values for your environment and execute the following command:

```
$ oc create -f deploy/crds/redhatcop_v1alpha1_quayintegration_cr.yaml
```

Organizations within Quay should be created for the related namespaces in OpenShift