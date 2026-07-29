-- Salience determines this plugin's run order relative to other OnDelete
-- plugins (higher runs first). WhoAmI reports salience only — which hook
-- this plugin attaches to comes from the OnDelete function name below.
Salience = 1

function WhoAmI()
    return Salience
end

-- OnDelete runs synchronously before a file is actually removed. Calling
-- error(...) here aborts the deletion — the caller gets a 403 with this
-- message instead of the file being deleted. Returning the table normally
-- allows the deletion to proceed.
function OnDelete(entry)
    if entry.Filename:match("^protected_") then
        error("refusing to delete a protected_ file: " .. entry.Filename)
    end

    return entry
end
