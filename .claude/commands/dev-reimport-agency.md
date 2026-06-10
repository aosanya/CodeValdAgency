---
description: Rebuild and restart the codevaldagency Docker container, then reimport the utility-app-builder agency.json (updates event_flows and work plans without needing a DB reset).
---

# Reimport Agency

Rebuilds the `codevaldagency` image from local source, restarts the container, and runs the import against the running Cross instance.

The import is safe to run on a published agency — structural fields are frozen by the readonly guard, but `event_flows` and work plans are always refreshed.

Per FEAT-20260609-002, per-workflow `event_flows` live in `flows_<workflow.code>.json` siblings of `agency.json`. The importer does not touch the filesystem — this skill bundles each matching file inline into the corresponding workflow's `event_flows` field before POSTing.

---

## Step 1 — Rebuild and restart `codevaldagency`

```bash
cd /workspaces/CodeVald-AIProject/Deployment/local
docker compose build codevaldagency
docker compose up -d codevaldagency
```

Wait for the service to register (look for "registered with CodeValdCross"):

```bash
sleep 4 && docker logs codevald-local-codevaldagency-1 2>&1 | tail -6
```

If the log does not show "registered with CodeValdCross" after 4 seconds, wait another few seconds and retry the `docker logs` command.

---

## Step 2 — Locate the Cross container IP

`localhost:8081` is not reachable from within the dev container. Use the container IP instead:

```bash
CROSS_IP=$(docker inspect codevald-local-codevaldcross-1 --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
echo "Cross IP: $CROSS_IP"
```

---

## Step 3 — Bundle per-workflow `flows_<workflow.code>.json` into the request

The importer reads `event_flows` from the request body only. For each workflow whose `code` matches a `flows_<code>.json` sibling, inject the file contents into `workflows[].event_flows` of a temp copy of `agency.json`. Orphans (flow files with no matching workflow code) are logged so naming mismatches surface immediately.

```bash
AGENCY_DIR="/workspaces/CodeVald-AIProject/CodeValdImplementations/Agencies/utility-app-builder"
AGENCY_FILE="$AGENCY_DIR/agency.json"
BUNDLED=$(mktemp --suffix=.json)
cp "$AGENCY_FILE" "$BUNDLED"

bundled_count=0
for code in $(jq -r '.workflows[].code' "$BUNDLED"); do
  flow_file="$AGENCY_DIR/flows_${code}.json"
  if [[ -f "$flow_file" ]]; then
    jq --slurpfile flow "$flow_file" --arg code "$code" \
      '(.workflows[] | select(.code == $code)).event_flows = $flow[0]' \
      "$BUNDLED" > "$BUNDLED.tmp" && mv "$BUNDLED.tmp" "$BUNDLED"
    echo "bundled: flows_${code}.json -> workflows[code=$code].event_flows"
    bundled_count=$((bundled_count + 1))
  else
    echo "skip:    no flow file for workflow code '$code' (looked for flows_${code}.json)"
  fi
done

# Surface orphan flow files (named flows_<x>.json but no workflow with code x).
mapfile -t workflow_codes < <(jq -r '.workflows[].code' "$BUNDLED")
for f in "$AGENCY_DIR"/flows_*.json; do
  [[ -e "$f" ]] || continue
  base=$(basename "$f" .json); flow_code="${base#flows_}"
  matched=0
  for code in "${workflow_codes[@]}"; do
    [[ "$code" == "$flow_code" ]] && matched=1 && break
  done
  [[ $matched -eq 0 ]] && echo "orphan:  $f (no workflow with code '$flow_code' — rename file or add the workflow)"
done

echo "Bundled $bundled_count flow file(s) into $BUNDLED"
```

If `bundled_count` is 0, the live agency will receive no flows — fix the naming mismatch before proceeding.

---

## Step 4 — Run the import

```bash
export CV_AUTH="codevald:chanuisosnnau@geme"
AGENCY_ID=$(jq -r '.agency.code' "$BUNDLED")

RESP=$(curl -sw "\n%{http_code}" -X POST "http://$CROSS_IP:8081/agency/$AGENCY_ID/import" \
  -u "$CV_AUTH" \
  -H "Content-Type: application/json" \
  --data-binary "@$BUNDLED")

HTTP_CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | sed '$d')
echo "HTTP $HTTP_CODE"
echo "$BODY" | jq . 2>/dev/null || echo "$BODY"
```

**Expected:** HTTP 200. The response body for a published agency may be `{}` — that is normal; the import still refreshes `event_flows` and work plans via the readonly bypass.

---

## Step 5 — Verify per-workflow `event_flows` landed

```bash
curl -s "http://$CROSS_IP:8081/agency/$AGENCY_ID/workflows" \
  -u "$CV_AUTH" \
  | jq '.workflows[] | {
      name,
      flows: ((.eventFlows // "") | if . == "" then 0 else (fromjson.flows // [] | length) end),
      steps: ((.eventFlows // "") | if . == "" then 0 else ([fromjson.flows[]?.steps[]?] | length) end)
    }'
```

**Assertions:**
1. Each workflow that had a matching `flows_<code>.json` in Step 3 reports `flows > 0` and `steps > 0`.
2. Workflows without a matching file legitimately report `0` — that is expected, not a regression.

If a bundled workflow reports `0`, the import did not write the workflow's `event_flows` prop — check `docker logs codevald-local-codevaldagency-1` for `[ImportDraft]` lines mentioning that workflow code.
