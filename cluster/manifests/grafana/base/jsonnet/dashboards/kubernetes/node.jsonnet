local url = import "github.com/jsonnet-libs/xtd/url.libsonnet";
local g = import "github.com/grafana/grafonnet/gen/grafonnet-v10.4.0/main.libsonnet";

local common = import "common.libsonnet";

local filterByNamespacedPod = ' * on(namespace,pod) group_left group(kube_pod_created{namespace="${__data.fields["namespace"]}",pod="${__data.fields["pod"]}"}) by (namespace,pod)';

local joinAggregationLabel = ' * on(namespace,pod) group_left(w) group(label_join(kube_pod_info, "w", "/", "namespace", "pod")) by (namespace,pod,w)';

local cpuQuery = 'sum(rate(container_cpu_usage_seconds_total{container!=""}[$__range]) * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local cpuRequestsQuery = 'sum(kube_pod_all_container_resource_requests{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local cpuLimitsQuery = 'sum(kube_pod_all_container_resource_limits{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local memoryQuery = 'sum(container_memory_usage_bytes{container!=""} * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local memoryRequestsQuery = 'sum(kube_pod_all_container_resource_requests{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local memoryLimitsQuery = 'sum(kube_pod_all_container_resource_limits{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local networkReceiveQuery = 'sum(rate(container_network_receive_bytes_total[$__range]) * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local networkTransmitQuery = 'sum(rate(container_network_transmit_bytes_total[$__range]) * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local diskQuery = 'sum(container_fs_usage_bytes * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local inodeQuery = 'sum(container_fs_inodes_total * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local fdQuery = 'sum(container_file_descriptors * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local podsQuery = 'sum(kube_pod_status_phase{phase="Running"} * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';

local cpuThrottledQuery = 'sum(rate(container_cpu_cfs_throttled_seconds_total{container!=""}[$__range]) * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local memoryThrottledQuery = 'sum(rate(container_memory_failcnt{container!=""}[$__range]) * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local networkReceiveDroppedQuery = 'sum(rate(container_network_receive_packets_dropped_total[$__range]) * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local networkTransmitDroppedQuery = 'sum(rate(container_network_transmit_packets_dropped_total[$__range]) * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';

local podCreatedQuery = 'sum(kube_pod_created * 1000 * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';
local podPhaseQuery = 'sum(kube_pod_status_phase{phase="Running"} * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))';

local totalExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString("${node}"), "${node}"],
];
local totalQuotasPanel =
    g.panel.table.new("Quotas")
    + g.panel.table.panelOptions.withGridPos(w=24, h=4)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Requests")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuRequestsQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Limits")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuLimitsQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #E")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Requests")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryRequestsQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #F")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Limits")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryLimitsQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #G")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Receive")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkReceiveQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #H")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkTransmitQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #I")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Disk")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(diskQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #J")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Inode")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(inodeQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #K")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("File Descriptors")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(fdQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #L")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Pods")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(podsQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", cpuQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuRequestsQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuLimitsQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryRequestsQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryLimitsQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkReceiveQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkTransmitQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", diskQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", inodeQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", fdQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", podsQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
    ]);

local totalGaugesPanel =
    g.panel.gauge.new("Gauges")
    + g.panel.gauge.panelOptions.withGridPos(w=24, h=8)
    + g.panel.gauge.standardOptions.withUnit("percentunit")
    + g.panel.gauge.standardOptions.withMin(0)
    + g.panel.gauge.standardOptions.withMax(1)
    + g.panel.gauge.standardOptions.thresholds.withMode("absolute")
    + g.panel.gauge.standardOptions.thresholds.withSteps([
        g.panel.gauge.standardOptions.threshold.step.withColor("green") + g.panel.gauge.standardOptions.threshold.step.withValue(null),
        g.panel.gauge.standardOptions.threshold.step.withColor("yellow") + g.panel.gauge.standardOptions.threshold.step.withValue(0.7),
        g.panel.gauge.standardOptions.threshold.step.withColor("red") + g.panel.gauge.standardOptions.threshold.step.withValue(0.9),
    ])
    + g.panel.gauge.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", cpuQuery + ' / sum(kube_pod_all_container_resource_requests{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))') + g.query.prometheus.withLegendFormat("CPU Requests %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuQuery + ' / sum(kube_pod_all_container_resource_limits{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))') + g.query.prometheus.withLegendFormat("CPU Limits %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' / sum(kube_pod_all_container_resource_requests{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))') + g.query.prometheus.withLegendFormat("Memory Requests %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' / sum(kube_pod_all_container_resource_limits{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running * on(namespace,pod) group_left(node) group(kube_pod_info{node="${node}"}) by (node,namespace,pod))') + g.query.prometheus.withLegendFormat("Memory Limits %") + g.query.prometheus.withInstant(true),
    ]);

local perPodExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString("${node}"), "${node}"],
    [url.escapeString('${__data.fields[\\"namespace\\"]}'), '${__data.fields["namespace"]}'],
    [url.escapeString('${__data.fields[\\"pod\\"]}'), '${__data.fields["pod"]}'],
];
local perPodQuotasPanel =
    g.panel.table.new("Quotas")
    + g.panel.table.panelOptions.withGridPos(w=24, h=16)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("w")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Pod")
          + g.panel.table.standardOptions.withLinks([
              g.dashboard.link.link.new("Drill down", '/d/pod/pod?var-namespace=${__data.fields["namespace"]}&var-pod=${__data.fields["pod"]}&from=${__from}&to=${__to}')
              + g.dashboard.link.link.options.withTargetBlank(true)
              + g.dashboard.link.link.withTooltip("Drill down"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("namespace")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("pod")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Created Time")
          + g.panel.table.standardOptions.withUnit("dateTimeAsIso")
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Phase")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withMappings([
              g.panel.table.standardOptions.mapping.RangeMap.withType("range")
              + g.panel.table.standardOptions.mapping.RangeMap.options.withFrom(0)
              + g.panel.table.standardOptions.mapping.RangeMap.options.withTo(0)
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withColor("red")
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withText("NotRunning"),
              g.panel.table.standardOptions.mapping.RangeMap.withType("range")
              + g.panel.table.standardOptions.mapping.RangeMap.options.withFrom(1)
              + g.panel.table.standardOptions.mapping.RangeMap.options.withTo(1)
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withColor("green")
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withText("Running"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Requests")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuRequestsQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #E")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Limits")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuLimitsQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #F")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #G")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Requests")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryRequestsQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #H")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Limits")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryLimitsQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #I")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Receive")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkReceiveQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #J")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkTransmitQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #K")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Disk")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(diskQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #L")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Inode")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(inodeQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #M")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("File Descriptors")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(fdQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #N")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Pods")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(podsQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", podCreatedQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", podPhaseQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuRequestsQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuLimitsQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryRequestsQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryLimitsQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkReceiveQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkTransmitQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", diskQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", inodeQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", fdQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", podsQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "w",
            mode: "outer",
        })
    ]);

