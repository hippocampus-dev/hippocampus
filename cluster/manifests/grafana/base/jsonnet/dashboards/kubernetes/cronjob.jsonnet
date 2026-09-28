local url = import "github.com/jsonnet-libs/xtd/url.libsonnet";
local g = import "github.com/grafana/grafonnet/gen/grafonnet-v10.4.0/main.libsonnet";

local common = import "common.libsonnet";

local filterByCronJob = ' * on(namespace,cronjob) group_left group(kube_cronjob_info{namespace="${__data.fields["namespace"]}",cronjob="${__data.fields["cronjob"]}"}) by (namespace,cronjob)';

local joinAggregationLabel = ' * on(namespace,cronjob) group_left(w) group(label_join(kube_cronjob_info, "w", "/", "namespace", "cronjob")) by (namespace,cronjob,w)';

local cronJobQuery = 'sum(kube_cronjob_info{namespace=~"${namespace}"})';
local nextScheduleQuery = 'sum(kube_cronjob_next_schedule_time{namespace=~"${namespace}"} * 1000)';
local lastScheduleQuery = 'sum(kube_cronjob_status_last_schedule_time{namespace=~"${namespace}"} * 1000)';

local schedulesExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString("${namespace}"), '${__data.fields["namespace"]}'],
    [url.escapeString('${__data.fields[\\"namespace\\"]}'), '${__data.fields["namespace"]}'],
    [url.escapeString('${__data.fields[\\"cronjob\\"]}'), '${__data.fields["cronjob"]}'],
];
local schedulesPanel =
    g.panel.table.new("Schedules")
    + g.panel.table.panelOptions.withGridPos(w=24, h=24)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(false)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("w")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("CronJob")
          + g.panel.table.standardOptions.withLinks([
              g.dashboard.link.link.new("Drill down", '/d/workload/workload?var-namespace=${__data.fields["namespace"]}&var-workload=${__data.fields["cronjob"]}&var-workload_type=cronjob&from=${__from}&to=${__to}')
              + g.dashboard.link.link.options.withTargetBlank(true)
              + g.dashboard.link.link.withTooltip("Drill down"),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("namespace")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("cronjob")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("schedule")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Schedule")
        ),
        g.panel.table.standardOptions.override.byName.new("Value #A")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
        g.panel.table.standardOptions.override.byName.new("Value #B")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Next")
          + g.panel.table.standardOptions.withUnit("dateTimeAsIso")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(nextScheduleQuery + ' by (namespace,cronjob)' + filterByCronJob, "${__from}", "${__to}", schedulesExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Value #C")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Last")
          + g.panel.table.standardOptions.withUnit("dateTimeAsIso")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(lastScheduleQuery + ' by (namespace,cronjob)' + filterByCronJob, "${__from}", "${__to}", schedulesExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", cronJobQuery + ' by (namespace,cronjob,schedule)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", nextScheduleQuery + ' by (namespace,cronjob)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
        g.query.prometheus.new("Prometheus", lastScheduleQuery + ' by (namespace,cronjob)' + joinAggregationLabel) + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
        + g.panel.table.queryOptions.transformation.withOptions({
            byField: "w",
            mode: "outer",
        })
    ]);

g.dashboard.new("CronJob")
+ g.dashboard.withUid("cron_job")
+ g.dashboard.time.withFrom(value="now-1h")
+ g.dashboard.withTimezone("browser")
+ g.dashboard.graphTooltip.withSharedCrosshair()
+ g.dashboard.withAnnotations([
    common.alertAnnotation,
])
+ g.dashboard.withVariables([
    g.dashboard.variable.query.new("namespace")
    + g.dashboard.variable.query.generalOptions.withCurrent("All")
    + g.dashboard.variable.query.selectionOptions.withIncludeAll(value=true)
    + g.dashboard.variable.query.selectionOptions.withMulti(value=true)
    + g.dashboard.variable.query.withDatasource("prometheus", "Prometheus")
    + g.dashboard.variable.query.queryTypes.withLabelValues("namespace", "kube_cronjob_info"),
])
+ g.dashboard.withPanels([
    g.panel.row.new("Schedules") + g.panel.row.withCollapsed(false),
    schedulesPanel,
])
