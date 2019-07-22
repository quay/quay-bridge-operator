Quay OpenShift Registry Operator
=============================

[![Build Status](https://travis-ci.org/redhat-cop/quay-openshift-registry-operator.svg?branch=master)](https://travis-ci.org/redhat-cop/quay-openshift-registry-operator) [![Docker Repository on Quay](https://quay.io/repository/redhat-cop/quay-openshift-registry-operator/status "Docker Repository on Quay")](https://quay.io/repository/redhat-cop/quay-openshift-registry-operator)


Operator responsible for facilitating the utilization of Red Hat Quay as the default image registry for an OpenShift Container Platform environment

## Overview

The OpenShift Container Platform contains an in cluster image registry for pushing and pulling images that either produced within the cluster or are references from external sources. Quay is a highly available, enterprise grade image registry with enhanced role based access control (RBAC) features. The goal of this operator is to support replicating the features found within OpenShift intgrated image registry with Quay.

## Functionality

The following capabilities are enabled through this operator:

* Synchronization of OpenShift namespaces as Quay organizations
    * Creation of Robot accounts for each default namespace service account
    * Creation of Secrets for each created Robot Account
        * Associate Robot Secret to Service Account as Mountable and Image Pull Secret
    * Synchronization of OpenShift ImageStreams as Quay Repositories
* Automatically rewrite new Builds making use of ImageStreams to output to Quay
* Automatically import ImageStream tag once build completes

## Prerequisites

The following requirements must be satisfied prior to setting and configuring the integration between OpenShift and Quay:

* A Red Hat Quay environment with a user with superuser permissions
* A Red Hat OpenShift Container Platform environment (at least version 3.6 [3.11 recommended]) with cluster-admin permissions on an OpenShift cluster 
*	OpenShift Command Line tool (CLI)

## Setup and Configuration

The setup process involves steps within both Quay and OpenShift as described below:

### Quay Setup

The integration between OpenShift and Quay makes extensive use of the Quay RESTFul API. To allow for other methods aside from user credentials to be used for authenticating against the API, an application is required to be created. When an application is created, an OAuth token is generated which can be used by the operator in OpenShift. Applications are created within a Quay organization. While an existing organization can be utilized, it is recommended that a dedicated organization be used.

1.	Login to Quay as a user with superuser access and select the organization for which the external application will be configured within.
2.	On the left hand side of the page, select **Applications**
3.	Create a new application by selecting the **Create New Application** button and entering a name for the application
4.	After the new application has been created, select the application that was created
5.	On the lefthand navigation bar, select the **Generate Token** button to create a new OAuth2 token
6.	Select all of the checkboxes to grant the necessary access that is needed for the integration
7.	Review the permissions that will be assigned and then select the **Authorize Application** button
8.	Take note of the generated Access Token as it will be needed in the following section

### OpenShift Setup

To complete the process to begin to integrate OpenShift and Quay, several steps are required. Before beginning, ensure that you have the OpenShift Command Line tool installed and that you are logged into OpenShift as a cluster administrator. 

#### Create QuayIntegration Custom Resource Definition

The operator makes use of values defined within a `quayintegration` custom resource. For the custom resource to be used with the OpenShift cluster, the Custom Resource Definition must be applied by executing the following commnad:

```
oc create -f deploy/crds/redhatcop_v1alpha1_quayintegration_crd.yaml
```

#### MutatingWebhookConfiguration Support

Support for dynamic interception of API requests that are performed as part of OpenShift’s typical build process is facilitated through a [MutatingWebhookConfiguration](https://docs.openshift.com/container-platform/3.11/architecture/additional_concepts/dynamic_admission_controllers.html). A MutatingWebhookConfiguration allows for invoking an API running within a project on OpenShift when certain API requests are received. In particular, to support Quay’s integration with OpenShift, any new Build requests should be intercepted so that the output can be modified to target Quay instead of OpenShift’s integrated registry. 

Before a MutatingWebhookConfiguration can be created, there are a number of prerequisites that must be completed. 

First, webhook support must be enabled in the OpenShift cluster by modifying the _master-config.yaml_. On each Master instance, perform the following steps:

1. Edit the `/etc/origin/master/master-config.yaml` file and update the Admission plugins with the following content

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

2. Restart the OpenShift Master and Controllers pods

```
/usr/local/bin/master-restart api && /usr/local/bin/master-restart controllers
```

Next, Kubernetes requires that the webhook endpoint is secured via SSL using a certificate that makes use of the certificate authority for the cluster. Fortunately, OpenShift provides support for generating a certificate signed by the cluster. A script is available in the hack directory in the project repository called [webhook-create-signed-cert.sh](hack/webhook-create-signed-cert.sh) that automates the process for requesting a new certificate.

Execute the following command to request the certificate:

```
hack/webhook-create-signed-cert.sh --namespace=<namespace> --secret=quay-openshift-registry-operator --service=quay-openshift-registry-operator
```

The result will place the newly created private key and certificate within a secret within the secret specified. The secret will be mounted into the appropriate located within the operator as declared in the Deployment of the operator. 

Once the secret has been created, focus can shift to the MutatingWebhookConfiguration. The baseline can be found in the [deploy/mutatingwebhookconfiguration.yaml](deploy/mutatingwebhookconfiguration.yaml) file. The majority of the content should be in place to apply to the cluster. The one section that requires configuration is the caBundle property. This refers to the Certificate Authority (CA) for the OpenShift environment. The OpenShift CA is available as a _ConfigMap_ within the `kube-system` namespace.

Execute the following command to retrieve the CA and format the result as a single line so that it can be entered into the MutatingWebhookConfiguration resource:

```
oc get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 | tr -d '\n'
```

Replace the `${CA_BUNDLE}` variable in the `deploy/mutatingwebhookconfiguration.yaml` file.

Add the _MutatingWebhookConfiguration_ to the cluster by executing the following command:

```
oc create -f deploy/mutatingwebhookconfiguration.yaml
```

_Note: Until the operator is running, new requests for builds will fail since the webserver the MutatingWebhookConfiguration invokes is not available and a proper is response is required in order for the object to be persisted in etcd._

#### Deploying the Operator

With the CRD available on the cluster, the operator can be deployed. The operator must be deployed within a namespace. Create a new namespace if the desired namespace does not currently exist.

```
oc new-project quayintegratiom
```

_Note: Any namespace can be specified_

The integrated registry operator operates as any other pod running within OpenShift using a service account.  Specialized services are recommended against using the default service account and instead use a dedicated account for the specialized service.

Execute the following command to create a service account called `quay-openshift-integration-operator`

```
oc create -f deploy/service_account.yaml
```

By default, OpenShift service accounts operates with minimal permissions on the OpenShift API. A set of policies that the operator needs in order to properly operate can be created using a _ClusterRole_. 

Execute the following command to create the _ClusterRole_:

```
oc create -f deploy/clusterrole.yaml
```

To associate the ClusterRole with the previously created service account, a _ClusterRoleBinding_ can be created. A ClusterRoleBinding for can be found in the [deploy/clusterrole_binding.yaml](deploy/clusterrole_binding.yaml) file. Since a service account is created in a single user defined namespace, the name of the namespace must be provided in this file.

Edit the _deploy/clusterrole_binding.yaml_ file and **update the value in the namespace property**:

```
oc create -f deploy/clusterrole_binding.yaml
```

Finally, deploy the operator by executing the following command:

```
oc create -f deploy/operator.yaml
```

#### Create A Secret for the Quay OAuth Token

The Operator will use the previously obtained Access Token to communicate with Quay. Store this token within OpenShift as a secret.

Execute the following command to create a secret called `quay-integration` with a key called `token` containing the access token:

```
$ oc create secret generic quay-integration --from-literal=token=<access_token>
```

This token will be referenced in the following section.


#### Create the QuayIntegration Custom Resource

Finally, to complete the integration between OpenShift and Quay, a `QuayIntegration` custom resource needs to be created.

The following is an example of a basic definition of a `QuayIntegration` resource associated from the associated CRD.

```
apiVersion: redhatcop.redhat.io/v1alpha1
kind: QuayIntegration
metadata:
  name: example-quayintegration
spec:
  clusterID: openshift
  credentialsSecretName: <NAMESPACE>/<SECRET>
  quayHostname: https://<QUAY_URL>
```

The _clusterID_ is a value which should be unique across the entire ecosystem. This value is optional and defaults to `openshift`.

The _credentialsSecretName_ is a NamespacedName value of the secret containing the token that was previously created.

A baseline `QuayIntegration` Custom Resource can be found in _deploy/crds/redhatcop_v1alpha1_quayintegration_cr.yaml_. Update the values for your environment and execute the following command:

```
$ oc create -f deploy/crds/redhatcop_v1alpha1_quayintegration_cr.yaml
```

Organizations within Quay should be created for the related namespaces in OpenShift


## End to End Demonstration

This walkthrough demonstrates the creation of a new project and deployment of one of the out of the box (ootb) examples that is included with OpenShift. From an OpenShift perspective, no elevated rights are required and each of these steps can be accomplished by even the most basic user with rights to create resources on the platform.  Each of the OpenShift steps can be accomplished either from the web user interface, but also using the command line tool. For brevity, the command line option will be described in this guide.

First, ensure that you are logged into both the OpenShift cluster and into Quay:

Create a new OpenShift project called _e2e-demo_

```
oc new-project e2e-demo
```

When a new project is created in OpenShift, a new Organization is created in Quay. From the Quay homepage on the righthand side, confirm that an organization called _openshift_e2e-demo_. 

_Note: The “openshift” value may differ if the clusterId in the QuayIntegration resource had a different value_

Enter the organization and select **Robot Accounts** on the lefthand navigation bar. Three robot accounts should have been created as part of the new project creation process.

In OpenShift, confirm three secrets containing docker configurations associated with the applicable robot accounts were created by executing the following command

```
oc get secrets builder-quay-openshift deployer-quay-openshift default-quay-openshift
```

Each service account will be configured with the appropriate secret as a mountable secret and an image pull secret as shown using the following command

```
oc describe sa builder default deployer
```

As the project has been confirmed to be integrated, instantiate the example template “httpd-template” using the following command:

```
oc new-app --template=httpd-example
```

Similar to most of the other example applications, a BuildConfig, ImageStream, Service, Route and DeploymentConfig will be created. As soon as the ImageStream is created, an associated Repository will be created in Quay.

An ImageChangeTrigger for the BuildConfig will trigger a new Build when the Apache HTTPD image which is located in the openshift namespace is resolved. As the new build is created, the MutatingWebhookConfiguration will automatically rewrite the output to point at Quay. This can be confirmed by querying the output field of the build using the following command:

```
oc get build httpd-example-1 --template='{{ .spec.output.to.name }}'
```

Confirm the build completes successfully.

Once complete, navigate to the openshift_e2e-demo organization in Quay and select the httpd-example repository. 

On the lefthand side, select **Tags** and confirm a latest tag has been pushed to the registry.

On OpenShift, as soon as a build successfully completes, an import of the ImageStreamTag associated with the build is performed. Execute the following command to confirm the latest tag has been resolved.

```
oc describe is httpd-example
```

Confirm the _latest_ tag is pointing at the appropriate Quay repository.

With the ImageStream resolved, a new Deployment should have been triggered. Navigate to the URL output by the following command in a web browser:

```
oc get route httpd-example --template='{{ .spec.host }}'
```

If the sample webpage appears, the deployment was successful.

Finally, cleanup the resources created in this walkthough by deleting the project using the following command:

```
oc delete project e2e-demo
```

_Note: The command will wait until the project resources have been removed. This can be bypassed by adding the `--wait=false` to the above command_

Once the command completes, navigate to Quay and confirm the _openshift_e2e-demo_ organization is no longer available.

## Additional Considerations

### TLS Considerations

Best practices dictate that all communications between a client and an image registry be facilitated through secure means. Communications should all leverage HTTPS/TLS with a certificate trust between the parties. While Quay can be configured to serve in an insecure configuration, proper certificates should be utilized on the server and configured on the client. Follow the [OpenShift documentation](https://docs.openshift.com/container-platform/3.11/day_two_guide/docker_tasks.html#day-two-guide-managing-docker-certs) for adding and managing certificates at the container runtime level. 


