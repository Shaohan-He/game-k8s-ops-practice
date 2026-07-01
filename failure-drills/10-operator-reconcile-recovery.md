# Operator reconciliation recreates a deleted service Deployment

## Phenomenon

In v2.0 operator mode, a managed application Deployment such as `login-service` is manually deleted from the `game-ops` namespace. The service may briefly disappear from `kubectl get deployment`, and traffic to login endpoints may fail until the replacement Pod becomes ready.

## Impact

- `login-service` requests can fail during the short recovery window.
- `game-gateway` may return upstream errors for login-related routes.
- Prometheus may temporarily show the `login-service` target as down.

## Investigation steps

1. Confirm that the platform is managed by the operator.

   ```bash
   kubectl -n game-ops get gameplatform game-platform
   kubectl -n game-ops-system get pods
   ```

2. Delete the managed Deployment.

   ```bash
   kubectl -n game-ops delete deployment login-service
   ```

3. Watch the operator recreate it.

   ```bash
   kubectl -n game-ops get deployment login-service --watch
   kubectl -n game-ops get pods -l app=login-service
   ```

4. Check platform status.

   ```bash
   kubectl -n game-ops describe gameplatform game-platform
   kubectl -n game-ops get gameplatform game-platform -o yaml
   ```

## Key commands

```bash
kubectl -n game-ops get deployment login-service -o yaml
kubectl -n game-ops rollout status deployment/login-service
kubectl -n game-ops logs deployment/login-service
kubectl -n game-ops-system logs deployment/game-platform-operator-controller-manager
```

## Root cause

The Deployment was deleted manually, but it is a child resource owned by the `GamePlatform` custom resource. The operator watches owned Deployments and reconciles the expected state again, so the missing Deployment is recreated from the CR spec and built-in defaults.

## Recovery

No manual recreation is required. Wait for the operator to recreate the Deployment and for the new Pods to pass readiness probes.

If the new image is wrong or unavailable, rollback remains manual:

```bash
bash scripts/rollback.sh login-service
```

## Review summary

This drill verifies the main value of v2.0: the project is no longer only a static manifest deployment. The operator can detect drift, recreate managed resources, and expose rollout state through `GamePlatform.status` while keeping manual rollback explicit and understandable.