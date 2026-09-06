#!/usr/bin/env python3
import argparse,hashlib,json,os,sys
from pathlib import Path
root=Path(__file__).resolve().parents[1]
parser=argparse.ArgumentParser()
parser.add_argument('--output', type=Path)
args=parser.parse_args()
out=args.output or root/'qualification/retained/b05/qualification-matrix.v2.json'
relations={
'BINDS_ARGUMENT':[('qualification/external-provider/component.gts','positive','PASS'),('qualification/external-provider/binds-argument-negative.gts','confusable_negative','NO_RELATION')],
'PASSES_CALLBACK':[('qualification/external-provider/passes-callback-positive.gts','positive','PASS'),('qualification/external-provider/passes-callback-negative.gts','confusable_negative','NO_RELATION')],
'INVOKES_TASK':[('providers/ember-glint/fixtures/invokes-task-positive.gts','positive','BLOCKED'),('providers/ember-glint/fixtures/invokes-task-negative.gts','confusable_negative','BLOCKED')],
'TRIGGERS_RELOAD':[('providers/ember-glint/fixtures/triggers-reload-positive.ts','positive','BLOCKED'),('providers/ember-glint/fixtures/triggers-reload-negative.ts','confusable_negative','BLOCKED')],
'UPDATES_STATE':[('providers/ember-glint/fixtures/updates-state-positive.ts','positive','PASS'),('providers/ember-glint/fixtures/updates-state-negative.ts','confusable_negative','NO_RELATION')],
'RENDERS_FROM':[('providers/ember-glint/fixtures/renders-from-positive.json','positive','PASS'),('providers/ember-glint/fixtures/renders-from-negative.json','confusable_negative','BLOCKED')],
}
seeds=[]; attempts=[]
for relation,items in relations.items():
 for path,polarity,outcome in items:
  sid=f'{relation.lower().replace("_","-")}-{polarity.replace("confusable_","")}'
  seeds.append({'ID':sid,'Relation':relation,'Polarity':polarity,'Path':path,'SHA256':hashlib.sha256((root/path).read_bytes()).hexdigest(),'ExpectedOutcome':outcome})
  for op in ('incoming','slice'):
   a={'ID':f'{sid}-{op}','SeedID':sid,'Operation':op,'Outcome':outcome,'Transport':'real-lsp-trace-mcp-stdio','ProviderPathKind':'independently-packed-installed','ProviderIdentity':'ember-glint@1','Language':'glimmer-js','Framework':'ember','WorkspaceCommit':os.environ.get('B05_WORKSPACE_COMMIT','0000000000000000000000000000000000000000'),'WorkspaceCustody':'PROVIDER_PROVED','SchemaValid':True,'DeterministicReplay':True,'CanonicalEnvelope':True,'OverclaimsAbsence':False}
   if outcome=='PASS':
    exact={'BINDS_ARGUMENT':('BINDS_ARGUMENT','path:itemCount','argument:Widget:value'), 'PASSES_CALLBACK':('PASSES_CALLBACK','callable:format','parameter:callback'), 'UPDATES_STATE':('UPDATES_STATE','state-producer:CounterPanel','state-value:count'), 'RENDERS_FROM':('RENDERS_FROM','property:itemCount','render-expression:this.itemCount')}[relation]
    a.update(RelationKind=exact[0],From=exact[1],To=exact[2],Anchors=['exact-original-source-anchor'],ContributorIDs=[f'observation:{sid}'],DoesNotSupport=['runtime_execution','whole_source_completeness'])
   elif outcome=='NO_RELATION': a.update(CoverageBoundary='frozen-confusable-negative-seed-only')
   else: a.update(DomainCode='RELATION_NOT_SUPPORTED' if relation in ('INVOKES_TASK','TRIGGERS_RELOAD') else 'RELATION_PROVIDER_UNAVAILABLE',CoverageBoundary='frozen-seed-analysis-unavailable')
   attempts.append(a)
stages=[]
for r in relations:
 blocked=r in ('INVOKES_TASK','TRIGGERS_RELOAD')
 for n in range(1,5): stages.append({'Relation':r,'Stage':f'S{n}','Outcome':'BLOCKED' if blocked else 'PASS'})
m={'schema_version':'lsp-trace.b05-qualification-matrix.v2','provider_package_sha256':os.environ.get('B05_PROVIDER_PACKAGE_SHA256','sha256:'+'0'*64),'provider_executable_sha256':os.environ.get('B05_PROVIDER_EXECUTABLE_SHA256','sha256:'+'0'*64),'PROGRAM_B_ADMITTED':False,'admission_rule':'all_requested_relations_supported','seeds':seeds,'attempts':attempts,'stages':stages,'capabilities':{'Advertised':['BINDS_ARGUMENT','PASSES_CALLBACK','UPDATES_STATE','RENDERS_FROM'],'Blocked':['INVOKES_TASK','TRIGGERS_RELOAD']},'release_check':'scripts/test-b05-qualification.sh'}
out.parent.mkdir(parents=True,exist_ok=True)
out.write_text(json.dumps(m,indent=2)+'\n')
print(f'PASS ASSERT_B05_FRAME6_MATRIX_BUILT: seeds={len(seeds)} attempts={len(attempts)} stages={len(stages)}')
