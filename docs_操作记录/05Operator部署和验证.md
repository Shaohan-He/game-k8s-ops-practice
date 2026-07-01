# Operator deployment and validation

This note records the v2.0 operator workflow for `game-k8s-ops-practice`. The goal is to keep the original static `k8s/` manifests available while adding an operator-managed path that is closer to real Kubernetes platform operations.

## What changed in v2.0

- A Go/controller-runtime operator is added under `operator/`.
- A namespaced CRD is added: `ops.shaohan.dev/v1alpha1`, kind `GamePlatform`.
- The sample `GamePlatform` in `operator/config/samples/ops_v1alpha1_gameplatform.yaml` describes the whole practice stack.
- The operator reconciles application services, Redis, MySQL, Kafka, monitoring, ingress, ConfigMaps, Secrets, and PVCs.
- `spec.imageTag` is the main release control for the four application Deployments.

## Deploy command

```bash
kubectl apply -k k8s-v2
```

This applies:

- `game-ops` namespace;
- operator CRD, RBAC, ServiceAccount, and controller Deployment;
- the sample `GamePlatform` resource.

## Validation commands

```bash
kubectl -n game-ops-system get pods
kubectl -n game-ops get gameplatform
kubectl -n game-ops get pods,svc,ingress
kubectl -n game-ops describe gameplatform game-platform
```

Expected result:

- the operator pod is Running;
- `GamePlatform` moves from `Progressing` to `Ready` after child Deployments become ready;
- four application Services exist and keep the same ports as v1;
- Prometheus, Grafana, and Alertmanager are created when `spec.monitoring.enabled` is true.

## Release validation

Change the sample image tag:

```yaml
spec:
  imageTag: "2.1.0"
```

Apply it again:

```bash
kubectl apply -f operator/config/samples/ops_v1alpha1_gameplatform.yaml
kubectl -n game-ops rollout status deployment/login-service
kubectl -n game-ops get gameplatform game-platform -o yaml
```

Expected result:

- the four application Deployment images are updated to `game-k8s-ops-practice/<service>:2.1.0`;
- status `serviceStatuses` reports each application image and rollout state;
- rollback remains manual with `kubectl rollout undo` or `scripts/rollback.sh`.

## Reconciliation validation

Delete one managed Deployment:

```bash
kubectl -n game-ops delete deployment login-service
kubectl -n game-ops get deployment login-service --watch
```

Expected result:

- the operator recreates `login-service`;
- the recreated Deployment keeps the configured image tag, probes, resource limits, security context, env sources, and Prometheus annotations.

## Notes

This operator is intentionally scoped for a practice and interview portfolio project. It demonstrates CRD design, reconciliation, owner references, rollout visibility, and self-healing behavior, but it is not a production database or Kafka operator.