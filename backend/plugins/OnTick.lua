-- Salience determines this plugin's run order relative to other OnTick
-- plugins (higher runs first). WhoAmI reports salience only — which hook
-- this plugin attaches to comes from the OnTick function name below.
Salience = 1

function WhoAmI()
    return Salience
end

-- OnTick fires once per daemon tick (daemonTickTime, consts.go). This
-- example exercises the full Go-native capability API (issue #284) as one
-- coherent workflow rather than four disconnected snippets:
--
--   1. http.get the current planetary K-index from NOAA's Space Weather
--      Prediction Center — a standard metric hams check for HF band
--      conditions (a high Kp means a disturbed geomagnetic field and
--      degraded HF propagation).
--   2. files.write it to the plugin scratch directory, caching the latest
--      fetch so it survives even if a later tick's http.get fails.
--   3. files.read the cache back — using the cache rather than the
--      just-fetched value means this step still works on a tick where the
--      fetch above failed, as long as some earlier tick succeeded.
--   4. http.post a beacon of the cached data out.
--
-- Each network call is wrapped in pcall: a transient failure (rate limit,
-- connectivity blip, the SSRF dial-time block if either URL ever resolved
-- somewhere it shouldn't) should skip that step for this tick, not error
-- the whole hook out — OnTick runs again next tick regardless.
function OnTick(entry)
    local ok, bandConditions = pcall(http.get, "https://services.swpc.noaa.gov/products/noaa-planetary-k-index.json")
    if ok then
        local wroteOk = pcall(files.write, "band-conditions.json", bandConditions)
        if not wroteOk then
            print("OnTick: failed to cache band conditions to scratch")
        end
    else
        print("OnTick: http.get for band conditions failed: " .. tostring(bandConditions))
    end

    local readOk, cached = pcall(files.read, "band-conditions.json")
    if readOk then
        local postOk = pcall(http.post, "https://httpbin.org/post", cached)
        if not postOk then
            print("OnTick: failed to beacon cached band conditions")
        end
    else
        print("OnTick: no cached band conditions available yet")
    end

    return entry
end
