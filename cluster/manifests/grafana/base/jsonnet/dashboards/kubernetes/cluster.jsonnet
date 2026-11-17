local url = import "github.com/jsonnet-libs/xtd/url.libsonnet";
local g = import "github.com/grafana/grafonnet/gen/grafonnet-v10.4.0/main.libsonnet";

local common = import "common.libsonnet";

local filterByNamespace = ' * on(namespace) group_left group(kube_namespace_created{namespace="${__data.fields["namespace"]}"}) by (namespace)';
local filterByNode = ' * on(node) group_left group(kube_node_created{node="${__data.fields["node"]}"}) by (node)';

local joinNodeLabel = ' * on(namespace,pod) group_left(node) group(kube_pod_status_ready{condition="true"} * on(namespace,pod) group_left(node) group(kube_pod_info{node!=""}) by (node,namespace,pod)) by (namespace,pod,node)';

local cpuQuery = 'sum(rate(container_cpu_usage_seconds_total{container!=""}[$__range]) * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local cpuRequestsQuery = 'sum(kube_pod_all_container_resource_requests{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local cpuLimitsQuery = 'sum(kube_pod_all_container_resource_limits{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local memoryQuery = 'sum(container_memory_usage_bytes{container!=""} * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local memoryRequestsQuery = 'sum(kube_pod_all_container_resource_requests{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local memoryLimitsQuery = 'sum(kube_pod_all_container_resource_limits{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local networkReceiveQuery = 'sum(rate(container_network_receive_bytes_total[$__range]) * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local networkTransmitQuery = 'sum(rate(container_network_transmit_bytes_total[$__range]) * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local diskQuery = 'sum(container_fs_usage_bytes * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local inodeQuery = 'sum(container_fs_inodes_total * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local fdQuery = 'sum(container_file_descriptors * on(namespace,pod,container) group_left kube_pod_all_container_status_running)';
local podsQuery = 'sum(kube_pod_status_phase{phase="Running"})';

local nodeCPUMaxQuery = 'sum(rate(node_cpu_seconds_total{mode!="idle"}[$__range])' + joinNodeLabel + ' / on(node) group_left count(sum(node_cpu_seconds_total' + joinNodeLabel + ') by (node,cpu)) by (node))';
local nodeCPURequestsMaxQuery = 'sum(kube_pod_all_container_resource_requests{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running' + joinNodeLabel + ' / on(node) group_left count(sum(node_cpu_seconds_total' + joinNodeLabel + ') by (node,cpu)) by (node))';
local nodeCPULimitsMaxQuery = 'sum(kube_pod_all_container_resource_limits{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running' + joinNodeLabel + ' / on(node) group_left count(sum(node_cpu_seconds_total' + joinNodeLabel + ') by (node,cpu)) by (node))';
local nodeMemoryMaxQuery = 'sum((node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes' + joinNodeLabel + ')';
local nodeMemoryRequestsMaxQuery = 'sum(kube_pod_all_container_resource_requests{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running' + joinNodeLabel + ' / on(node) group_left sum(node_memory_MemTotal_bytes' + joinNodeLabel + ') by (node))';
local nodeMemoryLimitsMaxQuery = 'sum(kube_pod_all_container_resource_limits{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running' + joinNodeLabel + ' / on(node) group_left sum(node_memory_MemTotal_bytes' + joinNodeLabel + ') by (node))';
local nodeDiskMaxQuery = '1 - sum(sum(node_filesystem_avail_bytes{mountpoint="/"}' + joinNodeLabel + ') by (node) / on(node) group_left sum(node_filesystem_size_bytes{mountpoint="/"}' + joinNodeLabel + ') by (node))';
local nodeInodeMaxQuery = '1 - sum(sum(node_filesystem_files_free{mountpoint="/"}' + joinNodeLabel + ') by (node) / on(node) group_left sum(node_filesystem_files{mountpoint="/"}' + joinNodeLabel + ') by (node))';
local nodeConntrackMaxQuery = 'sum(sum(node_nf_conntrack_entries' + joinNodeLabel + ') by (node) / on(node) group_left sum(node_nf_conntrack_entries_limit' + joinNodeLabel + ') by (node))';
local nodePodsMaxQuery = 'sum(kube_pod_status_phase{phase!~"Succeeded|Failed"}' + joinNodeLabel + ' / on(node) group_left kube_node_status_capacity{resource="pods",unit="integer"}' + joinNodeLabel + ')';

local nodeRoleQuery = 'sum(kube_node_role)';
local nodeCreatedQuery = 'sum(kube_node_created * 1000)';
local nodeConditionQuery = 'sum(kube_node_status_condition{condition="Ready",status="true"})';
local nodeCPUQuery = 'sum(rate(node_cpu_seconds_total{mode!="idle"}[$__range])' + joinNodeLabel + ')';
local nodeCPURequestsQuery = 'sum(kube_pod_all_container_resource_requests{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running' + joinNodeLabel + ')';
local nodeCPULimitsQuery = 'sum(kube_pod_all_container_resource_limits{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running' + joinNodeLabel + ')';
local nodeMemoryQuery = 'sum((node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes)' + joinNodeLabel + ')';
local nodeMemoryRequestsQuery = 'sum(kube_pod_all_container_resource_requests{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running' + joinNodeLabel + ')';
local nodeMemoryLimitsQuery = 'sum(kube_pod_all_container_resource_limits{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running' + joinNodeLabel + ')';
local nodeNetworkReceiveQuery = 'sum(rate(node_network_receive_bytes_total[$__range])' + joinNodeLabel + ')';
local nodeNetworkTransmitQuery = 'sum(rate(node_network_transmit_bytes_total[$__range])' + joinNodeLabel + ')';
local nodeDiskQuery = 'sum((node_filesystem_size_bytes{mountpoint="/"} - node_filesystem_avail_bytes{mountpoint="/"})' + joinNodeLabel + ')';
local nodeInodeQuery = 'sum((node_filesystem_files{mountpoint="/"} - node_filesystem_files_free{mountpoint="/"})' + joinNodeLabel + ')';
local nodeConntrackQuery = 'sum(node_nf_conntrack_entries' + joinNodeLabel + ')';
local nodeARPQuery = 'sum(node_network_arp_entries' + joinNodeLabel + ')';
local nodePodsQuery = 'sum(kube_pod_status_phase{phase!~"Succeeded|Failed"}' + joinNodeLabel + ')';
local nodeKubeletCPUQuery = 'sum(label_replace(rate(container_cpu_usage_seconds_total{id="/system.slice/kubelet.service"}[$__range]), "node", "$1", "instance", "(.*)"))';
local nodeKubeletMemoryQuery = 'sum(label_replace(rate(container_memory_usage_bytes{id="/system.slice/kubelet.service"}[$__range]), "node", "$1", "instance", "(.*)"))';

local nodeCPUStalledQuery = 'sum(rate(node_pressure_cpu_waiting_seconds_total[$__range])' + joinNodeLabel + ')';
local nodeMemoryWaitingQuery = 'sum(rate(node_pressure_memory_waiting_seconds_total[$__range])' + joinNodeLabel + ')';
local nodeMemoryStalledQuery = 'sum(rate(node_pressure_memory_stalled_seconds_total[$__range])' + joinNodeLabel + ')';
local nodeDiskWaitingQuery = 'sum(rate(node_pressure_disk_waiting_seconds_total[$__range])' + joinNodeLabel + ')';
local nodeDiskStalledQuery = 'sum(rate(node_pressure_disk_stalled_seconds_total[$__range])' + joinNodeLabel + ')';
local nodeNetworkReceiveDroppedQuery = 'sum(rate(node_network_receive_drop_total[$__range])' + joinNodeLabel + ')';
local nodeNetworkTransmitDroppedQuery = 'sum(rate(node_network_transmit_drop_total[$__range])' + joinNodeLabel + ')';
local nodeSoftnetDroppedQuery = 'sum(rate(node_softnet_dropped_total[$__range])' + joinNodeLabel + ')';
local nodeSoftnetTimesSqueezedQuery = 'sum(rate(node_softnet_times_squeezed_total[$__range])' + joinNodeLabel + ')';

local totalExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
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
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkReceiveQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #H")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(2)
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
          g.panel.table.standardOptions.withDisplayName("inode")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(inodeQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #K")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("fd")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
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
        g.query.prometheus.new("Prometheus", cpuQuery + ' / sum(kube_pod_all_container_resource_requests{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running)') + g.query.prometheus.withLegendFormat("CPU Requests %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuQuery + ' / sum(kube_pod_all_container_resource_limits{resource="cpu",unit="core"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running)') + g.query.prometheus.withLegendFormat("CPU Limits %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' / sum(kube_pod_all_container_resource_requests{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running)') + g.query.prometheus.withLegendFormat("Memory Requests %") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' / sum(kube_pod_all_container_resource_limits{resource="memory",unit="byte"} * on(namespace,pod,container) group_left kube_pod_all_container_status_running)') + g.query.prometheus.withLegendFormat("Memory Limits %") + g.query.prometheus.withInstant(true),
    ]);

local perNamespaceExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString('${__data.fields[\\"namespace\\"]}'), '${__data.fields["namespace"]}'],
];
local perNamespaceQuotasPanel =
    g.panel.table.new("Quotas")
    + g.panel.table.panelOptions.withGridPos(w=24, h=16)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("namespace")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Namespace")
          + g.panel.table.standardOptions.withLinks([
              g.dashboard.link.link.new("Drill down", '/d/namespace/namespace?var-namespace=${__data.fields["namespace"]}&from=${__from}&to=${__to}')
              + g.dashboard.link.link.options.withTargetBlank(true)
              + g.dashboard.link.link.withTooltip("Drill down"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Requests")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuRequestsQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Limits")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(cpuLimitsQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #E")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Requests")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryRequestsQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #F")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Limits")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(memoryLimitsQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #G")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Receive")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkReceiveQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #H")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(networkTransmitQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #I")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Disk")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(diskQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #J")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("inode")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(inodeQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #K")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("fd")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(fdQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #L")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Pods")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(podsQuery + ' by (namespace)' + filterByNamespace, "${__from}", "${__to}", perNamespaceExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", cpuQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuRequestsQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", cpuLimitsQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryRequestsQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", memoryLimitsQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkReceiveQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", networkTransmitQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", diskQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", inodeQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", fdQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", podsQuery + ' by (namespace)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "namespace",
            mode: "outer",
        })
    ]);

local perNodeGaugesPanel =
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
        g.query.prometheus.new("Prometheus", nodeCPUMaxQuery) + g.query.prometheus.withLegendFormat("CPU / Max") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCPURequestsMaxQuery) + g.query.prometheus.withLegendFormat("CPU Requests / Max") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCPULimitsMaxQuery) + g.query.prometheus.withLegendFormat("CPU Limits / Max") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryMaxQuery) + g.query.prometheus.withLegendFormat("Memory / Max") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryRequestsMaxQuery) + g.query.prometheus.withLegendFormat("Memory Requests / Max") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryLimitsMaxQuery) + g.query.prometheus.withLegendFormat("Memory Limits / Max") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeDiskMaxQuery) + g.query.prometheus.withLegendFormat("Disk / Max") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeInodeMaxQuery) + g.query.prometheus.withLegendFormat("inode / Max") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeConntrackMaxQuery) + g.query.prometheus.withLegendFormat("conntrack / Max") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodePodsMaxQuery) + g.query.prometheus.withLegendFormat("Pods / Max") + g.query.prometheus.withInstant(true),
    ]);

local perNodeExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString('${__data.fields[\\"node\\"]}'), '${__data.fields["node"]}'],
];
local perNodeQuotasPanel =
    g.panel.table.new("Quotas")
    + g.panel.table.panelOptions.withGridPos(w=24, h=8)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("node")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Node")
          + g.panel.table.standardOptions.withLinks([
              g.dashboard.link.link.new("Drill down", '/d/node/node?var-node=${__data.fields["node"]}&from=${__from}&to=${__to}')
              + g.dashboard.link.link.options.withTargetBlank(true)
              + g.dashboard.link.link.withTooltip("Drill down"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("role")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Role")
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Created Time")
          + g.panel.table.standardOptions.withUnit("dateTimeAsIso")
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
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
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeCPUQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #E")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU / Max")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("percentunit")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(1)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.7),
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.9),
          ])
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeCPUMaxQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #F")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Requests")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeCPURequestsQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #G")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Requests / Max")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("percentunit")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(1)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.7),
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.9),
          ])
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeCPURequestsMaxQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #H")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Limits")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeCPULimitsQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #I")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Limits / Max")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("percentunit")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(1)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.7),
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.9),
          ])
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeCPULimitsMaxQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #J")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeMemoryQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #K")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory / Max")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("percentunit")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(1)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.7),
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.9),
          ])
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeMemoryMaxQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #L")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Requests")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeMemoryRequestsQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #M")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Requests / Max")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("percentunit")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(1)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.7),
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.9),
          ])
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeMemoryRequestsMaxQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #N")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Limits")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeMemoryLimitsQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #O")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Limits / Max")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("percentunit")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(1)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.7),
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.9),
          ])
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeMemoryLimitsMaxQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #P")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Receive")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeNetworkReceiveQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #Q")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit")
          + g.panel.table.standardOptions.withUnit("bps")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeNetworkTransmitQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #R")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Disk")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeDiskQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #S")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Disk / Max")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("percentunit")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(1)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.7),
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.9),
          ])
          + g.panel.table.standardOptions.withLinks([
            common.exploreLink(nodeDiskMaxQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #T")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("inode")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeInodeQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #U")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("inode / Max")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("percentunit")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(1)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.7),
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.9),
          ])
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeInodeMaxQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #V")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("conntrack")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeConntrackQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #W")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("conntrack / Max")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("percentunit")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(1)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.7),
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.9),
          ])
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeConntrackMaxQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #X")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("ARP")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeARPQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #Y")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Pods")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodePodsQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #Z")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Pods / Max")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("percentunit")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(1)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.7),
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.9),
          ])
          + g.panel.table.standardOptions.withLinks([
            common.exploreLink(nodePodsMaxQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #AA")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Kubelet CPU")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeKubeletCPUQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #AB")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Kubelet Memory")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
            common.exploreLink(nodeKubeletMemoryQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", nodeRoleQuery + ' by (node,role)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCreatedQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeConditionQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCPUQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCPUMaxQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCPURequestsQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCPURequestsMaxQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCPULimitsQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCPULimitsMaxQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryMaxQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryRequestsQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryRequestsMaxQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryLimitsQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryLimitsMaxQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeNetworkReceiveQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeNetworkTransmitQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeDiskQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeDiskMaxQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeInodeQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeInodeMaxQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeConntrackQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeConntrackMaxQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeARPQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodePodsQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodePodsMaxQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeKubeletCPUQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeKubeletMemoryQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "node",
            mode: "outer",
        })
    ]);

