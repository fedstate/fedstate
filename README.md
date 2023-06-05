# FedState

**English** | [**简体中文**](./README_zh.md)

FedState refers to the Federation Stateful Service, which is mainly designed to
provide stateful service orchestration, scheduling, deployment, and automated
operation and maintenance capabilities in scenarios with multiple clouds, clusters, and data centers.

## Overview

FedState is used to deploy middleware, databases, and other stateful services that
need to be deployed in a multi-cloud environment to each member cluster through Karmada,
so that they can work normally and provide some advanced operation and maintenance capabilities.

## Architecture

![architecture](config/structure.png)

Component description:

- FedStateScheduler: A multi-cloud stateful service scheduler, based on the Karmada scheduler,
  adds some scheduling policies related to middleware services.
- FedState: The multi-cloud stateful service controller is mainly responsible for configuring
  various control clusters as needed and distributing them through Karmada.
- Member Operator: A concept that refers to a stateful service operator deployed in the control plane.
  FedState has built-in Mongo operators and will support more stateful services in the future.
- FedStateCR: A concept representing a multi-cloud stateful service instance.
- FedStateCR-Member: A concept representing an instance of a multi-cloud stateful service that
  has been distributed to the control plane.

### Current capabilities of FedState (using MongoDB Operator as an example)

- Add, delete, modify, and query multi-cloud MongoDB
- Scale multi-cloud MongoDB up and down
- Multi-cloud MongoDB failover
- Update multi-cloud MongoDB configuration and customize configurations
- Update multi-cloud MongoDB resources

## Quick Start

Deploy FedState to the Karmada Host cluster, deploy Member Operator to the member cluster,
create FedStateCR in the control plane, and wait for it to be created successfully until it can provide external services.

### Prerequisites

- Kubernetes v1.16+
- Karmada v1.4+
- Storage service
- Cluster VIP

### Environment preparation and Karmada installation

1. Prepare at least two Kubernetes clusters.
2. Use services such as Keepalived and HAProxy to manage the VIPs of the two clusters separately.
3. Deploy Karmada: [https://karmada.io/docs/installation/](https://karmada.io/docs/installation/).

### FedState installation (using MongoDB as an example)

> Note:
>
> - Karmada Host refers to the cluster where Karmada components are deployed.
> - Karmada Control refers to the Karmada control plane that interacts with the Karmada Apiserver.

1. (Optional) On the Karmada Host cluster, check whether the member cluster being managed has deployed estimator.

   ![get pod](config/Image.png)

   If the estimator is not enabled, the scheduler cannot estimate whether the resource settings
   of the multi-cloud stateful service can be met by the control plane.

   (Optional) Enable the estimator, and memberClusterName is the name of the member cluster on
   which you want to start the estimator:

   ```shell
   karmadactl addons enable  karmada-scheduler-estimator  -C {memberClusterName}
   ```

   (Optional) Check whether the name of the estimator service is suffixed with estimator-{clusterName}.

2. Deploy a custom resource interpreter on Karmada Control:

   ```shell
   kubectl apply -f customresourceinterpreter/pkg/deploy/customization.yaml
   ```

3. Deploy the control plane service on the Karmada Host cluster:

   ```shell
   kubectl create ns {your-namespace}
   kubectl create secret generic kubeconfig --from-file=/root/.kube/config -n {your-namespace} 

   # Check the Karmada ApiServer name in kubeconfig
   kubectl config get-contexts

   # Modify manager.yaml and change the value of KARMADA_CONTEXT_NAME to the Karmada Apiserver name
   vim config/manager/manager.yaml
   kubectl apply -f config/webhook/secret.yaml -n {your-namespace}
   kubectl apply -k config/deploy_contorlplane/. -n {your-namespace}
   ```

4. Deploy the webhook and control plane CRD on Karmada Control:

   ```shell
   kubectl label cluster <memberClusterName> VIP=<VIP of member cluster>
   kubectl apply -f config/webhook/external-svc.yaml
   kubectl apply -f config/crd/bases/.
   ```

5. Deploy the scheduler on the Karmada Host cluster:

   ```shell
   # Check the name of the Karmada Host Apiserver, Karmada Apiserver,
   # and the VIP address of the karmada Host in kubeconfig

   vim config/artifacts/deploy/deployment.yaml

   # Modify the following startup parameters to the values above:    

   - --karmada-context=<karmada>
   - --host-context=<10-29-14-21>
   - --host-VIP-address=<10.29.5.103>
   ```

6. Deploy the data planecontroller on the member cluster:

   ```shell
   kubectl apply -f config/crd/bases/mongodbs.yaml -n {your-namespace}
   kubectl apply -k config/deploy_dataplane/.
   ```

7. Deploy MultiCloudMongoDB on the control plane:

   ```shell
   kubectl apply -f config/sample/samples.yaml
   ```

   The `sample.yaml` is smilar to:

   ```yaml
   apiVersion: middleware.fedstate.io/v1alpha1
   kind: MultiCloudMongoDB
   metadata:
     name: multicloudmongodb-sample
   spec:
     # Number of replicas
     replicaset: 5
     # Monitoring configuration
     export:
       enable: false
     # Resource configuration
     resource:
       limits:
         cpu: "2"
         memory: 512Mi
       requests:
         cpu: "1"
         memory: 512Mi
     # Storage configuration
     storage:
       storageClass: managed-nfs-storage
       storageSize: 1Gi
     # Image configuration
     imageSetting:
       image: mongo:3.6
       imagePullPolicy: Always
   ```

8. Check the status of MultiCloudMongoDB and MongoDB on each controlled cluster:

   ![view status](config/multicloudstatus.png)

   Connect to the MongoDB replica set using the externalAddr address.
