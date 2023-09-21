# KEDA Redis scaler

KEDA external scaler for Redis using Lua script

## Install

```sh
helm install keda-redis-scaler ghcr.io/sun-asterisk-research/helm-charts/keda-redis-scaler
```

## Usage

Trigger Specification

```yaml
triggers:
- type: external
  metadata:
    scalerAddress: <address>
    address: <redis-address>
    host: <redis-host>
    port: <redis-port>
    enableTLS: <true-or-false>
    unsafeSSL: <true-or-false>
    username: <redis-username>
    password: <redis-password>
    database: <redis-database>
    script: <lua-script>
    metricName: <metric-name>
    activationValue: <activation-value>
    targetValue: <target-value>
```

Example `ScaledObject`.

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: myapp
  namespace: myapp
spec:
  scaleTargetRef:
    name: myapp
  idleReplicaCount: 0
  minReplicaCount: 1
  maxReplicaCount: 2
  triggers:
  - type: external
    metadata:
      scalerAddress: keda-redis-scaler:9000
      address: my-redis:6379
      password: redis-password
      database: "0"
      script: |-
        return redis.call('LLEN', 'my-list')
      targetValue: "10"
```
