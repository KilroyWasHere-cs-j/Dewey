#!/usr/bin/env lua

-- Seed the random number generator
math.randomseed(os.time())

-- ── ANSI color codes ──────────────────────────────────────────────────────────
local reset          = "\27[0m"
local bold           = "\27[1m"
local dim            = "\27[2m"
local red            = "\27[31m"
local green          = "\27[32m"
local yellow         = "\27[33m"
local magenta        = "\27[35m"
local cyan           = "\27[36m"
local white          = "\27[37m"

-- ── Random data pools ─────────────────────────────────────────────────────────
local firstNames     = { "Oliver", "Phoebe", "Marcus", "Ingrid", "Tariq", "Yuki", "Soren", "Amara", "Declan", "Priya" }
local lastNames      = { "Nakamura", "Osei", "Lindqvist", "Ferrara", "Patel", "Kowalski", "Okafor", "Reyes", "Svensson",
    "Mbeki" }
local employers      = { "Horizon Robotics", "Starfall Media", "Ironclad Materials", "Vivant Health",
    "Obsidian Logistics", "Luminary Tech", "Verdant Farms", "Nexus Analytics", "Solaris Energy", "Phalanx Security" }
local adjusters      = { "T. Hargrove", "M. Delacroix", "A. Fujimoto", "R. Oduya", "S. Bergmann", "C. Abramowitz",
    "D. Kazakov", "F. Osei-Mensah", "L. Cartwright", "P. Iyer" }
local claimTypes     = { "Workers Comp", "Liability", "Property", "Medical", "Disability", "Auto", "Product Liability",
    "Environmental" }
local supportLvls    = { "Full Support", "Partial", "Minimal", "Psychiatric", "Physical Therapy", "None",
    "Pending Review" }
local jurisdicts     = { "California", "New York", "Texas", "Florida", "Illinois", "Washington", "Colorado", "Georgia",
    "Ohio", "Michigan" }

local filePairs      = {
    { path = "two.png",         label = "two.png" },
    { path = "lenna.jpg",       label = "lenna.jpg" },
    { path = "test.pdf",        label = "test.pdf" },
    { path = "CrossDocTesting", label = "CrossDocTesting" },
    { path = "renamedELF.txt",  label = "renamedELF.txt" },
}

local statusMessages = {
    "Dispatching request…",
    "Knocking on the server door…",
    "Firing off multipart form…",
    "Launching claim into the void…",
    "Sending bytes across the wire…",
}

