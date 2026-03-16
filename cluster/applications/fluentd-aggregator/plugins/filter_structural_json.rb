require "json"

class Fluent::StructuralJSONFilter < Fluent::Filter
  Fluent::Plugin.register_filter("structural_json", self)

  def filter(_tag, _time, record)
    begin
      record["structural_message"] = JSON.parse(record["message"])
    rescue JSON::ParserError
    else
      record.delete("message")
    end

    record
  end
end
