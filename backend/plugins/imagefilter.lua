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
    print("Barcode  : " .. tostring(entry.Barcode))
    print("===============")

    if entry.Path:match("%.jpg$") or entry.Path:match("%.png$") then
        entry.Path = "./image/" .. entry.Path
    end
    return entry
end
