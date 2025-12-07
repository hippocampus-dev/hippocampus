local url = import "github.com/jsonnet-libs/xtd/url.libsonnet";
local g = import "github.com/grafana/grafonnet/gen/grafonnet-v10.4.0/main.libsonnet";

local common = import "../../../common.libsonnet";

local filterByScore = ' * on(namespace,pod,form_factor,url,category) group_left group(lighthouse_score{namespace="${__data.fields["namespace"]}",pod="${__data.fields["pod"]}",form_factor="${__data.fields["form_factor"]}",url="${__data.fields["url"]}",category="${__data.fields["category"]}"}) by (namespace,pod,form_factor,url,category)';

local scoreQuery = 'avg(lighthouse_score{namespace=~"${namespace}",pod=~"${pod}",form_factor=~"${form_factor}",url=~"${url}"})';
local lcpQuery = 'avg(lighthouse_audit{namespace=~"${namespace}",pod=~"${pod}",form_factor=~"${form_factor}",url=~"${url}",id="largest-contentful-paint"}) by (form_factor,url)';
local srtQuery = 'avg(lighthouse_audit{namespace=~"${namespace}",pod=~"${pod}",form_factor=~"${form_factor}",url=~"${url}",id="server-response-time"}) by (form_factor,url)';
local fidQuery = 'avg(lighthouse_audit{namespace=~"${namespace}",pod=~"${pod}",form_factor=~"${form_factor}",url=~"${url}",id="max-potential-fid"}) by (form_factor,url)';
local clsQuery = 'avg(lighthouse_audit{namespace=~"${namespace}",pod=~"${pod}",form_factor=~"${form_factor}",url=~"${url}",id="cumulative-layout-shift"}) by (form_factor,url)';

local perScoreExploreLinkMapping = [
    [url.escapeString("${__from}"), "${__from}"],
    [url.escapeString("${__to}"), "${__to}"],
    [url.escapeString("${namespace}"), '${__data.fields["namespace"]}'],
    [url.escapeString("${pod}"), '${__data.fields["pod"]}'],
    [url.escapeString("${form_factor}"), '${__data.fields["form_factor"]}'],
    [url.escapeString("${url}"), '${__data.fields["url"]}'],
    [url.escapeString('${__data.fields[\\"namespace\\"]}'), '${__data.fields["namespace"]}'],
    [url.escapeString('${__data.fields[\\"pod\\"]}'), '${__data.fields["pod"]}'],
    [url.escapeString('${__data.fields[\\"form_factor\\"]}'), '${__data.fields["form_factor"]}'],
    [url.escapeString('${__data.fields[\\"url\\"]}'), '${__data.fields["url"]}'],
    [url.escapeString('${__data.fields[\\"category\\"]}'), '${__data.fields["category"]}'],
];

local scorePanel =
    g.panel.table.new('lighthouse_score{namespace=~"$namespace",pod=~"$pod",form_factor=~"$form_factor",url=~"$url"}')
    + g.panel.table.panelOptions.withGridPos(w=24, h=12)
    + g.panel.table.fieldConfig.defaults.custom.withAlign("left")
    + g.panel.table.fieldConfig.defaults.custom.withFilterable(true)
    + g.panel.table.standardOptions.thresholds.withMode("absolute")
    + g.panel.table.standardOptions.thresholds.withSteps([])
    + g.panel.table.standardOptions.withOverrides([
        g.panel.table.standardOptions.override.byName.new("namespace")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Namespace")
        ),
        g.panel.table.standardOptions.override.byName.new("pod")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Pod")
        ),
        g.panel.table.standardOptions.override.byName.new("form_factor")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("FormFactor")
        ),
        g.panel.table.standardOptions.override.byName.new("url")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("URI")
        ),
        g.panel.table.standardOptions.override.byName.new("category")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Category")
        ),
        g.panel.table.standardOptions.override.byName.new("Value")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.standardOptions.withDisplayName("Score")
          + g.panel.table.fieldConfig.defaults.custom.cellOptions.TableColoredBackgroundCellOptions.withType("color-background")
          + g.panel.table.standardOptions.withUnit("short")
          + g.panel.table.standardOptions.withDecimals(0)
          + g.panel.table.standardOptions.withMin(0)
          + g.panel.table.standardOptions.withMax(100)
          + g.panel.table.standardOptions.thresholds.withMode("absolute")
          + g.panel.table.standardOptions.thresholds.withSteps([
              g.panel.table.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(null),
              g.panel.table.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(30),
              g.panel.table.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(60),
          ])
          + g.panel.table.standardOptions.withLinks([
              common.exploreLink(scoreQuery + ' by (namespace,pod,form_factor,url,category)' + filterByScore, "${__from}", "${__to}", perScoreExploreLinkMapping),
          ])
        ),
        g.panel.table.standardOptions.override.byName.new("Time")
        + g.panel.table.standardOptions.override.byType.withPropertiesFromOptions(
          g.panel.table.fieldConfig.defaults.custom.withHidden(true)
        ),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", scoreQuery + ' by (namespace,pod,form_factor,url,category)') + g.query.prometheus.withFormat("table") + g.query.prometheus.withInstant(true),
    ])
    + g.panel.table.queryOptions.withTransformations([
        g.panel.table.queryOptions.transformation.withId("joinByField")
    ]);

