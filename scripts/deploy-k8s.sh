#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NAMESPACE="${NAMESPACE:-game-ops}"

kubectl apply -f "${ROOT_DIR}/k8s/base/namespace.yaml"
kubectl apply -f "${ROOT_DIR}/k8s/base/configmap.yaml"
kubectl apply -f "${ROOT_DIR}/k8s/base/secret.yaml"

kubectl -n "${NAMESPACE}" create configmap mysql-init \
  --from-file=init.sql="${ROOT_DIR}/k8s/configs/init.sql" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "${NAMESPACE}" create configmap prometheus-config \
  --from-file=prometheus.yml="${ROOT_DIR}/k8s/configs/prometheus.yml" \
  --from-file=alerts.yml="${ROOT_DIR}/k8s/configs/alerts.yml" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "${NAMESPACE}" create configmap alertmanager-config \
  --from-file=alertmanager.yml="${ROOT_DIR}/k8s/configs/alertmanager.yml" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "${NAMESPACE}" create configmap grafana-provisioning \
  --from-file=datasource.yml="${ROOT_DIR}/k8s/configs/grafana-datasource.yml" \
  --from-file=dashboard.yml="${ROOT_DIR}/k8s/configs/grafana-dashboard-provider.yml" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "${NAMESPACE}" create configmap grafana-dashboard \
  --from-file=game-services-overview.json="${ROOT_DIR}/k8s/configs/game-services-overview.json" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl apply -f "${ROOT_DIR}/k8s/infra/infra.yaml"
kubectl apply -f "${ROOT_DIR}/k8s/apps/applications.yaml"
kubectl apply -f "${ROOT_DIR}/k8s/monitoring/monitoring.yaml"
kubectl apply -f "${ROOT_DIR}/k8s/networking/ingress.yaml"

kubectl -n "${NAMESPACE}" rollout status deployment/mysql --timeout=300s
kubectl -n "${NAMESPACE}" rollout status deployment/redis --timeout=180s
kubectl -n "${NAMESPACE}" rollout status deployment/kafka --timeout=300s
for deployment in game-gateway login-service match-service room-service; do
  kubectl -n "${NAMESPACE}" rollout status "deployment/${deployment}" --timeout=300s
done

kubectl -n "${NAMESPACE}" get pods,svc,ingress

