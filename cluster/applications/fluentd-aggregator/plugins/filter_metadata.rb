class Fluent::MetadataFilter < Fluent::Filter
  Fluent::Plugin.register_filter("metadata", self)

  def filter(tag, _time, record)
    record["grouping"] = tag

    if record["kubernetes"]
      grouping = ["kubernetes"]
      if (namespace_name = record.dig("kubernetes", "namespace_name"))
        grouping << namespace_name
      end
      if (name = record.dig("kubernetes", "labels", "app.kubernetes.io/name") || record.dig("kubernetes", "labels", "k8s-app") || record.dig("kubernetes", "labels", "app") || record.dig("kubernetes", "labels", "component"))
        component = record.dig("kubernetes", "labels", "app.kubernetes.io/component")
        name = "#{name}-#{component}" unless component.to_s.empty?

        grouping << name
      end
      record["grouping"] = grouping.join(".")
    end

    record
  end
end
