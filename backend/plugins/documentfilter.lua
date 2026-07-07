-- Routes office/PDF uploads into ./documents/ — the upload allowlist
-- (routes.go) accepts pdf/doc/docx/xls/xlsx/csv/ppt alongside the image and
-- text extensions imagefilter.lua/textfilter.lua already handle, but none of
-- the existing filters covered this half of the allowlist.
Type = "filter"
Salience = 1

function WhoAmI()
    return Type, Salience
end

function Begin(entry)
    print("=== DBEntry plugin document filter ===")
    print("Filename : " .. tostring(entry.Filename))
    print("Path     : " .. tostring(entry.Path))
    print("===============")

    if entry.Path:match("%.pdf$")
        or entry.Path:match("%.docx?$")
        or entry.Path:match("%.xlsx?$")
        or entry.Path:match("%.csv$")
        or entry.Path:match("%.ppt$") then
        entry.Path = "./documents/" .. entry.Path
    end
    return entry
end
