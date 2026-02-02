GITHUB_TOKEN_REGEXP = /gh[pousr]_[a-zA-Z0-9]{36}/.freeze
OPENAI_API_KEY_REGEXP = /sk-[a-zA-Z0-9]{48}/.freeze

class Fluent::SecretFilter < Fluent::Filter
  Fluent::Plugin.register_filter("secret", self)

  def filter(_tag, _time, record)
    record["message"]&.gsub!(GITHUB_TOKEN_REGEXP, "[REDACTED]")
    record["message"]&.gsub!(OPENAI_API_KEY_REGEXP, "[REDACTED]")

    record
  end
end
