#!/usr/bin/env python3
"""Measure certificate boost variants on the official split."""

from __future__ import annotations

import collections, json, math, os, re
from collections import Counter, defaultdict
import psycopg2

DSN=os.environ.get('DATABASE_URL','postgres://lk:lk@localhost:55432/lk?sslmode=disable')
TRAIN_END='2026-08-31'; TEST_START='2026-09-07'
REGULATED={'Медицинские организации','Образование и наука','Органы власти','Финансы (банки, страхование)'}

def tokens(name):
    return set(str(name or '').lower().replace('ё','е').replace(',',' ').replace('.',' ').replace('-',' ').replace('(',' ').replace(')',' ').split())

def main():
    conn=psycopg2.connect(DSN); cur=conn.cursor()
    cur.execute('select item_id,title,available,category from lk_items')
    items={r[0]:{'title':r[1],'available':r[2],'category':r[3],'tokens':tokens(r[1])} for r in cur.fetchall()}
    cur.execute('select client_id,client_type,coalesce(industry,\'\') from lk_clients')
    clients={r[0]:{'audience':'b2c' if (r[1] or '').upper()=='ФЛ' else 'b2b','industry':r[2]} for r in cur.fetchall()}
    cur.execute('select client_id,order_id,item_id,order_week::text from lk_orders')
    orders=cur.fetchall()
    pre=defaultdict(set); test=defaultdict(set); baskets=defaultdict(set)
    for cid,oid,iid,w in orders:
        if w<=TRAIN_END: pre[cid].add(iid); baskets[(oid,w)].add(iid)
        elif w>=TEST_START: test[cid].add(iid)
    targets={c:test[c]-pre.get(c,set()) for c in test}
    group=[c for c in pre if targets.get(c)]
    pop_sets=defaultdict(set); item_pop=Counter()
    for cid,oid,iid,w in orders:
        if w>TRAIN_END: continue
        if clients.get(cid,{}).get('audience')=='b2b': pop_sets[iid].add(cid)
    pop={i:len(s) for i,s in pop_sets.items()}; maxp=max(pop.values()) if pop else 1
    poplist=[i for i,_ in sorted(pop.items(),key=lambda x:-x[1])]
    for b in baskets.values():
        for i in b: item_pop[i]+=1
    pairs=Counter()
    for b in baskets.values():
        for a in b:
            for c in b:
                if a!=c: pairs[(a,c)]+=1
    nb=defaultdict(list)
    for (a,b),c in pairs.items(): nb[a].append((b,c))
    for a in nb: nb[a].sort(key=lambda x:(-x[1],x[0]))
    cur.execute("""select m.item_id, bool_or(c.valid_until>=current_date) from fstec_certificate_items m join fstec_certificates c on c.id=m.certificate_id group by m.item_id""")
    cert={r[0]:bool(r[1]) for r in cur.fetchall()}

    def recommend(cid,weight):
        c=clients.get(cid,{}); owned=pre[cid]
        if c.get('audience')=='b2c':
            return [i for i in poplist if i not in owned][:3]
        scores=defaultdict(float)
        for a in owned:
            for b,count in nb.get(a,[])[:200]:
                if b in owned or not items.get(b,{}).get('available'): continue
                scores[b]+=3*count/math.sqrt(max(1,item_pop[a])*max(1,item_pop[b]))
        for b,count in pop.items():
            if b not in owned and items.get(b,{}).get('available'): scores[b]+=0.5*count/maxp
        for a in owned:
            at=items.get(a,{}).get('tokens',set())
            for b in poplist[:500]:
                if b in owned or not items.get(b,{}).get('available'): continue
                ov=len(at & items.get(b,{}).get('tokens',set()))
                if ov: scores[b]+=0.2*ov
        if weight!=1.0:
            for b in scores:
                if cert.get(b): scores[b]*=weight
        return [i for i,_ in sorted(scores.items(),key=lambda kv:(-kv[1],kv[0]))][:3]

    sec_clients={c for c in group if any(items.get(i,{}).get('category')=='security' for i in targets[c])}
    reg_clients={c for c in group if clients.get(c,{}).get('industry') in REGULATED}
    security_owned={c for c in group if any(items.get(i,{}).get('category')=='security' for i in pre[c])}
    def ev(ids,weight):
        hits=0;p3=0;sec_hits=0;total_hits=0
        for cid in ids:
            r=recommend(cid,weight); h=[i for i in r if i in targets[cid]]
            hits+=1 if h else 0; p3+=len(h)/3; total_hits+=len(h)
            sec_hits+=sum(1 for i in h if items.get(i,{}).get('category')=='security')
        n=len(ids); return {'n':n,'p3':round(p3/n,4) if n else 0,'hit':round(hits/n,4) if n else 0,'sec_share':round(sec_hits/total_hits,4) if total_hits else 0}
    out={}
    for w in [1.0,1.02,1.05,1.1,1.2]:
        out[str(w)]={'all':ev(group,w),'security_targets':ev(sec_clients,w),'regulated':ev(reg_clients,w),'security_owned':ev(security_owned,w)}
    print(json.dumps(out,ensure_ascii=False,indent=2))
    open(os.path.join(os.path.dirname(__file__),'cert_signal_evaluation.json'),'w',encoding='utf-8').write(json.dumps(out,ensure_ascii=False,indent=2))
if __name__=='__main__': main()
