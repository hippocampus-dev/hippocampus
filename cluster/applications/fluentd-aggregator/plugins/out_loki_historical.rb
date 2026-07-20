require 'fluent/plugin/out_loki'

module Fluent
  module Plugin
    class LokiHistoricalOutput < LokiOutput
      Fluent::Plugin.register_output('loki_historical', self)

      config_param :historical_label_key, :string, default: 'version'

      def chunk_to_loki(chunk)
        streams = {}
        chunk.each do |time, record|
          result = line_to_loki(record)
          result[:labels][@historical_label_key] = Time.at(time).strftime('%Y%m%d%H')
          chunk_labels = result[:labels]
          streams[chunk_labels] ||= []
          streams[chunk_labels].push(
            'ts' => to_nano(time),
            'line' => result[:line]
          )
        end
        streams
      end

      # Explicitly define to satisfy fluentd's implement?(:buffered) check
      def write(chunk)
        super
      end
    end
  end
end
