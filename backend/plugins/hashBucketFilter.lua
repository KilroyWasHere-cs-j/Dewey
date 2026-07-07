-- Demonstrates a filter branching on entry.Hash rather than filename/path.
-- Fans uploads out across subdirectories keyed by the first two hex
-- characters of the file's SHA256 hash (same idea as git's object store),
-- so a single flat directory doesn't end up holding an unbounded number of
-- files as upload volume grows.
Type = "filter"
Salience = 1

function WhoAmI()
    return Type, Salience
end

function Begin(entry)
    print("=== DBEntry plugin hash bucket filter ===")
    print("Filename : " .. tostring(entry.Filename))
    print("Hash     : " .. tostring(entry.Hash))
    print("===============")

    if entry.Hash ~= nil and #entry.Hash >= 2 then
        local bucket = entry.Hash:sub(1, 2)
        entry.Path = "./bucket/" .. bucket .. "/" .. entry.Path
    end
    return entry
end
