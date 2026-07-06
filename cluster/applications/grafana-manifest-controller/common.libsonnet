local url = import "github.com/jsonnet-libs/xtd/url.libsonnet";
local g = import "github.com/grafana/grafonnet/gen/grafonnet-v10.4.0/main.libsonnet";

{
    exploreLink(expr, from, to, mappings = [])::
        local convertedExpr = std.strReplace(expr, "sum(rate(", "sum(irate(");

        local href = "/explore?schemaVersion=1&" + url.encodeQuery({
            panes: '{"left":{"datasource":"prometheus","queries":[{"refId":"A","expr":' + std.escapeStringJson(convertedExpr) + ',"range":true,"instant":true}],"range":{"from":"' + from + '","to":"' + to + '"}}}'
        });

        local apply(s, mapping) = std.strReplace(s, mapping[0], mapping[1]);
        local result = std.foldl(apply, mappings, href);

        g.dashboard.link.link.new("Explore", result)
        + g.dashboard.link.link.options.withTargetBlank(true)
        + g.dashboard.link.link.withTooltip("Explore"),

    alertAnnotation:
        g.dashboard.annotation.withName("Critical Alerts")
        + g.dashboard.annotation.withIconColor("rgba(255, 96, 96, 1)")
        + g.query.prometheus.new(
            "Prometheus",
            'sum(ALERTS{alertstate="firing",severity="critical"}) by (alertname)',
        )
        + { textFormat: "{{alertname}}" },

    revisionAnnotation:
        g.dashboard.annotation.withName("Revision")
        + g.dashboard.annotation.withIconColor("rgba(135, 206, 235, 1)")
        + g.query.prometheus.new(
            "Prometheus",
            'sum(kube_pod_annotations) by (annotation_revision) unless sum(kube_pod_annotations offset 1m) by (annotation_revision)',
        )
        + { textFormat: "{{annotation_revision}}" },
}
