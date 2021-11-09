# Quay OpenShift Registry Operator

![CI](https://github.com/quay/quay-bridge-operator/workflows/CI/badge.svg?branch=master)

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
#### Deploying the Operator

The fastest method for deploying the operator is to deploy from OperatorHub. From the _Administrator_ perspective in the OpenShift Web Console, navigate to the _Operators_ tab, and then select _OperatorHub_.

Search for _Quay Bridge Operator_ and then select _Install_.

Select an Approval Strategy and then select _Install_ which will deploy the operator to the cluster.

#### Create A Secret for the Quay OAuth Token

The Operator will use the previously obtained Access Token to communicate with Quay. Store this token within OpenShift as a secret.

Execute the following command to create a secret called `quay-integration` in the `openshift-operators` namespace with a key called `token` containing the access token:

```
$ oc create secret -n openshift-operators generic quay-integration --from-literal=token=<access_token>
```


#### Create the QuayIntegration Custom Resource


Finally, to complete the integration between OpenShift and Quay, a `QuayIntegration` custom resource needs to be created. This can be completed in the Web Console or from the command line.

The following is an example of a basic definition of a `QuayIntegration` resource when created manually.

```
apiVersion: quay.redhat.com/v1
kind: QuayIntegration
metadata:
  name: quay
spec:
  clusterID: openshift
  credentialsSecret:
    namespace: openshift-operators
    name: quay-integration
  quayHostname: https://<QUAY_URL>
```

The _clusterID_ is a value which should be unique across the entire ecosystem. This value is optional and defaults to `openshift`.

The _credentialsSecret_ property refers to tis a NamespacedName value of the secret containing the token that was previously created.

Note: If Quay is using self signed certificates, the property `insecureRegistry: true`

A baseline `QuayIntegration` Custom Resource can be found in _config/samples/quay_v1_quayintegration.yaml_. Update the values for your environment and execute the following command:

```
$ oc create -f config/samples/quay_v1_quayintegration.yaml
```

Organizations within Quay should be created for the related namespaces from the OpenShift environment

## Development

The operator can be deployed manually without the use of the Operator Lifecycle Manager (OLM)

### Prerequisites

Since webhooks are a key component of the featureset, certificates must be configured in order to facilitate the communication between the API server and the operator. Deploy [Cert Manager](https://cert-manager.io/) using the OLM. Deploy the operator and create `CertManager` resource.

### Deployment

The first step is to install the CRD's to the cluster

```shell
make install
```

Next, Deploy the operator to the cluster

```shell
make deploy IMG=quay.io/quay/quay-bridge-operator:latest
```

### End to End Testing

Once you have an installed and configured Quay Bridge Operator on an OpenShift cluster, you can run end-to-end tests to verify that it works as expected

```
make test-e2e
```

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

Best practices dictate that all communications between a client and an image registry be facilitated through secure means. Communications should all leverage HTTPS/TLS with a certificate trust between the parties. While Quay can be configured to serve in an insecure configuration, proper certificates should be utilized on the server and configured on the client. Follow the [OpenShift documentation](https://docs.openshift.com/container-platform/4.7/security/certificate_types_descriptions/proxy-certificates.html) for adding and managing certificates at the container runtime level. 


