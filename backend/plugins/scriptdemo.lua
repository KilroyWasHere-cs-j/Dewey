-- Demonstrates the "script" plugin bucket, which has no existing example.
-- Unlike "filter" plugins (which may only rewrite entry.Path), the backend
-- reads Act and Meta back from "script" plugins (see callBeginWithReturn in
-- plugins.go) — this plugin exists to show that half of the contract.
Type = "script"
-- Defines the salience of the plugin
Salience = 1

function WhoAmI()
    return Type, Salience
end

-- Receives a table representing DBEntry, modifies and prints it
function Begin(entry)
    print("=== DBEntry plugin script demo ===")
    print("Filename : " .. tostring(entry.Filename))
    print("Act      : " .. tostring(entry.Act))
    print("Hash     : " .. tostring(entry.Hash))
    print("Path     : " .. tostring(entry.Path))
    print("Meta     : " .. tostring(entry.Meta))
    print("Barcode  : " .. tostring(entry.Barcode))
    print("===============")

    -- Tag Meta based on whether barcode scanning found anything, so it's
    -- easy to see this ran and to see what data it had available.
    if entry.Barcode ~= nil and entry.Barcode ~= "Nil" and entry.Barcode ~= "" then
        entry.Meta = "barcode:" .. entry.Barcode
    else
        entry.Meta = "no-barcode"
    end

    -- Script plugins may also rewrite Act (the ACTs ID) — fall back to the
    -- file hash so every entry still gets a non-empty Act even if the
    -- uploader left the acts_id field blank.
    if entry.Act == nil or entry.Act == "" then
        entry.Act = entry.Hash
    end

    return entry
end
