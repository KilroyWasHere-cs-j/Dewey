-- Defines type of plugin
Type = "test"

-- Defines the salience of the plugin
Salience = 1

-- print(arg[1])

-- Identify the plugin type and salience to the backend
function WhoAmI()
    return Type, Salience
end

-- Hook for backend to register
function Begin()
    print("Apluginsinit Begin")
end

-- Hook for backend to call on shutdown
function End()
    -- This code will be run before server end
    -- If this is a filter plugin don't add anything here, it won't be called
end

local http_request = require("http.request")
local http_util = require("http.util")
local ltn12 = require("ltn12") -- Used for body handling if using LuaSocket

-- ── Config ────────────────────────────────────────────────────────────────────
local DEFAULT_BASE = "http://localhost:8080"
local BASE_URL = arg[1] or DEFAULT_BASE
BASE_URL = BASE_URL:gsub("/+$", "")

-- ── Helpers ───────────────────────────────────────────────────────────────────
local PASS = "\27[92m✔\27[0m"
local FAIL = "\27[91m✘\27[0m"
local INFO = "\27[94m·\27[0m"
local results = {}

local function record(name, passed, detail)
    local tag = passed and PASS or FAIL
    local detail_str = detail and (" — " .. detail) or ""
    print(string.format("  %s  %s%s", tag, name, detail_str))
    table.insert(results, { name = name, passed = passed })
end

local function summary()
    local total = #results
    local passed_count = 0
    for _, res in ipairs(results) do
        if res.passed then passed_count = passed_count + 1 end
    end

    print(string.rep("─", 56))
    print(string.format("  Results: %d/%d passed", passed_count, total))
    if passed_count < total then
        print("  Failed tests:")
        for _, res in ipairs(results) do
            if not res.passed then
                print(string.format("    %s  %s", FAIL, res.name))
            end
        end
    end
    print(string.rep("─", 56) .. "\n")
    return passed_count == total
end

local function sleep(n)
    os.execute("sleep " .. tonumber(n))
end

local function quick_get(path)
    local req = http_request.new_from_uri(BASE_URL .. path)
    local headers, stream = req:go(5)
    if not headers then return 0 end
    return tonumber(headers:get(":status"))
end

local function upload(filename, content, content_type)
    -- Minimal multipart/form-data construction
    local boundary = "----LuaBoundary" .. os.time()
    local body = string.format(
        "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\nContent-Type: %s\r\n\r\n%s\r\n--%s--\r\n",
        boundary, filename, content_type or "application/octet-stream", content, boundary
    )

    local req = http_request.new_from_uri(BASE_URL .. "/upload")
    req.headers:upsert(":method", "POST")
    req.headers:upsert("content-type", "multipart/form-data; boundary=" .. boundary)
    req:set_body(body)

    local headers, stream = req:go(10)
    if not headers then return 0 end
    return tonumber(headers:get(":status"))
end

local function section(title)
    print("\n" .. string.rep("─", 56))
    print("  " .. title)
    print(string.rep("─", 56))
end

local function is_rejected(status)
    return status == 400 or status == 403 or status == 415 or status == 422 or status == 429
end

local function is_safe(status)
    return status ~= 500
end

-- ── 1. Rate Limiting ──────────────────────────────────────────────────────────
local function test_rate_limiting()
    section("Rate Limiting (burst=5, rate=1 req/s)")

    local successes = 0
    for i = 1, 5 do
        if quick_get("/files") == 200 then successes = successes + 1 end
    end
    record("Burst of 5 requests all succeed", successes == 5, successes .. "/5 returned 200")

    local s6 = quick_get("/files")
    record("6th immediate request rate-limited", s6 == 429, "status=" .. s6)

    print(string.format("  %s  Waiting 6s for recovery...", INFO))
    sleep(6)
    local recover = quick_get("/files")
    record("Requests recover after waiting", recover == 200, "status=" .. recover)
end

-- ── 2. Invalid File Types ─────────────────────────────────────────────────────
local function test_invalid_types()
    section("Invalid / Disallowed File Types")
    local disallowed = {
        { "exploit.php", "<?php system($_GET['cmd']); ?>", "application/x-php" },
        { "script.js",   "alert('xss')",                   "application/javascript" },
        { "shell.sh",    "#!/bin/bash\nrm -rf /",          "application/x-sh" }
    }
    for _, f in ipairs(disallowed) do
        sleep(0.3)
        local status = upload(f[1], f[2], f[3])
        record("Rejects " .. f[1], is_rejected(status), "status=" .. status)
    end
end

-- ── 3. File Size ──────────────────────────────────────────────────────────────
local function test_file_size()
    section("Oversized File Upload")

    -- 512 KB
    local small_content = string.rep("A", 512 * 1024)
    local s1 = upload("small.bin", small_content, "image/png")
    record("512 KB accepted", s1 == 200 or s1 == 201, "status=" .. s1)

    -- 32 MB (Note: Lua strings can handle this, but watch RAM)
    local big_content = string.rep("B", 32 * 1024 * 1024)
    local s2 = upload("huge.bin", big_content, "image/png")
    record("32 MB rejected", is_rejected(s2) or s2 == 413, "status=" .. s2)
end

-- ── 4. SQL Injection ──────────────────────────────────────────────────────────
local function test_sql_injection()
    section("SQL Injection")
    local payloads = {
        "' OR '1'='1",
        "'; DROP TABLE files; --",
        "admin'--"
    }
    for _, p in ipairs(payloads) do
        sleep(0.3)
        local status = upload(p .. ".png", "fake-png-data", "image/png")
        record("Upload filename injection safe: " .. p, is_safe(status), "status=" .. status)

        local encoded = http_util.encode_path(p)
        local s2 = quick_get("/files/test.png/" .. encoded)
        record("Path injection safe: " .. p, is_safe(s2), "status=" .. s2)
    end
end

-- ── 5. Path Traversal ─────────────────────────────────────────────────────────
local function test_path_traversal()
    section("Path Traversal")
    local traversals = { "../../../etc/passwd", "..%2F..%2Fetc%2Fpasswd" }
    for _, path in ipairs(traversals) do
        local status = quick_get("/files/" .. path .. "/meta")
        record("Path traversal blocked: " .. path, is_rejected(status) or status == 404, "status=" .. status)
    end
end

-- ── Main ──────────────────────────────────────────────────────────────────────
print(string.rep("═", 56))
print("  Go File Server — Lua Security Test Suite")
print("  Target: " .. BASE_URL)
print(string.rep("═", 56))

local initial_check = quick_get("/files")
if initial_check == 0 then
    print("\n  " .. FAIL .. " Cannot reach server. Is it running?")
    os.exit(1)
end

test_rate_limiting()
test_invalid_types()
test_file_size()
test_sql_injection()
test_path_traversal()

local ok = summary()
os.exit(ok and 0 or 1)
