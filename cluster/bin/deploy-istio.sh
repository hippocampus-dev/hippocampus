#!/usr/bin/env bash

set -e

kubectl get namespace istio-system > /dev/null 2>&1 || kubectl create namespace istio-system
kubectl label namespace/istio-system name=istio-system
kubectl label namespace/istio-system pod-security.kubernetes.io/enforce=privileged

kubectl get namespace istio-gateways > /dev/null 2>&1 || kubectl create namespace istio-gateways
kubectl label namespace/istio-gateways name=istio-gateways
kubectl label namespace/istio-gateways pod-security.kubernetes.io/enforce=baseline

curl -fsSL https://istio.io/downloadIstio | ISTIO_VERSION=${ISTIO_VERSION} sh -

(
  cd istio-${ISTIO_VERSION}
  cat <<EOS | bin/istioctl install -y -f -
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
spec:
  hub: ghcr.io/hippocampus-dev/hippocampus/mirror/docker.io/istio
  values:
    cni:
      ambient:
        enabled: true
        ipv6: false
    pilot:
      # istiod should not be horizontally scaled
      # https://github.com/istio/istio/issues/36763#issuecomment-1008512037
      autoscaleEnabled: false
      env:
        ENABLE_NATIVE_SIDECARS: "true"
        ENABLE_INBOUND_RETRY_POLICY: "true"
        ISTIO_META_DNS_CAPTURE: "true"
        ISTIO_META_DNS_AUTO_ALLOCATE: "true"
        PILOT_ENABLE_IP_AUTOALLOCATE: "true"
        PILOT_ENABLE_AMBIENT: "true"
        CA_TRUSTED_NODE_ACCOUNTS: "istio-system/ztunnel,kube-system/ztunnel"
    global:
      autoscalingv2API: true
      logAsJson: true
      proxy:
        autoInject: disabled
        logLevel: error
        resources:
          requests:
            cpu: 10m
            memory: 128Mi
          limits:
            cpu: 100m
            memory: 128Mi
    # https://istio.io/latest/docs/setup/additional-setup/gateway/#deploying-a-gateway
    gateways:
      istio-ingressgateway:
        injectionTemplate: gateway
      istio-egressgateway:
        injectionTemplate: gateway
  meshConfig:
    accessLogFile: /dev/null
    enableTracing: true
    defaultConfig:
      tracing: {}
      holdApplicationUntilProxyStarts: true
      proxyMetadata:
        EXIT_ON_ZERO_ACTIVE_CONNECTIONS: "true"
        ISTIO_META_ENABLE_HBONE: "true"
      proxyHeaders:
        metadataExchangeHeaders:
          mode: IN_MESH
      # https://www.envoyproxy.io/docs/envoy/latest/configuration/upstream/cluster_manager/cluster_stats
      proxyStatsMatcher:
        inclusionRegexps:
          - ".*outlier_detection.*"
        inclusionSuffixes:
          - upstream_cx_total
          - upstream_cx_active
          - upstream_cx_connect_fail
          - upstream_cx_connect_timeout
    localityLbSetting:
      enabled: false
    outboundTrafficPolicy:
      mode: REGISTRY_ONLY
    extensionProviders:
      - name: oauth2-proxy
        envoyExtAuthzHttp:
          service: oauth2-proxy.oauth2-proxy.svc.cluster.local
          port: "4180"
          includeRequestHeadersInCheck: ["cookie", "authorization"]
          includeAdditionalHeadersInCheck:
            "X-Auth-Request-Redirect": "%REQ(:SCHEME)%://%REQ(:AUTHORITY)%%REQ(:PATH)%"
          headersToUpstreamOnAllow: ["X-Auth-Request-User"]
          headersToDownstreamOnDeny: ["content-type", "set-cookie"]
      - name: otel-agent
        opentelemetry:
          service: otel-agent.otel.svc.cluster.local
          port: "4317"
      - name: prometheus
        prometheus: {}
      - name: envoy
        envoyFileAccessLog:
          path: /dev/stdout
          logFormat:
            labels:
              # http://ltsv.org/
              time: "%START_TIME%"
              host: "%DOWNSTREAM_REMOTE_ADDRESS_WITHOUT_PORT%"
              forwardedfor: "%REQ(X-FORWARDED-FOR)%"
              method: "%REQ(:METHOD)%"
              uri: "%REQ(:PATH)%"
              protocol: "%PROTOCOL%"
              status: "%RESPONSE_CODE%"
              size: "%BYTES_SENT%"
              reqsize: "%BYTES_RECEIVED%"
              referer: "%REQ(REFERER)%"
              ua: "%REQ(USER-AGENT)%"
              vhost: "%REQ(:AUTHORITY)%"
              reqtime: "%DURATION%"
              # Envoy specific fields
              respflag: "%RESPONSE_FLAGS%"
              # Additional fields
              scheme: "%REQ(:SCHEME)%"
              traceparent: "%REQ(TRACEPARENT)%"
              "x-client-ip": "%REQ(X-CLIENT-IP)%"
  components:
    base:
      enabled: true
    cni:
      enabled: true
    ztunnel:
      enabled: true
      k8s:
        resources:
          requests:
            cpu: 10m
            memory: 64Mi
          limits:
            cpu: 2000m
            memory: 1Gi
    pilot:
      enabled: true
      k8s:
        replicaCount: 2
        resources:
          limits:
            cpu: 2000m
            memory: 1Gi
          requests:
            cpu: 50m
            memory: 256Mi
        strategy:
          rollingUpdate:
            maxSurge: 25%
            maxUnavailable: 1
    ingressGateways:
      - name: istio-ingressgateway
        namespace: istio-gateways
        enabled: true
        k8s:
          service:
            externalTrafficPolicy: Local
            ports:
              - port: 15021
                targetPort: 15021
                name: status-port
              - port: 80
                targetPort: 8080
                name: http2
              - port: 443
                targetPort: 8443
                name: https
          resources:
            limits:
              cpu: 2000m
              memory: 1Gi
            requests:
              cpu: 20m
              memory: 128Mi
          strategy:
            rollingUpdate:
              maxSurge: 25%
              maxUnavailable: 1
          hpaSpec:
            maxReplicas: 5
            minReplicas: 2
            metrics:
              - type: Resource
                resource:
                  name: cpu
                  target:
                    type: Utilization
                    averageUtilization: 80
          overlays:
            - kind: Deployment
              name: istio-ingressgateway
              patches:
                - path: spec.template.spec.containers.[name:istio-proxy].lifecycle
                  value: |
                    preStop:
                      exec:
                        command: ["sleep", "3"]
      - name: cluster-local-gateway
        namespace: istio-gateways
        enabled: true
        label:
          app: cluster-local-gateway
          istio: cluster-local-gateway
        k8s:
          service:
            ports:
              - port: 80
                targetPort: 8080
                name: http2
          resources:
            limits:
              cpu: 2000m
              memory: 1Gi
            requests:
              cpu: 20m
              memory: 128Mi
          strategy:
            rollingUpdate:
              maxSurge: 25%
              maxUnavailable: 1
          hpaSpec:
            maxReplicas: 5
            minReplicas: 2
            metrics:
              - type: Resource
                resource:
                  name: cpu
                  target:
                    type: Utilization
                    averageUtilization: 80
          overlays:
            - kind: Deployment
              name: cluster-local-gateway
              patches:
                - path: spec.template.spec.containers.[name:istio-proxy].lifecycle
                  value: |
                    preStop:
                      exec:
                        command: ["sleep", "3"]
    egressGateways:
      - name: istio-egressgateway
        namespace: istio-gateways
        enabled: true
        k8s:
          resources:
            limits:
              cpu: 2000m
              memory: 1Gi
            requests:
              cpu: 20m
              memory: 128Mi
          strategy:
            rollingUpdate:
              maxSurge: 25%
              maxUnavailable: 1
          hpaSpec:
            maxReplicas: 5
            minReplicas: 2
            metrics:
              - type: Resource
                resource:
                  name: cpu
                  target:
                    type: Utilization
                    averageUtilization: 80
          overlays:
            - kind: Deployment
              name: istio-egressgateway
              patches:
                - path: spec.template.spec.containers.[name:istio-proxy].lifecycle
                  value: |
                    preStop:
                      exec:
                        command: ["sleep", "3"]
EOS
)

rm -rf istio-${ISTIO_VERSION}
