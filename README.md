# FedState

FedState是Federation Stateful Service的意思，主要的设计目标是为了解决在多云，多集群，多数据中心的场景下，有状态应用的编排，调度，部署和自动化运维等能力。

## 概述：

FedState对需要部署在多云环境上的中间件，数据库等有状态的服务通过Karmada下发到各个成员集群，使其正常工作的同时并提供一些高级运维能力。


## 架构：

![structure.png](config/structure.png)

FedState自身包含以下组件：

- FedInfraScheduler: 中间件等服务多云环境调度器，包含一些与中间件服务运行时状态相关的调度策略。
- FedInfraOps：中间件等服务控制器主要负责这些服务的按需配置与通过Karmada分发。

## 快速开始：

在Karmada Host集群，部署FedInfraOps和调度器。在workload集群，部署InfraServer Operator。创建联邦中间件实例。

### 先决条件：

- Kubernetes v1.16+
- Karmada v1.4+
- 存储服务
- 集群VIP

### 环境准备及Karmada安装：

1. 准备不少于两个Kubernetes集群。
2. 使用Keepalived，HAProxy等服务分别管理两个集群的VIP。
3. 部署Karmada：[https://karmada.io/docs/installation/](https://karmada.io/docs/installation/)。

### FedInfraOps以及FedInfraScheduler安装（以Mongo为例）：

1. 在Karmada Host集群，检查所纳管的成员集群是否部署了estimator。

```other
## 检查是否部署了karmada estimator组件。
kubectl get po -n karmada-system  | grep estimator
## 如果没有部署，进行karmada estimator组件部署：
kubectl-karmada addons
## 检查estimator service的名称后缀必须为{*}-estimator-{clustername}
kubectl get svc -n karmada-system | grep estimator
```

2. 在Karmada Host集群，部署自定义资源解释器。

```other
kubectl apply -f customization.yaml
```

3. 在Karmada Host集群，部署控制面服务 multicloud-mongo-operator。

```other
cd pkg/install/config
kubectl create ns federation-mongo-operator
kubectl create secret generic kubeconfig --from-file=/root/.kube/config -n federation-mongo-operator
## 在kubeconfig查看Karmada ApiServer名称
kubectl config get-contexts
## 修改manager.yaml将其中的KARMADA_CONTEXT_NAME值改为karmada apiserver名称
vim manager/manager.yaml
kubectl apply -f config/webhook/secret.yaml -n federation-mongo-operator
kubectl apply -k config/deploy_contorlplane/.
```

4. 在Karmada Host集群，部署webhook以及控制面的CRD。

```other
kubectl label cluster <成员clsuter名称> vip=<成员集群对应的Vip>
kubectl apply -f config/webhook/external-svc.yaml
kubectl apply -f config/crd/bases/.
```

5. 在Karmada Host集群，部署调度器。

```other
cd install/scheduler/artifacts
## 在kubeconfig查看Karmada Host Apiserver的名称以及Karmada Apiserver的名称和karmada Host的Vip地址
vim ./deployment.yaml
## 修改以下启动参数为上面的值           
- --karmada-context=karmada
- --host-context=10-29-14-21
- --host-vip-address=10.29.5.103
```

6. 在所有workload Cluster上，部署数据面控制器mongo-operator。

```other
cd insatll/config
kubectl apply -f config/crd/bases/mongodbs.yaml
kubectl apply -k config/deploy_dataplane/.
```

7. 在Karmada Host集群，创建MiddleCloudMongoDB实例。

```shell
kubectl apply -f config/sample/samples.yaml
```

