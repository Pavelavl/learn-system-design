-- add_task.lua
-- ARGV[1]: taskJSON (JSON-строка задачи)
-- ARGV[2]: priority (целочисленный приоритет)
-- ARGV[3]: executeAt (Unix-время выполнения, 0 для немедленных задач)
-- KEYS[1]: priority_queue (ключ приоритетной очереди)
-- KEYS[2]: delayed_queue (ключ отложенной очереди)

local taskJSON = ARGV[1]
local priority = tonumber(ARGV[2])
local executeAt = tonumber(ARGV[3]) or 0
local now = tonumber(redis.call('TIME')[1])

if not executeAt then
    return redis.error_reply("Invalid executeAt: not a number")
end

if not now then
    return redis.error_reply("Invalid current time: not a number")
end

if executeAt == 0 or executeAt <= now then
    -- Немедленная задача: добавляем в priority_queue
    redis.call('ZADD', KEYS[1], priority, taskJSON)
else
    -- Отложенная задача: добавляем в delayed_queue
    redis.call('ZADD', KEYS[2], executeAt, taskJSON)
end

return 1