local lcpTextPanel =
    g.panel.text.new("Largest Contentful Paint")
    + g.panel.text.panelOptions.withGridPos(w=24, h=8)
    + g.panel.text.options.withMode("html")
    + g.panel.text.options.withContent('<div style="text-align: center"><p><img src="https://web.dev/static/articles/lcp/image/good-lcp-values.svg?hl=ja" style="height: 150px" /></p><p>読み込みのパフォーマンスを測定するための指標です。<br>優れたユーザー エクスペリエンスを提供するためには、ページの読み込みが開始されてからの LCP を 2.5 秒以内にする必要があります。</p></div>')
    + g.panel.text.panelOptions.withTransparent(true);

local latencyPanel =
    g.panel.timeSeries.new("Largest Contentful Paint / Server Response Time")
    + g.panel.timeSeries.panelOptions.withGridPos(w=24, h=12)
    + g.panel.timeSeries.fieldConfig.defaults.custom.thresholdsStyle.withMode("area")
    + g.panel.timeSeries.standardOptions.thresholds.withMode("absolute")
    + g.panel.timeSeries.standardOptions.thresholds.withSteps([
      g.panel.timeSeries.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
      g.panel.timeSeries.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(2500),
      g.panel.timeSeries.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(4000),
    ])
    + g.panel.timeSeries.standardOptions.withUnit("ms")
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", lcpQuery) + g.query.prometheus.withLegendFormat("LCP - {{ form_factor }}"),
        g.query.prometheus.new("Prometheus", srtQuery) + g.query.prometheus.withLegendFormat("SRT - {{ form_factor }}"),
    ]);

local fidTextPanel =
    g.panel.text.new("First Input Delay")
    + g.panel.text.panelOptions.withGridPos(w=24, h=8)
    + g.panel.text.options.withMode("html")
    + g.panel.text.options.withContent('<div style="text-align: center"><p><img src="https://web.dev/static/articles/fid/image/good-fid-values-25.svg?hl=ja" style="height: 150px" /></p><p>インタラクティブ性を測定するための指標です。<br>優れたユーザー エクスペリエンスを提供するためには、ページの FID を 100 ミリ秒以下にする必要があります。</p></div>')
    + g.panel.text.panelOptions.withTransparent(true);

local fidPanel =
    g.panel.timeSeries.new("First Input Delay")
    + g.panel.timeSeries.panelOptions.withGridPos(w=24, h=12)
    + g.panel.timeSeries.fieldConfig.defaults.custom.thresholdsStyle.withMode("area")
    + g.panel.timeSeries.standardOptions.thresholds.withMode("absolute")
    + g.panel.timeSeries.standardOptions.thresholds.withSteps([
      g.panel.timeSeries.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
      g.panel.timeSeries.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(100),
      g.panel.timeSeries.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(300),
    ])
    + g.panel.timeSeries.standardOptions.withUnit("ms")
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", fidQuery) + g.query.prometheus.withLegendFormat("FID - {{ form_factor }}"),
    ]);