local perPodSaturationPanel =
    g.panel.table.new("Saturation")
    + g.panel.table.panelOptions.withGridPos(w=24, h=16)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("w")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Pod")
          + g.panel.table.standardOptions.withLinks([
              g.dashboard.link.link.new("Drill down", '/d/pod/pod?var-namespace=${__data.fields["namespace"]}&var-pod=${__data.fields["pod"]}&from=${__from}&to=${__to}')
              + g.dashboard.link.link.options.withTargetBlank(true)
              + g.dashboard.link.link.withTooltip("Drill down"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("namespace")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("pod")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Created Time")
          + g.panel.table.standardOptions.withUnit("dateTimeAsIso")
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Phase")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withMappings([
              g.panel.table.standardOptions.mapping.RangeMap.withType("range")
              + g.panel.table.standardOptions.mapping.RangeMap.options.withFrom(0)
              + g.panel.table.standardOptions.mapping.RangeMap.options.withTo(0)
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withColor("red")
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withText("NotRunning"),
              g.panel.table.standardOptions.mapping.RangeMap.withType("range")
              + g.panel.table.standardOptions.mapping.RangeMap.options.withFrom(1)
              + g.panel.table.standardOptions.mapping.RangeMap.options.withTo(1)
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withColor("green")
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withText("Running"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Throttled")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuThrottledQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Throttled")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryThrottledQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #E")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Receive Dropped")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkReceiveDroppedQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #F")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit Dropped")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkTransmitDroppedQuery + ' by (namespace,pod)' + filterByNamespacedPod, "${__from}", "${__to}", perPodExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", podCreatedQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", podPhaseQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuThrottledQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryThrottledQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkReceiveDroppedQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkTransmitDroppedQuery + ' by (namespace,pod)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "w",
            mode: "outer",
        })
    ]);

g.dashboard.new("Node")
+ g.dashboard.withUid("node")
+ g.dashboard.time.withFrom(value="now-1h")
+ g.dashboard.withTimezone("browser")
+ g.dashboard.graphTooltip.withSharedCrosshair()
+ g.dashboard.withAnnotations([
    common.alertAnnotation,
])
+ g.dashboard.withVariables([
    g.dashboard.variable.query.new("node")
    + g.dashboard.variable.query.withDatasource("prometheus", "Prometheus")
    + g.dashboard.variable.query.queryTypes.withLabelValues("node", "kube_pod_info"),
])
+ g.dashboard.withPanels([
    g.panel.row.new("Summary") + g.panel.row.withCollapsed(false),
    totalQuotasPanel,
    totalGaugesPanel,
    g.panel.row.new("Summary per pod") + g.panel.row.withCollapsed(false),
    perPodQuotasPanel,
    perPodSaturationPanel,
])
