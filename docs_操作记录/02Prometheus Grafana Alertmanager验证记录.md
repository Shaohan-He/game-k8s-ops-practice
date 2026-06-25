# Prometheus、Grafana 与 Alertmanager 监控告警验证记录

## 一、记录说明

本文档记录 `game-k8s-ops-practice` 项目在 Docker Compose 环境下的监控告警验证过程。

本阶段主要目标是验证 Prometheus、Grafana、Alertmanager 是否正常运行，并确认业务服务指标采集、Grafana Dashboard 展示、服务不可用告警触发和服务恢复流程是否正常。

------

## 二、当前验证环境

### 1. 项目入口

由于当前主机 `8080` 端口已被 Kuboard 占用，项目 Nginx 入口已调整为：

```text
http://localhost:18080
```

### 2. 监控组件访问地址

| 组件           | 地址                     | 说明                   |
| -------------- | ------------------------ | ---------------------- |
| Prometheus     | `http://localhost:9090`  | 指标采集与告警规则计算 |
| Grafana        | `http://localhost:3000`  | 监控面板展示           |
| Alertmanager   | `http://localhost:9093`  | 告警接收与管理         |
| Nginx 业务入口 | `http://localhost:18080` | 游戏业务统一入口       |

------

## 三、监控组件可用性检查

![监控组件检查](操作记录图片\监控组件检查.png)

### 1. Prometheus 可用性检查

执行命令：

```bash
curl -i http://localhost:9090/-/ready
```

返回结果：

```text
HTTP/1.1 200 OK
Prometheus Server is Ready.
```

### 验证结论

Prometheus 服务正常，已经处于 Ready 状态，可以用于抓取业务指标和计算告警规则。

------

### 2. Grafana 可用性检查

执行命令：

```bash
curl -I http://localhost:3000
```

返回结果：

```text
HTTP/1.1 302 Found
Location: /login
```

### 验证结论

Grafana 服务正常。访问根路径时跳转到 `/login` 属于正常行为，说明 Grafana Web 服务已经启动。

------

### 3. Alertmanager 可用性检查

执行命令：

```bash
curl -I http://localhost:9093
```

返回结果：

```text
HTTP/1.1 405 Method Not Allowed
Allow: GET, OPTIONS
```

### 验证结论

Alertmanager 服务正常。当前使用 `curl -I` 发送的是 `HEAD` 请求，而 Alertmanager 根路径允许 `GET` 和 `OPTIONS`，因此返回 `405 Method Not Allowed` 不代表服务异常。

------

## 四、Grafana Dashboard 验证

![Grafana](操作记录图片\Grafana.png)

### 1. 登录 Grafana

访问地址：

```text
http://192.168.88.101:3000
```

默认账号：

```text
admin / admin
```

进入 Dashboard：

```text
Game Services Overview
```

### 2. 面板观察结果

Grafana Dashboard 中可以看到以下监控面板：

1. 服务存活实例。
2. 各服务请求速率。
3. 5xx 错误率。
4. P95 请求延迟。
5. 业务事件。

### 3. 当前观察结果

当前 Dashboard 显示：

```text
服务存活实例：4
```

说明四个核心业务服务均处于可观测状态：

```text
game-gateway
login-service
match-service
room-service
```

请求速率面板中可以看到各服务请求量变化，包括：

```text
game-gateway
login-service
match-service
room-service
```

P95 请求延迟面板中可以看到各服务接口延迟变化。

业务事件面板中可以看到登录、匹配、房间创建、加入、离开等业务事件指标。

### 验证结论

Grafana Dashboard 已成功加载项目监控面板，并能够展示服务存活、请求量、延迟和业务事件等指标。

------

## 五、业务指标采集验证

### 1. 产生业务请求

在验证 Grafana 数据前，已通过业务接口产生请求，包括：

```bash
curl -sS -X POST http://localhost:18080/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'
curl -sS -X POST http://localhost:18080/match \
  -H 'Content-Type: application/json' \
  -d '{"player_id":1,"mode":"ranked"}'
curl -sS -X POST http://localhost:18080/room/create \
  -H 'Content-Type: application/json' \
  -d '{"owner_id":1,"max_players":4}'
curl -sS -X POST http://localhost:18080/room/join \
  -H 'Content-Type: application/json' \
  -d '{"room_id":"a65af29d","player_id":2}'
curl -sS -X POST http://localhost:18080/room/leave \
  -H 'Content-Type: application/json' \
  -d '{"room_id":"a65af29d","player_id":2}'
```