local perNodeSaturationPanel =
    g.panel.table.new("Saturation")
    + g.panel.table.panelOptions.withGridPos(w=24, h=8)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("node")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Node")
          + g.panel.table.standardOptions.withLinks([
              g.dashboard.link.link.new("Drill down", '/d/node/node?var-node=${__data.fields["node"]}&from=${__from}&to=${__to}')
              + g.dashboard.link.link.options.withTargetBlank(true)
              + g.dashboard.link.link.withTooltip("Drill down"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("role")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Role")
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Created Time")
          + g.panel.table.standardOptions.withUnit("dateTimeAsIso")
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Condition")
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
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CPU Stalled")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeCPUStalledQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #E")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Waiting")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeMemoryWaitingQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #F")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Memory Stalled")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeMemoryStalledQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #G")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Disk Waiting")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeDiskWaitingQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #H")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Disk Stalled")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeDiskStalledQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #I")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Receive Dropped")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeNetworkReceiveDroppedQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #J")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Network Transmit Dropped")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeNetworkTransmitDroppedQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #K")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Softnet Dropped")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeSoftnetDroppedQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #L")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Softnet Times Squeezed")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nodeSoftnetTimesSqueezedQuery + ' by (node)' + filterByNode, "${__from}", "${__to}", perNodeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", nodeRoleQuery + ' by (node,role)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCreatedQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeConditionQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeCPUStalledQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryWaitingQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeMemoryStalledQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeDiskWaitingQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeDiskStalledQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeNetworkReceiveDroppedQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeNetworkTransmitDroppedQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeSoftnetDroppedQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nodeSoftnetTimesSqueezedQuery + ' by (node)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "node",
            mode: "outer",
        })
    ]);

g.dashboard.new("Cluster")
+ g.dashboard.withUid("cluster")
+ g.dashboard.time.withFrom(value="now-1h")
+ g.dashboard.withTimezone("browser")
+ g.dashboard.graphTooltip.withSharedCrosshair()
+ g.dashboard.withAnnotations([
    common.alertAnnotation,
])
+ g.dashboard.withPanels([
    g.panel.row.new("Summary") + g.panel.row.withCollapsed(false),
    totalGaugesPanel,
    totalQuotasPanel,
    g.panel.row.new("Summary per namespace") + g.panel.row.withCollapsed(false),
    perNamespaceQuotasPanel,
    g.panel.row.new("Summary per node") + g.panel.row.withCollapsed(false),
    perNodeGaugesPanel,
    perNodeQuotasPanel,
    perNodeSaturationPanel,
])
