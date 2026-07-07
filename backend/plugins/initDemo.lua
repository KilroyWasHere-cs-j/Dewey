-- Demonstrates the "init" plugin bucket: one-time setup work that should
-- happen once when the server starts, before any files are processed.
-- Begin() takes no arguments here (unlike filter/script plugins) since
-- there's no DBEntry yet at startup — see Apluginsinit.lua for the minimal
-- version this expands on.
Type = "init"
Salience = 1

function WhoAmI()
    return Type, Salience
end

-- Runs once when the plugin is invoked
function Begin()
    print("=== initDemo: plugin system starting up ===")
    print("os.date        : " .. tostring(os.date()))
    print("os.clock       : " .. tostring(os.clock()))
    print("============================================")
end

-- Hook for backend to call on system shutdown
function End()
    -- This code will be run before server end
    -- If this is a filter plugin don't add anything here, it won't be called
end
