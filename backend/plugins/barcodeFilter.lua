-- Demonstrates a filter branching on entry.Barcode rather than
-- filename/extension. Routes files with a successfully decoded barcode
-- into ./scanned/, leaving everything else (no barcode, or not a
-- barcode-candidate extension per issue #161) wherever earlier filters
-- put it.
Type = "filter"
Salience = 1

function WhoAmI()
    return Type, Salience
end

function Begin(entry)
    print("=== DBEntry plugin barcode filter ===")
    print("Filename : " .. tostring(entry.Filename))
    print("Barcode  : " .. tostring(entry.Barcode))
    print("===============")

    if entry.Barcode ~= nil and entry.Barcode ~= "Nil" and entry.Barcode ~= "" then
        entry.Path = "./scanned/" .. entry.Path
    end
    return entry
end
