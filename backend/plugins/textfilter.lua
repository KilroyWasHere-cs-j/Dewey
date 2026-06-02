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
    print("=== DBEntry plugin image plugin ===")
    print("Filename : " .. tostring(entry.Filename))
    print("Act      : " .. tostring(entry.Act))
    print("Hash     : " .. tostring(entry.Hash))
    print("Path     : " .. tostring(entry.Path))
    print("Meta     : " .. tostring(entry.Meta))
    print("===============")

    if entry.Filename:match("%.txt$") or entry.Filename:match("%.md$") then
        entry.Filename = "./text/" .. entry.Filename
    end
    return entry
end
