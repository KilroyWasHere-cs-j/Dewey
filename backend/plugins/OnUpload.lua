-- Salience determines this plugin's run order relative to other OnUpload
-- plugins (higher runs first). WhoAmI reports salience only — which hook
-- this plugin attaches to comes from the OnUpload function name below.
Salience = 1

-- OnUpload fires once per upload, before OnFilter, as a notification hook —
-- it runs async (after the 200 OK is already sent), so it can't veto the
-- upload the way OnDelete can veto a deletion. It can still tag the entry:
-- this example writes into Meta, something a bucket-typed "filter" plugin
-- was never allowed to do under the old design.
function WhoAmI()
    return Salience
end

function OnUpload(entry)
    if entry.Barcode ~= "" and entry.Barcode ~= "Nil" then
        entry.Meta = "barcode:" .. entry.Barcode
    end

    return entry
end
