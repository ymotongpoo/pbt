"""第12章の表の測定用。シードを変えてテストを実行し、発見数と縮小された列を数える。

使い方:
    go test -c -o /tmp/ch12.test ./ch12/
    PBT_MEASURE=1 python3 ch12/measure_seeds.py /tmp/ch12.test 40 TestAuditSwapMutant TestMeasureDeeper2
"""
import subprocess,re,sys,statistics,collections
binp=sys.argv[1]
def run(test,seed):
    r=subprocess.run([binp,'-test.count=1','-test.run','^'+test+'$','-pbt.seed=%d'%seed],capture_output=True,text=True,timeout=60)
    o=r.stdout+r.stderr
    m=re.search(r'failed after (\d+) passed',o)
    if not m: return None
    prog=re.search(r'Program\{(.*?)\n\s*\[pbt\]\s+\}',o,re.S)
    steps=re.findall(r'var\d+ = ([a-z_]+)',prog.group(1)) if prog else []
    e=re.search(r'error: step \d+: (.*)',o)
    who=None
    if e:
        k=re.search(r'postcondition of ([a-z_]+)',e.group(1)); who=k.group(1) if k else e.group(1)[:30]
    if 'panic' in o and not who: who='panic'
    if e and 'panicked' in e.group(1): who='panic:'+re.search(r'([a-z_]+)\(',e.group(1)).group(1)
    return int(m.group(1)),len(steps),who,' '.join(steps)
N=int(sys.argv[2])
for test in sys.argv[3:]:
    res=[run(test,1000+i) for i in range(N)]
    found=[r for r in res if r]
    print('##',test,'found',len(found),'/',N)
    if found:
        print(' passed mean %.1f median %d max %d'%(statistics.mean(r[0] for r in found),statistics.median(r[0] for r in found),max(r[0] for r in found)))
        print(' by',collections.Counter(r[2] for r in found).most_common(4))
        print(' seqs',collections.Counter(r[3] for r in found).most_common(4))
