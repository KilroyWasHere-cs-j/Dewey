-- Defines type of plugin
Type = "init"

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
