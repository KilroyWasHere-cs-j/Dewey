-- Demonstrates the "tick" plugin bucket: periodic work that should happen
-- once per daemon cycle (alongside cache cleanup and the backup pass —
-- see startDaemon in utils.go). Begin() takes no arguments, same as init
-- plugins, since a tick isn't tied to any single file. See tickMacro.lua
-- for the minimal version this expands on.
Type = "tick"
Salience = 1

function WhoAmI()
    return Type, Salience
end

function Begin()
    print("=== tickDemo: daemon tick fired ===")
    print("os.date  : " .. tostring(os.date()))
    print("====================================")
end

-- Hook for backend to call on shutdown
function End()
    -- This code will be run before server end
    -- If this is a filter plugin don't add anything here, it won't be called
end
