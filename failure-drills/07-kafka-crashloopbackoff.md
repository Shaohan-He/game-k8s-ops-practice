# 故障场景：Kafka CrashLoopBackOff

## 现象

Kafka 镜像能够创建容器，但 Pod 反复重启并进入：

```text
CrashLoopBackOff
```

上一实例日志包含：

```text
Received a fatal error while waiting for the controller
to acknowledge that we are caught up
```

## 影响范围

- 登录、匹配和房间行为事件无法发送到 Kafka。
- 应用启动阶段初始化 Producer 失败时，业务服务也可能无法启动。
- `/health` 中 Kafka 状态为 `down`，监控可能触发依赖异常。

## 排查步骤

1. 查看 Kafka Pod 的重启次数和状态。
2. 读取当前容器及 `--previous` 日志。
3. Describe Pod，排除 OOM、探针和镜像问题。
4. 检查 KRaft 角色、监听器、Controller 名称和 quorum voter 配置。
5. 核对单节点环境中的 Controller 地址能否在进程启动阶段正确解析和访问。

## 关键命令

```bash
kubectl -n game-ops get pods -l app=kafka
kubectl -n game-ops describe pod -l app=kafka

kubectl -n game-ops logs deployment/kafka --tail=100
kubectl -n game-ops logs deployment/kafka --previous --tail=100

kubectl -n game-ops get deployment kafka -o yaml
```

## 根因

Kafka 使用单节点 KRaft 模式，原 Controller quorum voter 配置为：

```text
KAFKA_CONTROLLER_QUORUM_VOTERS=1@kafka:9093
```

该地址与当前单 Pod 启动环境的实际监听方式不匹配，Controller 无法完成启动确认，进程退出后被 Kubernetes 反复重启。

## 恢复方案

在本次练习环境中，将配置调整为：

```text
KAFKA_CONTROLLER_QUORUM_VOTERS=1@localhost:9093
```

重新应用配置并滚动重启：

```bash
kubectl apply -f k8s/infra.yaml
kubectl -n game-ops rollout restart deployment/kafka
kubectl -n game-ops rollout status deployment/kafka
kubectl -n game-ops logs deployment/kafka --tail=100
```

Kafka 恢复后，再重启依赖它的业务服务：

```bash
kubectl -n game-ops rollout restart deployment/game-gateway
kubectl -n game-ops rollout restart deployment/login-service
kubectl -n game-ops rollout restart deployment/match-service
kubectl -n game-ops rollout restart deployment/room-service
```

## 复盘总结

- `CrashLoopBackOff` 是结果，不是根因，必须查看 `--previous` 日志。
- KRaft 的节点 ID、角色、监听器和 voter 配置必须一致。
- 单节点练习配置不能直接作为生产 Kafka 集群方案。
- 中间件恢复后，还要验证依赖它的应用 Producer 是否重新连接。

