local url = import "github.com/jsonnet-libs/xtd/url.libsonnet";
local g = import "github.com/grafana/grafonnet/gen/grafonnet-v10.4.0/main.libsonnet";

local common = import "../../../common.libsonnet";

local filterByAlert = ' * on(alertname) group_left group(ALERTS{alertname="${__data.fields["alertname"]}"}) by (alertname)';
local filterByDestinationWorkload = ' * on(destination_workload_namespace,destination_workload) group_left group(istio_requests_total{destination_workload_namespace="${__data.fields["destination_workload_namespace"]}",destination_workload="${__data.fields["destination_workload"]}"}) by (destination_workload_namespace,destination_workload)';
local filterBySourceWorkload = ' * on(source_workload_namespace,source_workload) group_left group(istio_requests_total{source_workload_namespace="${__data.fields["source_workload_namespace"]}",source_workload="${__data.fields["source_workload"]}"}) by (source_workload_namespace,source_workload)';
local filterByClusterName = ' * on(cluster_name) group_left group(envoy_cluster_outlier_detection_ejections_active{cluster_name="${__data.fields["cluster_name"]}"}) by (cluster_name)';

local joinDestinationWorkloadAggregationLabel = ' * on(destination_workload_namespace,destination_workload) group_left(w) group(label_join(istio_requests_total, "w", "/", "destination_workload_namespace", "destination_workload")) by (destination_workload_namespace,destination_workload,w)';
local joinSourceWorkloadAggregationLabel = ' * on(source_workload_namespace,source_workload) group_left(w) group(label_join(istio_requests_total, "w", "/", "source_workload_namespace", "source_workload")) by (source_workload_namespace,source_workload,w)';

local alertQuery = 'sum(ALERTS{alertstate="firing"})';
local nodesQuery = 'sum(kube_node_labels) by (node)';
local preemptedPodsQuery = 'sum(increase(kube_events_total{reason="Preempted"}[$__interval])) by (reason)';

local rpsQuery = 'sum(rate(istio_requests_total[$__range]))';
local errorsQuery = 'sum(rate(istio_requests_total{response_code=~"(500|502|503|504)"}[$__range]))';
local p50LatencyQuery = 'sum(histogram_quantile(0.5, rate(istio_request_duration_milliseconds_bucket[$__range])))';
local p90LatencyQuery = 'sum(histogram_quantile(0.5, rate(istio_request_duration_milliseconds_bucket[$__range])))';

local blackholeRPSQuery = 'sum(rate(istio_requests_total{destination_service_name="BlackHoleCluster"}[$__range]))';

local circuitBreakerQuery = 'sum(envoy_cluster_outlier_detection_ejections_active > 0)';

local locQuery = 'sum(loc{type!="Total"}) by (repository,type)';

local alertsExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString('${__data.fields[\\"alertname\\"]}'), '${__data.fields["alertname"]}'],
];
local alertsPanel =
    g.panel.table.new("Alerts")
    + g.panel.table.panelOptions.withGridPos(w=24, h=4)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("alertname")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Name")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(alertQuery + ' by (alertname)' + filterByAlert, "${__from}", "${__to}", alertsExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", alertQuery + ' by (alertname)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
    ]);

local nodesPanel =
    g.panel.timeSeries.new("Nodes")
    + g.panel.timeSeries.panelOptions.withGridPos(w=12, h=6)
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", nodesQuery) + g.query.prometheus.withLegendFormat("{{ node }}"),
    ]);

local preemptedPodsPanel =
    g.panel.timeSeries.new("Preempted Pods")
    + g.panel.timeSeries.panelOptions.withGridPos(w=12, h=6)
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", preemptedPodsQuery) + g.query.prometheus.withLegendFormat("{{ reason }}"),
    ]);

local requestsExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString('${__data.fields[\\"destination_workload_namespace\\"]}'), '${__data.fields["destination_workload_namespace"]}'],
    [url.escapeString('${__data.fields[\\"destination_workload\\"]}'), '${__data.fields["destination_workload"]}'],
];
local requestsPanel =
    g.panel.table.new("Requests")
    + g.panel.table.panelOptions.withGridPos(w=24, h=12)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("w")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Workload")
          + g.panel.table.standardOptions.withLinks([
              g.dashboard.link.link.new("Drill down", '/d/workload/workload?var-namespace=${__data.fields["destination_workload_namespace"]}&var-workload=${__data.fields["destination_workload"]}&from=${__from}&to=${__to}')
              + g.dashboard.link.link.options.withTargetBlank(true)
              + g.dashboard.link.link.withTooltip("Drill down"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("destination_workload_namespace")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("destination_workload")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("RPS")
          + g.panel.table.standardOptions.withUnit("reqps")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(rpsQuery + ' by (destination_workload_namespace,destination_workload)' + filterByDestinationWorkload, "${__from}", "${__to}", requestsExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Errors")
          + g.panel.table.standardOptions.withUnit("reqps")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(errorsQuery + ' by (destination_workload_namespace,destination_workload)' + filterByDestinationWorkload, "${__from}", "${__to}", requestsExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("P50 Latency")
          + g.panel.table.standardOptions.withUnit("ms")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(p50LatencyQuery + ' by (destination_workload_namespace,destination_workload)' + filterByDestinationWorkload, "${__from}", "${__to}", requestsExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #D")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("P90 Latency")
          + g.panel.table.standardOptions.withUnit("ms")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(p90LatencyQuery + ' by (destination_workload_namespace,destination_workload)' + filterByDestinationWorkload, "${__from}", "${__to}", requestsExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", rpsQuery + ' by (destination_workload_namespace,destination_workload)' + joinDestinationWorkloadAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", errorsQuery + ' by (destination_workload_namespace,destination_workload)' + joinDestinationWorkloadAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", p50LatencyQuery + ' by (destination_workload_namespace,destination_workload)' + joinDestinationWorkloadAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", p90LatencyQuery + ' by (destination_workload_namespace,destination_workload)' + joinDestinationWorkloadAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "w",
            mode: "outer",
        })
    ]);

local blackholeRequestsExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString('${__data.fields[\\"source_workload_namespace\\"]}'), '${__data.fields["source_workload_namespace"]}'],
    [url.escapeString('${__data.fields[\\"source_workload\\"]}'), '${__data.fields["source_workload"]}'],
];
local blackholeRequestsPanel =
    g.panel.table.new("Blackhole Requests")
    + g.panel.table.panelOptions.withGridPos(w=24, h=6)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("w")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Workload")
          + g.panel.table.standardOptions.withLinks([
              g.dashboard.link.link.new("Drill down", '/d/workload/workload?var-namespace=${__data.fields["source_workload_namespace"]}&var-workload=${__data.fields["source_workload"]}&from=${__from}&to=${__to}')
              + g.dashboard.link.link.options.withTargetBlank(true)
              + g.dashboard.link.link.withTooltip("Drill down"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("source_workload_namespace")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("source_workload")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Value")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("RPS")
          + g.panel.table.standardOptions.withUnit("reqps")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(blackholeRPSQuery + ' by (source_workload_namespace,source_workload,destination_service)' + filterBySourceWorkload, "${__from}", "${__to}", blackholeRequestsExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", blackholeRPSQuery + ' by (source_workload_namespace,source_workload)' + joinSourceWorkloadAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "w",
            mode: "outer",
        })
    ]);

local circuitBreakerExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString('${__data.fields[\\"cluster_name\\"]}'), '${__data.fields["cluster_name"]}'],
];
local circuitBreakerPanel =
    g.panel.table.new("Circuit Breaker")
    + g.panel.table.panelOptions.withGridPos(w=24, h=4)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("cluster_name")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Cluster Name")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(circuitBreakerQuery + ' by (cluster_name)' + filterByClusterName, "${__from}", "${__to}", circuitBreakerExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", circuitBreakerQuery + ' by (cluster_name)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
    ]);

local locPanel =
    g.panel.timeSeries.new("LOC")
    + g.panel.timeSeries.panelOptions.withGridPos(w=12, h=6)
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", locQuery) + g.query.prometheus.withLegendFormat("{{ repository }} {{ type }}"),
    ]);

g.dashboard.new("Overview")
+ g.dashboard.withUid("overview")
+ g.dashboard.time.withFrom(value="now-1d")
+ g.dashboard.withTimezone("browser")
+ g.dashboard.graphTooltip.withSharedCrosshair()
+ g.dashboard.withAnnotations([
    common.alertAnnotation,
])
+ g.dashboard.withPanels(std.flattenArrays([
    [
        g.panel.row.new("Summary") + g.panel.row.withCollapsed(false),
        alertsPanel,
        circuitBreakerPanel,
        requestsPanel,
        blackholeRequestsPanel,
    ],
    g.util.grid.makeGrid([
        nodesPanel,
        preemptedPodsPanel,
        locPanel,
    ], panelWidth=12, panelHeight=8),
]))
