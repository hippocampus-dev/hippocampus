local url = import "github.com/jsonnet-libs/xtd/url.libsonnet";
local g = import "github.com/grafana/grafonnet/gen/grafonnet-v10.4.0/main.libsonnet";

local common = import "common.libsonnet";

local filterByContainer = ' * on(container) group_left group(kube_pod_all_container_info{container="${__data.fields["container"]}"}) by (container)';

local cpuQuery = 'sum(rate(container_cpu_usage_seconds_total{namespace="${namespace}",pod="${pod}",container!=""}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';
local cpuRequestsQuery = 'sum(kube_pod_all_container_resource_requests{namespace="${namespace}",pod="${pod}",resource="cpu",unit="core"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container) > 0)';
local cpuLimitsQuery = 'sum(kube_pod_all_container_resource_limits{namespace="${namespace}",pod="${pod}",resource="cpu",unit="core"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container) > 0)';
local memoryQuery = 'sum(container_memory_usage_bytes{namespace="${namespace}",pod="${pod}",container!=""} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';
local memoryRequestsQuery = 'sum(kube_pod_all_container_resource_requests{namespace="${namespace}",pod="${pod}",resource="memory",unit="byte"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container) > 0)';
local memoryLimitsQuery = 'sum(kube_pod_all_container_resource_limits{namespace="${namespace}",pod="${pod}",resource="memory",unit="byte"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container) > 0)';
local networkReceiveQuery = 'sum(rate(container_network_receive_bytes_total{namespace="${namespace}",pod="${pod}"}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';
local networkTransmitQuery = 'sum(rate(container_network_transmit_bytes_total{namespace="${namespace}",pod="${pod}"}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';
local diskQuery = 'sum(container_fs_usage_bytes{namespace="${namespace}",pod="${pod}"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';
local inodeQuery = 'sum(container_fs_inodes_total{namespace="${namespace}",pod="${pod}"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';
local fdQuery = 'sum(container_file_descriptors{namespace="${namespace}",pod="${pod}"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';

local cpuThrottledQuery = 'sum(rate(container_cpu_cfs_throttled_seconds_total{namespace="${namespace}",pod="${pod}",container!=""}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';
local memoryThrottledQuery = 'sum(rate(container_memory_failcnt{namespace="${namespace}",pod="${pod}",container!=""}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';
local networkReceiveDroppedQuery = 'sum(rate(container_network_receive_packets_dropped_total{namespace="${namespace}",pod="${pod}"}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';
local networkTransmitDroppedQuery = 'sum(rate(container_network_transmit_packets_dropped_total{namespace="${namespace}",pod="${pod}"}[$__range]) * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))';

local containerConditionQuery = 'sum(kube_pod_all_container_status_ready{namespace="${namespace}",pod="${pod}"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container) > 0)';

local totalExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString("${namespace}"), "${namespace}"],
    [url.escapeString("${pod}"), "${pod}"],
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
        g.query.prometheus.new("Prometheus", cpuQuery + ' / sum(kube_pod_all_container_resource_requests{namespace="${namespace}",pod="${pod}",resource="cpu",unit="core"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))') + g.query.prometheus.withLegendFormat("CPU Requests %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuQuery + ' / sum(kube_pod_all_container_resource_limits{namespace="${namespace}",pod="${pod}",resource="cpu",unit="core"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))') + g.query.prometheus.withLegendFormat("CPU Limits %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' / sum(kube_pod_all_container_resource_requests{namespace="${namespace}",pod="${pod}",resource="memory",unit="byte"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))') + g.query.prometheus.withLegendFormat("Memory Requests %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' / sum(kube_pod_all_container_resource_limits{namespace="${namespace}",pod="${pod}",resource="memory",unit="byte"} * on(namespace,pod,container) group_left group(kube_pod_all_container_status_running{namespace="${namespace}",pod="${pod}"}) by (namespace,pod,container))') + g.query.prometheus.withLegendFormat("Memory Limits %") + g.query.prometheus.withInstant(true),
    ]);

local perContainerExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString("${namespace}"), "${namespace}"],
    [url.escapeString("${pod}"), "${pod}"],
    [url.escapeString('${__data.fields[\\"container\\"]}'), '${__data.fields["container"]}'],
];
local perContainerQuotasPanel =
    g.panel.table.new("Quotas")
    + g.panel.table.panelOptions.withGridPos(w=24, h=8)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("container")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Container")
          + g.panel.table.standardOptions.withMappings([
              g.panel.table.standardOptions.mapping.RegexMap.withType("regex")
              + g.panel.table.standardOptions.mapping.RegexMap.options.withPattern("^$")
              + g.panel.table.standardOptions.mapping.RegexMap.options.result.withText("POD"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Condition")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withMappings([
              g.panel.table.standardOptions.mapping.RangeMap.withType("range")
              + g.panel.table.standardOptions.mapping.RangeMap.options.withFrom(0)
              + g.panel.table.standardOptions.mapping.RangeMap.options.withTo(0)
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withColor("red")
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withText("NotReady"),
              g.panel.table.standardOptions.mapping.RangeMap.withType("range")
              + g.panel.table.standardOptions.mapping.RangeMap.options.withFrom(1)
              + g.panel.table.standardOptions.mapping.RangeMap.options.withTo(1)
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withColor("green")
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withText("Ready"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Requests")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuRequestsQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Limits")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuLimitsQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #E")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #F")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Requests")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryRequestsQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #G")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Limits")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryLimitsQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #H")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Receive")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkReceiveQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #I")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkTransmitQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #J")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Disk")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(diskQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #K")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Inode")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(inodeQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #L")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("File Descriptors")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(fdQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", containerConditionQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuRequestsQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuLimitsQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryRequestsQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryLimitsQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkReceiveQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkTransmitQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", diskQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", inodeQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", fdQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "container",
            mode: "outer",
        })
    ]);

local perContainerSaturationPanel =
    g.panel.table.new("Saturation")
    + g.panel.table.panelOptions.withGridPos(w=24, h=8)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("container")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Container")
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Condition")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withMappings([
              g.panel.table.standardOptions.mapping.RangeMap.withType("range")
              + g.panel.table.standardOptions.mapping.RangeMap.options.withFrom(0)
              + g.panel.table.standardOptions.mapping.RangeMap.options.withTo(0)
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withColor("red")
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withText("NotReady"),
              g.panel.table.standardOptions.mapping.RangeMap.withType("range")
              + g.panel.table.standardOptions.mapping.RangeMap.options.withFrom(1)
              + g.panel.table.standardOptions.mapping.RangeMap.options.withTo(1)
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withColor("green")
              + g.panel.table.standardOptions.mapping.RangeMap.options.result.withText("Ready"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Throttled")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuThrottledQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Throttled")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryThrottledQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Receive Dropped")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkReceiveDroppedQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #E")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit Dropped")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkTransmitDroppedQuery + ' by (container)' + filterByContainer, "${__from}", "${__to}", perContainerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", containerConditionQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuThrottledQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryThrottledQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkReceiveDroppedQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkTransmitDroppedQuery + ' by (container)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "container",
            mode: "outer",
        })
    ]);

local logsPanel =
    g.panel.logs.new("Logs")
    + g.panel.logs.panelOptions.withGridPos(w=24, h=16)
    + g.panel.logs.options.withShowTime(true)
    + g.panel.logs.queryOptions.withTargets([
        g.query.loki.new("Loki", '{grouping=~"kubernetes.${namespace}.*"} | pattern "<raw>" | json | kubernetes_pod_name = "${pod}" | line_format "{{ $structural_message := index (fromJson $.raw) \\"structural_message\\" }}{{ if $structural_message }}{{ range $k, $v := $structural_message }}\\033[1;33m{{ $k }}\\033[0m={{ $v }}\\n{{ end }}{{ else }}{{ $.message }}{{ end }}"')
    ]);

g.dashboard.new("Pod")
+ g.dashboard.withUid("pod")
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
    g.dashboard.variable.query.new("pod")
    + g.dashboard.variable.query.withDatasource("prometheus", "Prometheus")
    + g.dashboard.variable.query.queryTypes.withLabelValues("pod", 'kube_pod_info{namespace="$namespace"}'),
])
+ g.dashboard.withPanels([
    g.panel.row.new("Summary") + g.panel.row.withCollapsed(false),
    totalQuotasPanel,
    totalGaugesPanel,
    g.panel.row.new("Summary per container") + g.panel.row.withCollapsed(false),
    perContainerQuotasPanel,
    perContainerSaturationPanel,
    g.panel.row.new("Logs") + g.panel.row.withCollapsed(false),
    logsPanel,
])
