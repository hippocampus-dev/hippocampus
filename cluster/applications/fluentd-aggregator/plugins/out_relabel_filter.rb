class Fluent::RelabelFilterOutput < Fluent::Output
  Fluent::Plugin.register_output('relabel_filter', self)

  helpers :record_accessor
  helpers :event_emitter

  config_section :rule, param_name: :rules, multi: true do
    config_param :key, :string
    config_param :pattern, :regexp
    config_param :label, :string
    config_param :invert, :bool, default: false
  end

  def configure(conf)
    super

    @rules.map do |rule|
      {
        accessor: record_accessor_create(rule.key),
        pattern: rule.pattern,
        router: event_emitter_router(rule.label),
        invert: rule.invert
      }
    end
  end

  def process(tag, es)
    es.each do |time, record|
      @rules.each do |rule|
        rule[:router].emit(tag, time, record) if relabel_needed?(rule[:accessor].call(record).to_s, rule[:pattern], rule[:invert])
      end
    end
  end

  def relabel_needed?(target, pattern, invert)
    return false if target.empty? && !invert

    invert ^ pattern.match(target)
  end
end