local clsTextPanel =
    g.panel.text.new("Cumulative Layout Shift")
    + g.panel.text.panelOptions.withGridPos(w=24, h=8)
    + g.panel.text.options.withMode("html")
    + g.panel.text.options.withContent('<div style="text-align: center"><p><img src="https://web.dev/static/articles/cls/image/good-cls-values.svg?hl=ja" style="height: 150px" /></p><p>視覚的な安定性を測定するための指標です。<br>優れたユーザー エクスペリエンスを提供するためには、ページの CLS を 0.1 以下に維持する必要があります。</p></div>')
    + g.panel.text.panelOptions.withTransparent(true);

local clsPanel =
    g.panel.timeSeries.new("Cumulative Layout Shift")
    + g.panel.timeSeries.panelOptions.withGridPos(w=24, h=12)
    + g.panel.timeSeries.fieldConfig.defaults.custom.thresholdsStyle.withMode("area")
    + g.panel.timeSeries.standardOptions.thresholds.withMode("absolute")
    + g.panel.timeSeries.standardOptions.thresholds.withSteps([
      g.panel.timeSeries.standardOptions.threshold.step.withColor("green") + g.panel.table.standardOptions.threshold.step.withValue(null),
      g.panel.timeSeries.standardOptions.threshold.step.withColor("yellow") + g.panel.table.standardOptions.threshold.step.withValue(0.1),
      g.panel.timeSeries.standardOptions.threshold.step.withColor("red") + g.panel.table.standardOptions.threshold.step.withValue(0.25),
    ])
    + g.panel.table.queryOptions.withTargets([
        g.query.prometheus.new("Prometheus", clsQuery) + g.query.prometheus.withLegendFormat("CLS - {{ form_factor }}"),
    ]);

g.dashboard.new("Lighthouse")
+ g.dashboard.withUid("lighthouse")
+ g.dashboard.time.withFrom(value="now-1h")
+ g.dashboard.withTimezone("browser")
+ g.dashboard.graphTooltip.withSharedCrosshair()
+ g.dashboard.withAnnotations([
    common.alertAnnotation,
    common.revisionAnnotation,
])
+ g.dashboard.withVariables([
    g.dashboard.variable.query.new("namespace")
    + g.dashboard.variable.query.generalOptions.withCurrent("All")
    + g.dashboard.variable.query.selectionOptions.withIncludeAll(value=true)
    + g.dashboard.variable.query.selectionOptions.withMulti(value=true)
    + g.dashboard.variable.query.withDatasource("prometheus", "Prometheus")
    + g.dashboard.variable.query.queryTypes.withLabelValues("namespace", "lighthouse_score"),
    g.dashboard.variable.query.new("pod")
    + g.dashboard.variable.query.generalOptions.withCurrent("All")
    + g.dashboard.variable.query.selectionOptions.withIncludeAll(value=true)
    + g.dashboard.variable.query.selectionOptions.withMulti(value=true)
    + g.dashboard.variable.query.withDatasource("prometheus", "Prometheus")
    + g.dashboard.variable.query.queryTypes.withLabelValues("pod", 'lighthouse_score{namespace="$namespace"}'),
    g.dashboard.variable.query.new("form_factor")
    + g.dashboard.variable.query.generalOptions.withCurrent("All")
    + g.dashboard.variable.query.selectionOptions.withIncludeAll(value=true)
    + g.dashboard.variable.query.selectionOptions.withMulti(value=true)
    + g.dashboard.variable.query.withDatasource("prometheus", "Prometheus")
    + g.dashboard.variable.query.queryTypes.withLabelValues("form_factor", 'lighthouse_score{namespace="$namespace",pod="$pod"}'),
    g.dashboard.variable.query.new("url")
    + g.dashboard.variable.query.generalOptions.withCurrent("All")
    + g.dashboard.variable.query.selectionOptions.withIncludeAll(value=true)
    + g.dashboard.variable.query.selectionOptions.withMulti(value=true)
    + g.dashboard.variable.query.withDatasource("prometheus", "Prometheus")
    + g.dashboard.variable.query.queryTypes.withLabelValues("url", 'lighthouse_score{namespace="$namespace",pod="$pod",form_factor=~"$form_factor"}'),
])
+ g.dashboard.withPanels([
    g.panel.row.new("Score") + g.panel.row.withCollapsed(false),
    scorePanel,
    g.panel.row.new("Core Web Vitals") + g.panel.row.withCollapsed(false),
    lcpTextPanel,
    latencyPanel,
    fidTextPanel,
    fidPanel,
    clsTextPanel,
    clsPanel,
])
