
-- Redis Schema and Lua Scripts for Routing Service

-- Key Structure:
-- 1. Head registration: "routing:head:{head_id}" -> JSON encoded HeadService
-- 2. Head status: "routing:head:{head_id}:status" -> JSON encoded status
-- 3. Head list by model: "routing:model:{model_type}" -> Set of head_ids
-- 4. Head list by region: "routing:region:{region}" -> Set of head_ids
-- 5. Routing policy: "routing:policy" -> JSON encoded RoutingPolicy
-- 6. Last heartbeat: "routing:head:{head_id}:heartbeat" -> Unix timestamp

-- Lua Script: Register Head
local function register_head()
    local head_id = KEYS[1]
    local head_data = ARGV[1]
    local model_type = ARGV[2]
    local region = ARGV[3]

    -- Store head information
    redis.call('SET', 'routing:head:' .. head_id, head_data)

    -- Add to model index
    redis.call('SADD', 'routing:model:' .. model_type, head_id)

    -- Add to region index
    redis.call('SADD', 'routing:region:' .. region, head_id)

    return 1
end

-- Lua Script: Update Head Status
local function update_head_status()
    local head_id = KEYS[1]
    local status_data = ARGV[1]
    local timestamp = ARGV[2]

    -- Store status information
    redis.call('SET', 'routing:head:' .. head_id .. ':status', status_data)

    -- Update heartbeat
    redis.call('SET', 'routing:head:' .. head_id .. ':heartbeat', timestamp)

    return 1
end

-- Lua Script: Get Routing Decision
local function get_routing_decision()
    local model_type = KEYS[1]
    local region = KEYS[2]
    local strategy = KEYS[3]

    -- Get all candidate heads based on model type
    local candidate_heads = redis.call('SMEMBERS', 'routing:model:' .. model_type)

    -- If no candidates, return nil
    if #candidate_heads == 0 then
        return nil
    end

    -- Get head information for all candidates
    local head_infos = {}
    for i, head_id in ipairs(candidate_heads) do
        local head_data = redis.call('GET', 'routing:head:' .. head_id)
        local status_data = redis.call('GET', 'routing:head:' .. head_id .. ':status')
        local heartbeat = redis.call('GET', 'routing:head:' .. head_id .. ':heartbeat')

        if head_data and status_data and heartbeat then
            table.insert(head_infos, {
                head_id = head_id,
                head_data = head_data,
                status_data = status_data,
                heartbeat = heartbeat
            })
        end
    end

    -- Return JSON encoded head information
    return cjson.encode(head_infos)
end

-- Lua Script: Get All Heads
local function get_all_heads()
    local pattern = 'routing:head:*'
    local keys = redis.call('KEYS', pattern)

    local result = {}
    for i, key in ipairs(keys) do
        if not string.find(key, ':status') and not string.find(key, ':heartbeat') then
            local head_id = string.match(key, 'routing:head:(.+)')
            local head_data = redis.call('GET', key)
            local status_data = redis.call('GET', key .. ':status')
            local heartbeat = redis.call('GET', key .. ':heartbeat')

            if head_data then
                table.insert(result, {
                    head_id = head_id,
                    head_data = head_data,
                    status_data = status_data,
                    heartbeat = heartbeat
                })
            end
        end
    end

    return cjson.encode(result)
end

-- Register the scripts
redis.register_function('register_head', register_head)
redis.register_function('update_head_status', update_head_status)
redis.register_function('get_routing_decision', get_routing_decision)
redis.register_function('get_all_heads', get_all_heads)

