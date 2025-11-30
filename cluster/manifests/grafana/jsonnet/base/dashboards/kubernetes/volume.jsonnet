local url = import "github.com/jsonnet-libs/xtd/url.libsonnet";
local g = import "github.com/grafana/grafonnet/gen/grafonnet-v10.4.0/main.libsonnet";

local common = import "../../../common.libsonnet";

local filterByVolume = ' * on(namespace,persistentvolumeclaim) group_left group(kubelet_volume_stats_inodes{namespace="${__data.fields["namespace"]}",persistentvolumeclaim="${__data.fields["persistentvolumeclaim"]}"}) by (namespace,persistentvolumeclaim)';

local joinAggregationLabel = ' * on(namespace,persistentvolumeclaim) group_left(w) group(label_join(kubelet_volume_stats_inodes, "w", "/", "namespace", "persistentvolumeclaim")) by (namespace,persistentvolumeclaim,w)';

local diskQuery = 'sum(kubelet_volume_stats_used_bytes{cluster=""})';
local inodeQuery = 'sum(kubelet_volume_stats_inodes_used{cluster=""})';

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
          g.panel.table.standardOptions.withDisplayName("Disk")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(diskQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Inode")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(inodeQuery, "${__from}", "${__to}", totalExploreLinkMapping),
          ])
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", diskQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", inodeQuery) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
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
        g.query.prometheus.new("Prometheus", diskQuery + ' / sum(kubelet_volume_stats_capacity_bytes{cluster=""})') + g.query.prometheus.withLegendFormat("Disk / Max") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", inodeQuery + ' / sum(kubelet_volume_stats_inodes{cluster=""})') + g.query.prometheus.withLegendFormat("Inode / Max") + g.query.prometheus.withInstant(true),
    ]);

local perVolumeExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString('${__data.fields[\\"namespace\\"]}'), '${__data.fields["namespace"]}'],
    [url.escapeString('${__data.fields[\\"persistentvolumeclaim\\"]}'), '${__data.fields["persistentvolumeclaim"]}'],
];
local perVolumeQuotasPanel =
    g.panel.table.new("Quotas")
    + g.panel.table.panelOptions.withGridPos(w=24, h=8)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("w")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Workload")
        ),
        g.panel.table.standardOptions.override.byName.new("namespace")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("persistentvolumeclaim")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Disk")
          + g.panel.table.standardOptions.withUnit("bytes")
          + g.panel.table.standardOptions.withDecimals(2)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(diskQuery + ' by (namespace,persistentvolumeclaim)' + filterByVolume, "${__from}", "${__to}", perVolumeExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Inode")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(inodeQuery + ' by (namespace,persistentvolumeclaim)' + filterByVolume, "${__from}", "${__to}", perVolumeExploreLinkMapping),
          ])
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", diskQuery + ' by (namespace,persistentvolumeclaim)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", inodeQuery + ' by (namespace,persistentvolumeclaim)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "w",
            mode: "outer",
        })
    ]);

g.dashboard.new("Volume")
+ g.dashboard.withUid("volume")
+ g.dashboard.time.withFrom(value="now-1h")
+ g.dashboard.withTimezone("browser")
+ g.dashboard.graphTooltip.withSharedCrosshair()
+ g.dashboard.withAnnotations([
    common.alertAnnotation,
])
+ g.dashboard.withPanels([
    g.panel.row.new("Summary") + g.panel.row.withCollapsed(false),
    totalQuotasPanel,
    totalGaugesPanel,
    g.panel.row.new("Summary per volume") + g.panel.row.withCollapsed(false),
    perVolumeQuotasPanel,
])