### 2. 指标展示情况

Grafana 中已观察到以下指标变化：

```text
各服务请求速率出现曲线
P95 请求延迟出现曲线
业务事件面板出现登录、匹配、房间相关事件
```

### 验证结论

业务请求已经被应用服务暴露为 Prometheus 指标，并成功被 Grafana Dashboard 展示。

------

## 六、Prometheus Targets 验证

### 1. 执行命令

```bash
python3 - <<'PY'
import json
import urllib.request

url = "http://localhost:9090/api/v1/targets"
data = json.load(urllib.request.urlopen(url))

for t in data["data"]["activeTargets"]:
    job = t["labels"].get("job", "-")
    instance = t["labels"].get("instance", "-")
    health = t.get("health", "-")
    error = t.get("lastError", "")
    print(f"{job:25} {instance:30} {health:8} {error}")
PY
```

### 2. 服务恢复后的结果

```text
game-services             login-service:8001             up
game-services             match-service:8002             up
game-services             room-service:8003              up
game-services             game-gateway:8000              up
prometheus                prometheus:9090                up
```

### 3. 验证结论

Prometheus 已成功抓取四个业务服务和自身指标，所有核心 Targets 均为 `up`。

------

## 七、模拟服务不可用告警

## ![模拟服务不可用告警](操作记录图片\模拟服务不可用告警.png)

### 1. 操作目标

通过停止 `login-service`，模拟游戏业务服务不可用场景，验证 Prometheus 告警规则是否能检测到服务异常。

### 2. 停止 login-service

执行命令：

```bash
docker compose stop login-service
```

返回结果：

```text
Container game-k8s-ops-practice-login-service-1 Stopped
```

### 3. 查询 Prometheus 告警状态

执行命令：

```bash
python3 - <<'PY'
import json
import urllib.request

url = "http://localhost:9090/api/v1/alerts"
data = json.load(urllib.request.urlopen(url))

for a in data["data"]["alerts"]:
    name = a["labels"].get("alertname", "-")
    job = a["labels"].get("job", "-")
    state = a.get("state", "-")
    summary = a["annotations"].get("summary", "")
    print(f"{name:30} {job:20} {state:10} {summary}")
PY
```

### 4. 第一次查询结果

```text
GameServiceDown              game-services        pending     游戏服务实例不可用
```

### 5. 状态说明

`pending` 表示 Prometheus 已经检测到异常条件，但还没有达到告警规则中设置的持续时间，因此暂未进入真正告警触发状态。

这说明告警规则已经开始生效。

------

## 八、告警进入 firing 状态

### 1. 再次查询告警

等待一段时间后，再次执行告警查询命令。

### 2. 查询结果

```text
GameServiceDown              game-services        firing      游戏服务实例不可用
```

### 3. 状态说明

`firing` 表示告警条件已经持续达到规则要求，Prometheus 正式触发告警。

### 验证结论

停止 `login-service` 后，Prometheus 成功检测到服务不可用，并将 `GameServiceDown` 告警从 `pending` 推进到 `firing` 状态。

------

## 九、恢复 login-service

![模拟恢复服务](操作记录图片\模拟恢复服务.png)

### 1. 恢复服务

执行命令：

```bash
docker compose start login-service
```

返回结果：

```text
Container game-k8s-ops-practice-login-service-1 Started
```

同时依赖服务状态保持正常：

```text
redis    Healthy
kafka    Healthy
mysql    Healthy
```

### 2. 执行健康检查

```bash
bash scripts/health-check.sh
```

### 3. 健康检查结果

```text
[check] http://localhost:18080/health
{"status":"ok","service":"game-gateway","dependencies":{"redis":"up","mysql":"up","kafka":"up"}}

[check] http://localhost:8001/health
{"status":"ok","service":"login-service","dependencies":{"redis":"up","mysql":"up","kafka":"up"}}

[check] http://localhost:8002/health
{"status":"ok","service":"match-service","dependencies":{"redis":"up","mysql":"up","kafka":"up"}}

[check] http://localhost:8003/health
{"status":"ok","service":"room-service","dependencies":{"redis":"up","mysql":"up","kafka":"up"}}

全部健康检查请求成功。
```

