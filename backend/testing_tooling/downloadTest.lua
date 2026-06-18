local Pegasus = require('pegasus')
-- We use a simple pattern matching helper or third-party router for params
local server = Pegasus:new({
    port = 8080,
    location = './public'
})

-- Mock controller functions to match your Go handlers
local function getFile(req, rep, filename, meta)
    rep:addHeader("Content-Type", "application/json")
    rep:write(string.format('{"file": "%s", "metadata_only": %s}', filename, tostring(meta)))
end

local function listFiles(req, rep)
    rep:addHeader("Content-Type", "application/json")
    rep:write('{"files": ["two.png", "lenna.jpg", "test.pdf"]}')
end

-- Start the server and handle incoming paths
server:start(function(req, rep)
    local path = req:path()

    -- Match GET /files/:filename/:meta
    -- Lua pattern captures everything between slashes
    local filename, meta = path:match("^/files/([^/]+)/([^/]+)$")

    if filename and meta and req:method() == "GET" then
        return getFile(req, rep, filename, meta)

        -- Match GET /files
    elseif path == "/files" and req:method() == "GET" then
        return listFiles(req, rep)
    else
        rep:status(404)
        rep:write("Not Found")
    end
end)
