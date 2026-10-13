"""Flat Claude token/cost usage between local datetimes (default UTC+6).

Table:  python -I .claude/skills/claude-usage-report/usage_range.py "2026-10-12 09:00" "2026-10-13 18:00" [--tz 6]
Report: python -I .claude/skills/claude-usage-report/usage_range.py --report "FROM1" "TO1" ["FROM2" "TO2" ...] [--tz 6]
        (markdown summary, one "Week N" section per window + per-model cost)

Counts each Claude API message once (dedup by claude_message_id), no parent/child
rollup, claude agent only. Cost is approximate: exact-model match in model_pricing,
1h cache writes at the 1h rate, tiered bands ignored (none for current Claude models).
"""
import sqlite3, os, sys, json
from datetime import datetime, timedelta, timezone

sys.stdout.reconfigure(encoding="utf-8")
argv = sys.argv[1:]
tz = 6.0
if "--tz" in argv:
    k = argv.index("--tz"); tz = float(argv[k + 1]); del argv[k:k + 2]
report = "--report" in argv
args = [a for a in argv if a != "--report"]
if len(args) < 2 or len(args) % 2:
    sys.exit(__doc__)
local = timezone(timedelta(hours=tz))
fmt = lambda s: datetime.strptime(s, "%Y-%m-%d %H:%M")
utc = lambda s: fmt(s).replace(tzinfo=local).astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.000Z")

c = sqlite3.connect("file:" + os.path.expanduser("~/.agentsview/sessions.db") + "?mode=ro", uri=True)
price = {r[0]: r[1:] for r in c.execute(
    "select model_pattern,input_microdollars_per_mtok,output_microdollars_per_mtok,"
    "cache_creation_microdollars_per_mtok,cache_read_microdollars_per_mtok,"
    "cache_creation_1h_microdollars_per_mtok from model_pricing")}

def window(a, b):
    lo, hi = utc(a), utc(b)
    rows = c.execute("""select m.model, m.token_usage, m.claude_message_id, m.id from messages m
        join sessions s on s.id=m.session_id
        where s.agent='claude' and m.token_usage!='' and m.timestamp>=? and m.timestamp<?
        order by m.id""", (lo, hi)).fetchall()
    seen = {}
    for model, tu, mid, i in rows:
        seen[mid or f"id{i}"] = (model, tu)  # last write wins
    tot = {}
    for model, tu in seen.values():
        try: u = json.loads(tu)
        except Exception: continue
        t = tot.setdefault(model, [0] * 6)  # in, out, cw, cr, msgs, cw_1h
        t[0] += u.get("input_tokens", 0); t[1] += u.get("output_tokens", 0)
        t[2] += u.get("cache_creation_input_tokens", 0); t[3] += u.get("cache_read_input_tokens", 0)
        t[4] += 1; t[5] += (u.get("cache_creation") or {}).get("ephemeral_1h_input_tokens", 0)
    out = {}
    for model, (i, o, cw, cr, n, w1) in tot.items():
        p = price.get(model)
        cost = (i * p[0] + o * p[1] + (cw - w1) * p[2] + w1 * (p[4] or p[2]) + cr * p[3]) / 1e12 if p else None
        out[model] = dict(i=i, o=o, cw=cw, cr=cr, n=n, cost=cost)
    return out

def pretty(model):  # claude-sonnet-5-5 -> Sonnet 5.5, claude-haiku-4-5-20251001 -> Haiku 4.5
    parts = model.removeprefix("claude-").split("-")
    parts = [x for x in parts if not (x.isdigit() and len(x) == 8)]
    fam = parts[0].capitalize()
    return fam + " " + ".".join(parts[1:]) if parts[1:] else fam

def sums(w):
    real = {m: v for m, v in w.items() if v["i"] + v["o"] + v["cw"] + v["cr"] > 0}  # drop <synthetic>
    return real, {k: sum(v[k] or 0 for v in real.values()) for k in ("i", "o", "cw", "cr", "n", "cost")}

pairs = list(zip(args[0::2], args[1::2]))
if report:
    fm = lambda s: fmt(s).strftime("%b %-d %H:%M") if os.name != "nt" else fmt(s).strftime("%b %d %H:%M").replace(" 0", " ")
    print(f"**Claude API Usage Summary (UTC{tz:+g}, {fmt(pairs[0][0]).year})**\n")
    brk = []
    for n, (a, b) in enumerate(pairs, 1):
        real, t = sums(window(a, b))
        print(f"**Week {n}: {fm(a)} → {fm(b)}**")
        print(f"• **Messages:** {t['n']:,}\n• **Input Tokens:** {t['i']:,}\n• **Output Tokens:** {t['o']:,}")
        print(f"• **Cache Write:** {t['cw']:,}\n• **Cache Read:** {t['cr']:,}\n• **Est. Cost:** ${t['cost']:,.2f}\n")
        brk.append((a, b, real))
    print("**Cost Breakdown by Model**")
    for a, b, real in brk:
        items = sorted(real.items(), key=lambda kv: -(kv[1]["cost"] or 0))
        print(f"• **{fm(a).rsplit(' ', 1)[0]} → {fm(b).rsplit(' ', 1)[0]}:** " +
              " | ".join(f"{pretty(m)} (${v['cost'] or 0:,.2f})" for m, v in items))
    print("\n*Note: Costs are estimates based on the pricing table in the local database, with 1-hour cache "
          "writes billed at the 1-hour rate. No tiered pricing bands apply to these models. Synthetic internal "
          "placeholders with zero tokens are excluded from the totals.*")
else:
    for a, b in pairs:
        w = window(a, b)
        print(f"Window: {a} -> {b} (UTC{tz:+g})")
        print(f"{'model':28} {'msgs':>6} {'input':>10} {'output':>10} {'cache_wr':>11} {'cache_rd':>13} {'~cost$':>9}")
        for m, v in sorted(w.items()):
            print(f"{m[:28]:28} {v['n']:6} {v['i']:10} {v['o']:10} {v['cw']:11} {v['cr']:13} {(v['cost'] if v['cost'] is not None else float('nan')):9.2f}")
        _, t = sums(w)
        print(f"{'TOTAL':28} {t['n']:6} {t['i']:10} {t['o']:10} {t['cw']:11} {t['cr']:13} {t['cost']:9.2f}\n")
