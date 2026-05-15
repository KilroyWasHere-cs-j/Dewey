-- Defines type of plugin
Type = "filter"
-- Defines the salience of the plugin
Salience = 1

function WhoAmI()
    return Type, Salience
end

-- Receives a table representing DBEntry, modifies and prints it
function Begin(entry)
    -- Print all fields
    print("=== DBEntry plugin ===")
    print("Filename : " .. tostring(entry.Filename))
    print("Act      : " .. tostring(entry.Act))
    print("Hash     : " .. tostring(entry.Hash))
    print("Path     : " .. tostring(entry.Path))
    print("Meta     : " .. tostring(entry.Meta))
    print("===============")

    entry.Meta = "Gabriel was here"
    return entry
end
