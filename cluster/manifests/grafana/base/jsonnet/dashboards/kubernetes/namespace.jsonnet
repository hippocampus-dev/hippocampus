local url = import "github.com/jsonnet-libs/xtd/url.libsonnet";
local g = import "github.com/grafana/grafonnet/gen/grafonnet-v10.4.0/main.libsonnet";

local common = import "common.libsonnet";

local filterByWorkload = ' * on(workload,workload_type) group_left group(mixin_pod_workload{workload="${__data.fields["workload"]}",workload_type="${__data.fields["workload_type"]}"}) by (workload,workload_type)';

local joinAggregationLabel = ' * on(workload,workload_type) group_left(w) group(label_join(mixin_pod_workload, "w", "/", "workload_type", "workload")) by (workload,workload_type,w)';

local cpuQuery = 'sum(rate(container_cpu_usage_seconds_total{namespace="${namespace}",container!=""}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local cpuRequestsQuery = 'sum(kube_pod_all_container_resource_requests{namespace="${namespace}",resource="cpu",unit="core"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local cpuLimitsQuery = 'sum(kube_pod_all_container_resource_limits{namespace="${namespace}",resource="cpu",unit="core"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local memoryQuery = 'sum(container_memory_usage_bytes{namespace="${namespace}",container!=""} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local memoryRequestsQuery = 'sum(kube_pod_all_container_resource_requests{namespace="${namespace}",resource="memory",unit="byte"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local memoryLimitsQuery = 'sum(kube_pod_all_container_resource_limits{namespace="${namespace}",resource="memory",unit="byte"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local networkReceiveQuery = 'sum(rate(container_network_receive_bytes_total{namespace="${namespace}"}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local networkTransmitQuery = 'sum(rate(container_network_transmit_bytes_total{namespace="${namespace}"}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local diskQuery = 'sum(container_fs_usage_bytes{namespace="${namespace}"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local inodeQuery = 'sum(container_fs_inodes_total{namespace="${namespace}"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local fdQuery = 'sum(container_file_descriptors{namespace="${namespace}"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local podsQuery = 'sum(kube_pod_status_phase{namespace="${namespace}",phase="Running"} * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';

local cpuThrottledQuery = 'sum(rate(container_cpu_cfs_throttled_seconds_total{namespace="${namespace}",container!=""}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local memoryThrottledQuery = 'sum(rate(container_memory_failcnt{namespace="${namespace}",container!=""}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local networkReceiveDroppedQuery = 'sum(rate(container_network_receive_packets_dropped_total{namespace="${namespace}"}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';
local networkTransmitDroppedQuery = 'sum(rate(container_network_transmit_packets_dropped_total{namespace="${namespace}"}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container) * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="${namespace}"})';

local totalExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString("${namespace}"), "${namespace}"],
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
        g.query.prometheus.new("Prometheus", cpuQuery + ' / sum(kube_pod_all_container_resource_requests{namespace="${namespace}",resource="cpu",unit="core"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container))') + g.query.prometheus.withLegendFormat("CPU Requests %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuQuery + ' / sum(kube_pod_all_container_resource_limits{namespace="${namespace}",resource="cpu",unit="core"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container))') + g.query.prometheus.withLegendFormat("CPU Limits %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' / sum(kube_pod_all_container_resource_requests{namespace="${namespace}",resource="memory",unit="byte"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container))') + g.query.prometheus.withLegendFormat("Memory Requests %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' / sum(kube_pod_all_container_resource_limits{namespace="${namespace}",resource="memory",unit="byte"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}"}) by (namespace,pod,container))') + g.query.prometheus.withLegendFormat("Memory Limits %") + g.query.prometheus.withInstant(true),
    ]);

local perWorkloadExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString("${namespace}"), "${namespace}"],
    [url.escapeString('${__data.fields[\\"workload\\"]}'), '${__data.fields["workload"]}'],
    [url.escapeString('${__data.fields[\\"workload_type\\"]}'), '${__data.fields["workload_type"]}'],
];
local perWorkloadQuotasPanel =
    g.panel.table.new("Quotas")
    + g.panel.table.panelOptions.withGridPos(w=24, h=16)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("w")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Workload")
          + g.panel.table.standardOptions.withLinks([
              g.dashboard.link.link.new("Drill down", '/d/workload/workload?var-namespace=${namespace}&var-workload=${__data.fields["workload"]}&var-workload_type=${__data.fields["workload_type"]}&from=${__from}&to=${__to}')
              + g.dashboard.link.link.options.withTargetBlank(true)
              + g.dashboard.link.link.withTooltip("Drill down"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("workload_type")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("workload")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Requests")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuRequestsQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Limits")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuLimitsQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #E")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Requests")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryRequestsQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #F")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Limits")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryLimitsQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #G")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Receive")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkReceiveQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #H")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkTransmitQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #I")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Disk")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(diskQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #J")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Inode")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(inodeQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #K")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("File Descriptors")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(fdQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #L")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Pods")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(podsQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", cpuQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuRequestsQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuLimitsQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryRequestsQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryLimitsQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkReceiveQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkTransmitQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", diskQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", inodeQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", fdQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", podsQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "w",
            mode: "outer",
        })
    ]);

local perWorkloadSaturationPanel =
    g.panel.table.new("Saturation")
    + g.panel.table.panelOptions.withGridPos(w=24, h=16)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("w")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Workload")
          + g.panel.table.standardOptions.withLinks([
              g.dashboard.link.link.new("Drill down", '/d/workload/workload?var-namespace=${namespace}&var-workload=${__data.fields["workload"]}&var-workload_type=${__data.fields["workload_type"]}&from=${__from}&to=${__to}')
              + g.dashboard.link.link.options.withTargetBlank(true)
              + g.dashboard.link.link.withTooltip("Drill down"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("workload_type")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("workload")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Throttled")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuThrottledQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Throttled")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryThrottledQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Receive Dropped")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkReceiveDroppedQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit Dropped")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkTransmitDroppedQuery + ' by (workload,workload_type)' + filterByWorkload, "${__from}", "${__to}", perWorkloadExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", cpuThrottledQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryThrottledQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkReceiveDroppedQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkTransmitDroppedQuery + ' by (workload,workload_type)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "w",
            mode: "outer",
        })
    ]);

g.dashboard.new("Namespace")
+ g.dashboard.withUid("namespace")
+ g.dashboard.time.withFrom(value="now-1h")
+ g.dashboard.withTimezone("browser")
+ g.dashboard.graphTooltip.withSharedCrosshair()
+ g.dashboard.withAnnotations([
    common.alertAnnotation,
])
+ g.dashboard.withVariables([
    g.dashboard.variable.query.new("namespace")
    + g.dashboard.variable.query.withDatasource("prometheus", "Prometheus")
    + g.dashboard.variable.query.queryTypes.withLabelValues("namespace", "kube_pod_info"),
])
+ g.dashboard.withPanels([
    g.panel.row.new("Summary") + g.panel.row.withCollapsed(false),
    totalQuotasPanel,
    totalGaugesPanel,
    g.panel.row.new("Summary per workload") + g.panel.row.withCollapsed(false),
    perWorkloadQuotasPanel,
    perWorkloadSaturationPanel,
])
