# 故障演练手册

本目录根据 [`docs_操作记录`](../docs_操作记录/) 中的实际部署错误和主动故障演练整理。每个场景独立成文，便于面试讲解、故障复现和日常排查。

| 编号 | 故障场景 | 类型 |
| --- | --- | --- |
| 01 | [宿主机 8080 端口被 Kuboard 占用](01-host-port-8080-conflict.md) | 实际部署故障 |
| 02 | [业务服务缺少 cryptography 依赖](02-mysql-auth-cryptography-missing.md) | 实际部署故障 |
| 03 | [Ingress Admission Webhook 异常](03-ingress-admission-webhook-failure.md) | 集群组件故障 |
| 04 | [MySQL PVC 长期 Pending](04-mysql-pvc-pending.md) | 存储故障 |
| 05 | [Redis 镜像拉取失败](05-redis-image-pull-backoff.md) | 镜像故障 |
| 06 | [MySQL 镜像拉取失败](06-mysql-image-pull-backoff.md) | 镜像故障 |
| 07 | [Kafka CrashLoopBackOff](07-kafka-crashloopbackoff.md) | 配置故障 |
| 08 | [login-service 不可用](08-login-service-unavailable.md) | 主动故障演练 |
| 09 | [错误镜像版本发布与回滚](09-bad-release-and-rollback.md) | 主动发布演练 |

> 文档中的默认密码、单节点中间件和 `emptyDir` 方案仅用于练习环境，不应直接用于生产环境。

