-- Salience determines this plugin's run order relative to other OnFilter
-- plugins (higher runs first). WhoAmI reports salience only — which hook
-- this plugin attaches to comes from the OnFilter function name below.
Salience = 1

function WhoAmI()
    return Salience
end

-- OnFilter receives a table representing DBEntry and may rewrite its Path
-- to route the file into a subfolder. Each check runs independently
-- (not elseif) so a file matching more than one rule gets re-prefixed by
-- each in turn, same as chaining separate filter plugins would have.
function OnFilter(entry)
    if entry.Path:match("%.jpg$") or entry.Path:match("%.png$") then
        entry.Path = "./image/" .. entry.Path
    end

    if entry.Path:match("%.txt$") or entry.Path:match("%.md$") then
        entry.Path = "./text/" .. entry.Path
    end

    if entry.Path:match("lenna") then
        entry.Path = "./Swedish/" .. entry.Path
    end

    if entry.Path:match("test") then
        entry.Path = "./testingFiles/" .. entry.Path
    end

    return entry
end
