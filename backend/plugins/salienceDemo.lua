-- Demonstrates salience ordering within the "filter" bucket. Every other
-- filter plugin (imagefilter/namefilter/textfilter) uses Salience = 1, so
-- they run in whatever order LoadPlugins happened to read the directory.
-- This one uses a much higher salience so it always runs first, and proves
-- it by prefixing Path with a staging folder — the later filters only match
-- on file extension (e.g. "%.txt$"), so this prefix doesn't interfere with
-- their own renames, it just shows up as an extra leading path segment.
Type = "filter"
Salience = 100

function WhoAmI()
    return Type, Salience
end

function Begin(entry)
    print("=== DBEntry plugin salience demo (runs first) ===")
    print("Filename : " .. tostring(entry.Filename))
    print("Path     : " .. tostring(entry.Path))
    print("===============")

    entry.Path = "./staged/" .. entry.Path
    return entry
end
