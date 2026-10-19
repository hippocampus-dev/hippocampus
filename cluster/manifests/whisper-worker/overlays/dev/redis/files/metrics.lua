local result = {}

table.insert(result, "events")
table.insert(result, tostring(redis.call("LLEN", "events")))

return result
