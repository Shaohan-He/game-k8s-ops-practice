# 故障场景：Ingress Admission Webhook 异常

## 现象

执行 Kubernetes 部署时，大部分资源创建成功，但 Ingress 创建失败：

```text
failed calling webhook "validate.nginx.ingress.kubernetes.io":
service "ingress-nginx-controller-admission" not found
```

## 影响范围

- `game.local` 无法通过 Ingress 对外提供服务。
- Deployment、Service 等内部资源不一定受影响。
- 可以通过 `port-forward` 临时访问，但正式入口链路不完整。

## 排查步骤

1. 确认失败对象仅为 Ingress。
2. 检查 IngressClass 是否存在。
3. 检查 ingress-nginx 命名空间中的 controller 和 admission Service。
4. 查看 `ValidatingWebhookConfiguration` 指向的 Service 是否真实存在。
5. 使用 Service 端口转发验证业务内部链路，排除应用故障。

## 关键命令

```bash
kubectl get ingressclass
kubectl get pods,svc -n ingress-nginx

kubectl get validatingwebhookconfigurations
kubectl describe validatingwebhookconfiguration ingress-nginx-admission

kubectl apply -f k8s/ingress.yaml

kubectl -n game-ops port-forward service/game-gateway 18000:8000
curl http://localhost:18000/health
```

## 根因

集群中残留了 ingress-nginx 的 Admission Webhook 配置，但 Webhook 引用的 `ingress-nginx-controller-admission` Service 不存在，导致 API Server 在校验 Ingress 时调用失败并拒绝创建资源。

## 恢复方案

临时验证方案：

```bash
kubectl -n game-ops port-forward service/game-gateway 18000:8000
```

正式恢复方案：

1. 修复或重新安装 ingress-nginx Controller。
2. 确认 admission Pod、Service、Secret 和 Webhook 配置完整。
3. 再次应用 Ingress 并检查地址。

```bash
kubectl apply -f k8s/ingress.yaml
kubectl -n game-ops get ingress
```

不要在生产环境中把永久删除 Admission Webhook 当作首选修复方式。

## 复盘总结

- Ingress 创建失败不等于业务 Pod 异常，应分层验证。
- Webhook 配置与其后端 Service 必须作为一个整体检查。
- `port-forward` 适合临时排障，不是长期流量入口。
- 集群插件卸载或重装后，要清理或恢复相关集群级资源。

