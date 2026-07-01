package controllers

const mysqlInitSQL = `CREATE DATABASE IF NOT EXISTS game_ops CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE game_ops;

CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL,
    password_hash CHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_username (username)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS login_records (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NULL,
    username VARCHAR(64) NOT NULL,
    action VARCHAR(16) NOT NULL,
    success TINYINT(1) NOT NULL,
    ip_address VARCHAR(45) NULL,
    login_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    logout_at TIMESTAMP NULL,
    PRIMARY KEY (id),
    KEY idx_login_records_user_time (user_id, login_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS rooms (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    room_id VARCHAR(32) NOT NULL,
    owner_id BIGINT UNSIGNED NOT NULL,
    max_players INT UNSIGNED NOT NULL DEFAULT 4,
    status VARCHAR(16) NOT NULL DEFAULT 'waiting',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_rooms_room_id (room_id),
    KEY idx_rooms_owner_id (owner_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS room_players (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    room_id VARCHAR(32) NOT NULL,
    player_id BIGINT UNSIGNED NOT NULL,
    joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    left_at TIMESTAMP NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_room_player (room_id, player_id),
    KEY idx_room_players_player_id (player_id)
) ENGINE=InnoDB;

INSERT INTO users (username, password_hash)
VALUES
    ('admin', SHA2('admin123', 256)),
    ('player1', SHA2('player123', 256))
ON DUPLICATE KEY UPDATE username = VALUES(username);
`

const prometheusConfig = `global:
  scrape_interval: 15s
  evaluation_interval: 15s
rule_files:
  - /etc/prometheus/alerts.yml
alerting:
  alertmanagers:
    - static_configs:
        - targets: ["alertmanager:9093"]
scrape_configs:
  - job_name: game-services
    metrics_path: /metrics
    static_configs:
      - targets:
          - game-gateway:8000
          - login-service:8001
          - match-service:8002
          - room-service:8003
  - job_name: prometheus
    static_configs:
      - targets: ["localhost:9090"]
`

const alertsConfig = `groups:
  - name: game-service-alerts
    rules:
      - alert: GameServiceDown
        expr: up{job="game-services"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Game service target is unavailable"
          description: "{{ $labels.instance }} has not been scraped successfully for 1 minute."
      - alert: GameServiceHighErrorRate
        expr: sum by (service) (rate(game_http_requests_total{status=~"5.."}[5m])) / clamp_min(sum by (service) (rate(game_http_requests_total[5m])), 0.001) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Game service 5xx error rate is above 5%"
      - alert: GameServiceHighLatency
        expr: histogram_quantile(0.95, sum by (le, service) (rate(game_http_request_duration_seconds_bucket[5m]))) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Game service P95 latency is above 1 second"
`

const alertmanagerConfig = `global:
  resolve_timeout: 5m
route:
  receiver: default
  group_by: ["alertname", "service"]
  group_wait: 10s
  group_interval: 5m
  repeat_interval: 2h
receivers:
  - name: default
`

const grafanaDatasourceConfig = `apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
`

const grafanaDashboardProviderConfig = `apiVersion: 1
providers:
  - name: game-ops
    folder: Game Ops
    type: file
    options:
      path: /var/lib/grafana/dashboards
`

const grafanaDashboardConfig = `{
  "editable": true,
  "panels": [
    {
      "type": "stat",
      "title": "Service targets up",
      "gridPos": {"h": 8, "w": 6, "x": 0, "y": 0},
      "targets": [{"expr": "sum(up{job=\"game-services\"})"}]
    },
    {
      "type": "timeseries",
      "title": "Request rate",
      "gridPos": {"h": 8, "w": 9, "x": 6, "y": 0},
      "targets": [{"expr": "sum by (service) (rate(game_http_requests_total[5m]))", "legendFormat": "{{service}}"}]
    },
    {
      "type": "timeseries",
      "title": "P95 latency",
      "gridPos": {"h": 8, "w": 9, "x": 15, "y": 0},
      "targets": [{"expr": "histogram_quantile(0.95, sum by (le, service) (rate(game_http_request_duration_seconds_bucket[5m])))", "legendFormat": "{{service}}"}]
    }
  ],
  "refresh": "15s",
  "schemaVersion": 40,
  "tags": ["game-ops"],
  "time": {"from": "now-1h", "to": "now"},
  "title": "Game Services Overview",
  "uid": "game-services-overview",
  "version": 1
}
`