-- ── Random Utility Functions ──────────────────────────────────────────────────
local function rnd(pool)
    return pool[math.random(#pool)]
end

local function randDate()
    -- 2024-01-01 basic epoch calculation
    local start_ts = os.time({ year = 2024, month = 1, day = 1, hour = 0, min = 0, sec = 0 })
    local days_seconds = math.random(0, 729) * 24 * 60 * 60
    return os.date("%Y-%m-%d", start_ts + days_seconds)
end

local function randClaimNum() return string.format("CLM-%05d", math.random(10000, 99999)) end
local function randPolicyNum() return string.format("POL-%06d", math.random(100000, 999999)) end
local function randActsID(i) return string.format("ACTS_%03d", i) end

local function getBasename(path)
    return path:match("^.+/(.+)$") or path
end

-- ── Pretty printing helpers ───────────────────────────────────────────────────
local function header()
    local w = 60
    print(string.format("\n%s%s╔%s╗%s", bold, cyan, string.rep("═", w), reset))
    print(string.format("%s%s║%s  🧪  CLAIM & FILE VERIFICATION TEST SUITE%s%s%s║%s", bold, cyan, white,
        string.rep(" ", w - 43), cyan, bold, reset))
    print(string.format("%s%s╚%s╝%s\n", bold, cyan, string.rep("═", w), reset))
end

local function sectionBanner(n, total, label)
    local bar = string.format("[%d/%d]", n, total)
    print(string.format("%s%s  %s %-45s%s", bold, yellow, bar, label, reset))
    print(string.format("%s%s%s", dim, string.rep("·", 60), reset))
end

local function fieldLine(key, value)
    print(string.format("  %s%-20s%s %s%s%s", dim, key, reset, cyan, value, reset))
end

local function statusLine(msg)
    print(string.format("\n  %s⟳  %s%s", magenta, msg, reset))
end

local function successLine(code, took_ms)
    local color = green
    local icon = "✓"
    if code >= 400 then
        color = red
        icon = "✗"
    elseif code >= 300 then
        color = yellow
        icon = "↪"
    end
    print(string.format("  %s%s%s  HTTP %s%d%s  %s(%dms)%s\n",
        bold, color, icon, bold, code, reset, dim, math.floor(took_ms + 0.5), reset))
end

local function errorLine(err)
    print(string.format("  %s✗  ERROR: %s%s\n", red, tostring(err), reset))
end

local function separator()
    print(string.format("%s%s%s", dim, string.rep("─", 60), reset))
end

local function summary(passed, failed, total_ms)
    local w = 60
    print(string.format("\n%s%s╔%s╗%s", bold, cyan, string.rep("═", w), reset))
    print(string.format("%s%s║%s  RESULTS%s%s%s║%s", bold, cyan, white, string.rep(" ", w - 9), cyan, bold, reset))
    print(string.format("%s%s╠%s╣%s", bold, cyan, string.rep("═", w), reset))

    local p_str = string.format("  ✓  Passed : %d", passed)
    print(string.format("%s%s║%s%s%s%s%s║%s", bold, cyan, green, p_str, string.rep(" ", w - #p_str), cyan, bold, reset))

    local f_str = string.format("  ✗  Failed : %d", failed)
    print(string.format("%s%s║%s%s%s%s%s║%s", bold, cyan, red, f_str, string.rep(" ", w - #f_str), cyan, bold, reset))

    local t_str = string.format("  ⏱  Total  : %dms", math.floor(total_ms + 0.5))
    print(string.format("%s%s║%s%s%s%s%s║%s", bold, cyan, dim, t_str, string.rep(" ", w - #t_str), cyan, bold, reset))

    print(string.format("%s%s╚%s╝%s\n", bold, cyan, string.rep("═", w), reset))
end

-- ── HTTP Helper Functions ─────────────────────────────────────────────────────
-- Attempt to load luasocket/luasec safely
local http = require("socket.http")
local ltn12 = require("ltn12")

local function doGET(url)
    local response_body = {}
    local res, code, _ = http.request {
        url = url,
        method = "GET",
        sink = ltn12.sink.table(response_body),
        timeout = 10
    }
    if not res then return 0, code end
    return code, nil
end

local function doUpload(url, fields, filePath)
    local boundary = "----LuaMultipartBoundary" .. string.format("%04x", math.random(0, 0xffff))
    local body_chunks = {}

    -- Add regular form fields
    for k, v in pairs(fields) do
        table.insert(body_chunks,
            string.format("--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n%s\r\n", boundary, k, v))
    end

    -- Add file payload if available
    if filePath then
        local f = io.open(filePath, "rb")
        if f then
            local content = f:read("*all")
            f:close()
            local filename = getBasename(filePath)
            table.insert(body_chunks,
                string.format(
                "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\nContent-Type: application/octet-stream\r\n\r\n%s\r\n",
                    boundary, filename, content))
        end
    end

    table.insert(body_chunks, string.format("--%s--\r\n", boundary))
    local body = table.concat(body_chunks)

    local response_body = {}
    local res, code, _ = http.request {
        url = url,
        method = "POST",
        headers = {
            ["Content-Type"] = "multipart/form-data; boundary=" .. boundary,
            ["Content-Length"] = tostring(#body)
        },
        source = ltn12.source.string(body),
        sink = ltn12.sink.table(response_body),
        timeout = 10
    }

    if not res then return 0, code end
    return code, nil
end

-- ── Main Execution ────────────────────────────────────────────────────────────
local function main()
    local base = "http://localhost:8080"
    if arg[1] then
        base = arg[1]:gsub("/+$", "") -- Trim trailing slashes
    end

    header()
    print(string.format("  %sTarget:%s %s%s%s\n", dim, reset, bold, base, reset))

    local cases = {}

    -- Step 1: Baseline Health Check
    table.insert(cases, {
        label = "GET / (health check)",
        fields = { url = base .. "/" },
        run = function() return doGET(base .. "/") end
    })

    -- Step 2: Build validation chains for files dynamically
    for i, fp in ipairs(filePairs) do
        local fname = rnd(firstNames) .. " " .. rnd(lastNames)
        local filenameOnly = getBasename(fp.path)

        local fields = {
            claim_number   = randClaimNum(),
            claimant_name  = fname,
            date_of_injury = randDate(),
            employer       = rnd(employers),
            adjuster       = rnd(adjusters),
            support        = rnd(supportLvls),
            claim_type     = rnd(claimTypes),
            jurisdiction   = rnd(jurisdicts),
            policy_number  = randPolicyNum(),
            acts_id        = randActsID(i),
            data           = string.format("run-%d-data-%04x", i, math.random(0, 0xffff)),
        }

        -- A: Document Submission Test
        table.insert(cases, {
            label = string.format("POST /upload [%s]", fp.label),
            fields = fields,
            run = function() return doUpload(base .. "/upload", fields, fp.path) end
        })

        -- B: Metadata Verification
        table.insert(cases, {
            label = string.format("GET /files/%s/true (Verify Meta)", filenameOnly),
            fields = { file = filenameOnly, meta_flag = "true" },
            run = function() return doGET(string.format("%s/files/%s/true", base, filenameOnly)) end
        })

        -- C: Payload Download Verification
        table.insert(cases, {
            label = string.format("GET /files/%s/false (Verify File)", filenameOnly),
            fields = { file = filenameOnly, meta_flag = "false" },
            run = function() return doGET(string.format("%s/files/%s/false", base, filenameOnly)) end
        })
    end

    -- Step 3: Global Catalog Verification
    table.insert(cases, {
        label = "GET /files (Verify Index Inventory)",
        fields = { endpoint = "/files" },
        run = function() return doGET(base .. "/files") end
    })

    -- Execution Loop
    local total = #cases
    local passed, failed = 0, 0
    -- Simple clock metric approximation in seconds
    local startTime = os.clock()

    for i, tc in ipairs(cases) do
        sectionBanner(i, total, tc.label)

        for k, v in pairs(tc.fields) do
            fieldLine(k .. ":", v)
        end

        statusLine(rnd(statusMessages))

        local t0 = os.clock()
        local code, err = tc.run()
        local elapsed_ms = (os.clock() - t0) * 1000

        if code == 0 or err then
            errorLine(err or "Connection Failed")
            failed = failed + 1
        else
            successLine(code, elapsed_ms)
            if code < 400 then
                passed = passed + 1
            else
                failed = failed + 1
            end
        end

        separator()

        -- Sleep replacement using a busy wait or socket select
        local socket = require("socket")
        socket.select(nil, nil, 0.250)
    end

    local total_duration_ms = (os.clock() - startTime) * 1000
    summary(passed, failed, total_duration_ms)
end

main()