### 验证结论

`login-service` 已恢复正常，所有业务服务健康检查均通过。

------

## 十、服务恢复后的 Prometheus Targets 验证

### 1. 执行命令

```bash
python3 - <<'PY'
import json
import urllib.request

url = "http://localhost:9090/api/v1/targets"
data = json.load(urllib.request.urlopen(url))

for t in data["data"]["activeTargets"]:
    job = t["labels"].get("job", "-")
    instance = t["labels"].get("instance", "-")
    health = t.get("health", "-")
    error = t.get("lastError", "")
    print(f"{job:25} {instance:30} {health:8} {error}")
PY
```

### 2. 查询结果

```text
game-services             login-service:8001             up
game-services             match-service:8002             up
game-services             room-service:8003              up
game-services             game-gateway:8000              up
prometheus                prometheus:9090                up
```

### 3. 验证结论

服务恢复后，Prometheus Targets 中所有业务服务均恢复为 `up`，说明指标采集链路恢复正常。

------

## 十一、本阶段验证结果汇总

### 1. 已验证内容

本阶段已完成以下验证：

1. Prometheus `/ready` 接口正常返回 `200 OK`。
2. Grafana 正常启动，并跳转到登录页面。
3. Alertmanager 服务正常响应。
4. Grafana Dashboard `Game Services Overview` 成功展示。
5. Dashboard 中可以看到服务存活实例数为 4。
6. Dashboard 中可以看到各服务请求速率。
7. Dashboard 中可以看到 P95 请求延迟。
8. Dashboard 中可以看到登录、匹配、房间等业务事件。
9. Prometheus Targets 能抓取四个业务服务。
10. 停止 `login-service` 后，Prometheus 告警进入 `pending` 状态。
11. 等待后，`GameServiceDown` 告警进入 `firing` 状态。
12. 恢复 `login-service` 后，健康检查通过。
13. 服务恢复后，Prometheus Targets 全部恢复为 `up`。

------

## 十二、本阶段排查与运维收获

### 1. Prometheus 状态检查

通过：

```bash
curl -i http://localhost:9090/-/ready
```

可以快速确认 Prometheus 是否处于可用状态。

### 2. Targets 检查

通过 Prometheus API 查询 Targets，可以判断服务指标是否被正常抓取：

```text
up 表示抓取正常
down 表示抓取失败
```

### 3. Grafana 验证

Grafana 中如果图表没有数据，应优先检查：

```text
1. Prometheus Targets 是否为 up
2. 业务接口是否产生了请求
3. Dashboard 时间范围是否正确
4. Grafana 数据源是否指向 Prometheus
```

### 4. 告警状态理解

Prometheus 告警状态分为：

```text
inactive：告警条件未满足
pending：告警条件已满足，但未达到持续时间
firing：告警条件持续达到规则要求，正式触发告警
```

### 5. 服务不可用排查路径

模拟停止 `login-service` 后，可以按以下顺序排查：

```text
1. docker compose ps 查看服务状态
2. Prometheus Targets 查看抓取状态
3. Prometheus Alerts 查看告警状态
4. health-check.sh 验证服务恢复
5. Grafana 观察服务存活实例数和请求曲线变化
```

------

## 十三、当前阶段总结

本阶段完成了 `game-k8s-ops-practice` 项目的监控告警验证。

通过本阶段操作，已确认以下链路正常：

```text
业务服务 /metrics
  ↓
Prometheus 指标抓取
  ↓
Grafana Dashboard 展示
  ↓
Prometheus 告警规则计算
  ↓
Alertmanager 接收告警
```

同时，已通过停止 `login-service` 的方式模拟服务不可用故障，并验证 `GameServiceDown` 告警可以从 `pending` 进入 `firing`，服务恢复后 Prometheus Targets 能重新恢复为 `up`。

本阶段说明项目已经具备基础可观测性能力，可以继续进入 Kubernetes 部署和版本发布回滚阶段。

------

